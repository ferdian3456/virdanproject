package chat

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/ferdian3456/virdanproject/services/notification"
	"github.com/ferdian3456/virdanproject/services/server"
	"github.com/ferdian3456/virdanproject/shared"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

const (
	dmDefaultLimit    = 20
	dmMaxLimit        = 50
	dmMaxContentLen   = 4000
	dmPreviewMaxRunes = 120
)

var dmTypingLast sync.Map

type Service struct {
	Log             *zap.Logger
	Config          *koanf.Koanf
	DB              *pgxpool.Pool
	Repo            *Repository
	ServerRepo      *server.Repository
	NotificationSvc *notification.Service
	Broker          shared.WsBroker
	Hub             *shared.WsHub
}

func NewService(log *zap.Logger, config *koanf.Koanf, db *pgxpool.Pool, repo *Repository, serverRepo *server.Repository, notificationSvc *notification.Service, broker shared.WsBroker, hub *shared.WsHub) *Service {
	return &Service{
		Log: log, Config: config, DB: db, Repo: repo, ServerRepo: serverRepo,
		NotificationSvc: notificationSvc, Broker: broker, Hub: hub,
	}
}

func (service *Service) requireMember(ctx context.Context, serverId, userId string) error {
	count, err := service.ServerRepo.CheckServerMember(ctx, serverId, userId)
	if err != nil {
		return err
	}
	if count == 0 {
		return &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
	}
	return nil
}

func (service *Service) GetOrCreateConversation(ctx fiber.Ctx, serverId, callerId string, payload GetOrCreateConversationRequest) (DmConversationResponse, error) {
	v := shared.NewValidator()
	v.UUID("serverId", serverId)
	v.UUID("peerUserId", payload.PeerUserId)
	if valErr := v.Validate(); valErr != nil {
		return DmConversationResponse{}, valErr
	}
	if payload.PeerUserId == callerId {
		return DmConversationResponse{}, &shared.BadRequestError{Code: shared.ERR_VALIDATION_CODE, Message: "Cannot start a conversation with yourself", Param: "peerUserId"}
	}

	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetOrCreateConversation")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	if err = service.requireMember(ctxContext, serverId, callerId); err != nil {
		return DmConversationResponse{}, err
	}
	if err = service.requireMember(ctxContext, serverId, payload.PeerUserId); err != nil {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "Peer is not a member of this server", Param: "peerUserId"}
		return DmConversationResponse{}, err
	}

	now := time.Now().UTC()
	low, high := shared.SortUUIDPair(callerId, payload.PeerUserId)

	var tx pgx.Tx
	tx, err = service.DB.Begin(ctxContext)
	if err != nil {
		return DmConversationResponse{}, err
	}
	defer func() { _ = tx.Rollback(ctxContext) }()

	conv := DmConversation{
		Id: uuid.New().String(), ServerId: serverId, UserLow: low, UserHigh: high,
		CreatedAt: now, UpdatedAt: now, CreatedBy: callerId, UpdatedBy: callerId,
	}
	if _, err = service.Repo.InsertConversationIdempotent(ctxContext, tx, conv); err != nil {
		return DmConversationResponse{}, err
	}
	conv, err = service.Repo.GetConversationByPair(ctxContext, tx, serverId, low, high)
	if err != nil {
		return DmConversationResponse{}, err
	}
	if err = service.Repo.InsertConversationStateIdempotent(ctxContext, tx, conv.Id, low, serverId, high, now); err != nil {
		return DmConversationResponse{}, err
	}
	if err = service.Repo.InsertConversationStateIdempotent(ctxContext, tx, conv.Id, high, serverId, low, now); err != nil {
		return DmConversationResponse{}, err
	}
	if err = tx.Commit(ctxContext); err != nil {
		return DmConversationResponse{}, err
	}

	return DmConversationResponse{
		Id: conv.Id, ServerId: conv.ServerId, PeerUserId: payload.PeerUserId,
		Peer: DmIdentity{},
	}, nil
}

func (service *Service) ListMembers(ctx fiber.Ctx, serverId, callerId, q, cursorStr, limitStr string) (DmMemberListResponse, error) {
	v := shared.NewValidator()
	v.UUID("serverId", serverId)
	if valErr := v.Validate(); valErr != nil {
		return DmMemberListResponse{}, valErr
	}
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.ListMembers")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	if err = service.requireMember(ctxContext, serverId, callerId); err != nil {
		return DmMemberListResponse{}, err
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = dmDefaultLimit
	} else if limit > dmMaxLimit {
		limit = dmMaxLimit
	}
	var cursor *DmMemberCursor
	if cursorStr != "" {
		if cur, decErr := shared.DecodeCursor[DmMemberCursor](cursorStr); decErr == nil {
			cursor = cur
		}
	}

	minioFullUrl := fmt.Sprintf("%s%s/%s", service.Config.String("MINIO_HTTP"), service.Config.String("MINIO_URL"), service.Config.String("MINIO_BUCKET_NAME"))
	rows, err := service.Repo.ListMembers(ctxContext, callerId, serverId, q, limit+1, cursor, minioFullUrl)
	if err != nil {
		return DmMemberListResponse{}, err
	}
	var resp DmMemberListResponse
	if len(rows) > limit {
		last := rows[limit-1]
		resp.Page.NextCursor = shared.EncodeCursor(DmMemberCursor{Nickname: last.Identity.Nickname, UserId: last.UserId})
		rows = rows[:limit]
	}
	resp.Data = rows
	return resp, nil
}

func (service *Service) ListConversations(ctx fiber.Ctx, serverId, callerId, cursorStr, limitStr string) (DmConversationListResponse, error) {
	v := shared.NewValidator()
	v.UUID("serverId", serverId)
	if valErr := v.Validate(); valErr != nil {
		return DmConversationListResponse{}, valErr
	}
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.ListConversations")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	if err = service.requireMember(ctxContext, serverId, callerId); err != nil {
		return DmConversationListResponse{}, err
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = dmDefaultLimit
	} else if limit > dmMaxLimit {
		limit = dmMaxLimit
	}
	var cursor *DmConversationCursor
	if cursorStr != "" {
		if cur, decErr := shared.DecodeCursor[DmConversationCursor](cursorStr); decErr == nil {
			cursor = cur
		}
	}

	minioFullUrl := fmt.Sprintf("%s%s/%s", service.Config.String("MINIO_HTTP"), service.Config.String("MINIO_URL"), service.Config.String("MINIO_BUCKET_NAME"))
	rows, err := service.Repo.ListConversations(ctxContext, callerId, serverId, limit+1, cursor, minioFullUrl)
	if err != nil {
		return DmConversationListResponse{}, err
	}
	var resp DmConversationListResponse
	if len(rows) > limit {
		last := rows[limit-1]
		if last.LastMessageAt != nil {
			resp.Page.NextCursor = shared.EncodeCursor(DmConversationCursor{LastMessageAt: *last.LastMessageAt, ConversationId: last.Id})
		}
		rows = rows[:limit]
	}
	for i := range rows {
		rows[i].IsOnline = service.Hub.IsOnline(rows[i].PeerUserId)
	}
	resp.Data = rows
	return resp, nil
}

func (service *Service) SendMessage(ctx fiber.Ctx, conversationId, callerId string, payload SendMessageRequest) (DmMessageResponse, error) {
	v := shared.NewValidator()
	v.UUID("conversationId", conversationId)
	v.UUID("clientMessageId", payload.ClientMessageId)
	v.String("content", payload.Content).Required().MaxLen(dmMaxContentLen)
	if valErr := v.Validate(); valErr != nil {
		return DmMessageResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.SendMessage")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	conv, peerId, err := service.requireParticipantAndMember(ctxContext, conversationId, callerId)
	if err != nil {
		return DmMessageResponse{}, err
	}

	now := time.Now().UTC()
	preview := shared.TruncateRunes(payload.Content, dmPreviewMaxRunes)
	msg := DmMessage{
		Id: uuid.New().String(), ConversationId: conversationId, SenderId: callerId,
		Type: "text", Content: payload.Content, ClientMessageId: payload.ClientMessageId, CreatedAt: now,
	}

	var tx pgx.Tx
	tx, err = service.DB.Begin(ctxContext)
	if err != nil {
		return DmMessageResponse{}, err
	}
	defer func() { _ = tx.Rollback(ctxContext) }()

	var inserted bool
	inserted, err = service.Repo.InsertMessageIdempotent(ctxContext, tx, msg)
	if err != nil {
		return DmMessageResponse{}, err
	}

	stored := msg
	if !inserted {
		stored, err = service.Repo.GetMessageByClientId(ctxContext, tx, conversationId, callerId, payload.ClientMessageId)
		if err != nil {
			return DmMessageResponse{}, err
		}
		if err = tx.Commit(ctxContext); err != nil {
			return DmMessageResponse{}, err
		}
		senderIdentity, identityErr := service.resolveIdentity(ctxContext, conv.ServerId, callerId)
		if identityErr != nil {
			shared.GetLoggerWithTraceContext(ctxContext, service.Log).Warn("resolve sender identity failed", zap.Error(identityErr))
		}
		return DmMessageResponse{
			Id: stored.Id, ConversationId: stored.ConversationId, SenderId: stored.SenderId,
			Type: stored.Type, Content: stored.Content, ClientMessageId: stored.ClientMessageId, CreatedAt: stored.CreatedAt,
			Sender: senderIdentity,
		}, nil
	}

	if err = service.Repo.UpdateConversationLastMessage(ctxContext, tx, conversationId, callerId, now); err != nil {
		return DmMessageResponse{}, err
	}
	if err = service.Repo.BumpConversationStates(ctxContext, tx, conversationId, callerId, preview, now); err != nil {
		return DmMessageResponse{}, err
	}
	if err = tx.Commit(ctxContext); err != nil {
		return DmMessageResponse{}, err
	}

	senderIdentity, identityErr := service.resolveIdentity(ctxContext, conv.ServerId, callerId)
	if identityErr != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Warn("resolve sender identity failed", zap.Error(identityErr))
	}
	resp := DmMessageResponse{
		Id: stored.Id, ConversationId: stored.ConversationId, SenderId: stored.SenderId,
		Type: stored.Type, Content: stored.Content, ClientMessageId: stored.ClientMessageId, CreatedAt: stored.CreatedAt,
		Sender: senderIdentity,
	}

	ev := shared.WsEvent{Type: "message.new", Payload: resp}
	if pubErr := service.Broker.Publish(ctxContext, []string{peerId}, ev); pubErr != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Warn("dm fanout failed", zap.Error(pubErr))
	}
	service.NotificationSvc.NotifyDM(ctxContext, peerId, conversationId, resp.Sender.Username, preview)
	return resp, nil
}

func (service *Service) ListMessages(ctx fiber.Ctx, conversationId, callerId, cursorStr, limitStr string) (DmMessageListResponse, error) {
	v := shared.NewValidator()
	v.UUID("conversationId", conversationId)
	if valErr := v.Validate(); valErr != nil {
		return DmMessageListResponse{}, valErr
	}
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.ListMessages")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	conv, _, err := service.requireParticipantAndMember(ctxContext, conversationId, callerId)
	if err != nil {
		return DmMessageListResponse{}, err
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = dmDefaultLimit
	} else if limit > dmMaxLimit {
		limit = dmMaxLimit
	}
	var cursor *DmMessageCursor
	if cursorStr != "" {
		if cur, decErr := shared.DecodeCursor[DmMessageCursor](cursorStr); decErr == nil {
			cursor = cur
		}
	}
	minioFullUrl := fmt.Sprintf("%s%s/%s", service.Config.String("MINIO_HTTP"), service.Config.String("MINIO_URL"), service.Config.String("MINIO_BUCKET_NAME"))
	rows, err := service.Repo.ListMessages(ctxContext, conversationId, conv.ServerId, limit+1, cursor, minioFullUrl)
	if err != nil {
		return DmMessageListResponse{}, err
	}
	var resp DmMessageListResponse
	if len(rows) > limit {
		last := rows[limit-1]
		resp.Page.NextCursor = shared.EncodeCursor(DmMessageCursor{CreatedAt: last.CreatedAt, Id: last.Id})
		rows = rows[:limit]
	}
	resp.Data = rows
	return resp, nil
}

func (service *Service) MarkRead(ctx fiber.Ctx, conversationId, callerId string, payload MarkReadRequest) error {
	v := shared.NewValidator()
	v.UUID("conversationId", conversationId)
	if payload.LastReadMessageId != nil {
		v.UUID("lastReadMessageId", *payload.LastReadMessageId)
	}
	if valErr := v.Validate(); valErr != nil {
		return valErr
	}
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.MarkRead")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	_, peerId, err := service.requireParticipantAndMember(ctxContext, conversationId, callerId)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	readAt := now
	if payload.LastReadMessageId != nil {
		if ts, e := service.Repo.GetMessageCreatedAt(ctxContext, conversationId, *payload.LastReadMessageId); e == nil {
			readAt = ts
		}
	}
	if err = service.Repo.MarkRead(ctxContext, conversationId, callerId, payload.LastReadMessageId, readAt, now); err != nil {
		return err
	}
	ev := shared.WsEvent{Type: "message.read", Payload: WsReadPayload{ConversationId: conversationId, UserId: callerId, LastReadAt: readAt}}
	if pubErr := service.Broker.Publish(ctxContext, []string{peerId}, ev); pubErr != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Warn("dm read receipt fanout failed", zap.Error(pubErr))
	}
	return nil
}

func (service *Service) resolveIdentity(ctx context.Context, serverId, userId string) (DmIdentity, error) {
	minioFullUrl := fmt.Sprintf("%s%s/%s", service.Config.String("MINIO_HTTP"), service.Config.String("MINIO_URL"), service.Config.String("MINIO_BUCKET_NAME"))
	return service.Repo.GetMemberIdentity(ctx, serverId, userId, minioFullUrl)
}

func (service *Service) requireParticipantAndMember(ctx context.Context, conversationId, callerId string) (DmConversation, string, error) {
	conv, err := service.Repo.GetConversationById(ctx, conversationId)
	if err == pgx.ErrNoRows {
		return DmConversation{}, "", &shared.NotFoundError{Code: shared.ERR_NOT_FOUND_CODE, Message: "Conversation not found", Param: "conversationId"}
	}
	if err != nil {
		return DmConversation{}, "", err
	}
	if conv.UserLow != callerId && conv.UserHigh != callerId {
		return DmConversation{}, "", &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "Not a participant of this conversation", Param: "conversationId"}
	}
	if err = service.requireMember(ctx, conv.ServerId, callerId); err != nil {
		return DmConversation{}, "", err
	}
	peerId := conv.UserHigh
	if callerId == conv.UserHigh {
		peerId = conv.UserLow
	}
	return conv, peerId, nil
}

func (service *Service) BroadcastPresence(userId string, online bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	peerIds, err := service.Repo.GetConversationPeerIds(ctx, userId)
	if err != nil || len(peerIds) == 0 {
		return
	}
	ev := shared.WsEvent{Type: "presence", Payload: WsPresencePayload{UserId: userId, Online: online}}
	if pubErr := service.Broker.Publish(ctx, peerIds, ev); pubErr != nil {
		shared.GetLoggerWithTraceContext(ctx, service.Log).Warn("presence fanout failed", zap.Error(pubErr))
	}
}

func (service *Service) HandleInboundFrame(userId string, raw []byte) {
	var in WsInboundTyping
	if err := sonic.Unmarshal(raw, &in); err != nil || in.Type != "typing" {
		return
	}
	if in.Payload.ConversationId == "" {
		return
	}
	key := userId + "|" + in.Payload.ConversationId
	now := time.Now()
	if last, ok := dmTypingLast.Load(key); ok {
		if now.Sub(last.(time.Time)) < time.Second {
			return
		}
	}
	dmTypingLast.Store(key, now)

	bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conv, err := service.Repo.GetConversationById(bg, in.Payload.ConversationId)
	if err != nil {
		return
	}
	if conv.UserLow != userId && conv.UserHigh != userId {
		return
	}
	peerId := conv.UserHigh
	if userId == conv.UserHigh {
		peerId = conv.UserLow
	}
	ev := shared.WsEvent{Type: "typing", Payload: WsTypingPayload{
		ConversationId: in.Payload.ConversationId, UserId: userId, IsTyping: in.Payload.IsTyping,
	}}
	if pubErr := service.Broker.Publish(bg, []string{peerId}, ev); pubErr != nil {
		shared.GetLoggerWithTraceContext(bg, service.Log).Warn("typing fanout failed", zap.Error(pubErr))
	}
}

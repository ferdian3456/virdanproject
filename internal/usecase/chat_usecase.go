package usecase

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/repository"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/ferdian3456/virdanproject/internal/ws"
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

type ChatUsecase struct {
	Log                 *zap.Logger
	Config              *koanf.Koanf
	DB                  *pgxpool.Pool
	ChatRepository      *repository.ChatRepository
	ServerRepository    *repository.ServerRepository
	NotificationUsecase *NotificationUsecase
	Broker              ws.Broker
	Hub                 *ws.Hub
}

func NewChatUsecase(log *zap.Logger, config *koanf.Koanf, db *pgxpool.Pool, chatRepo *repository.ChatRepository, serverRepo *repository.ServerRepository, notif *NotificationUsecase, broker ws.Broker, hub *ws.Hub) *ChatUsecase {
	return &ChatUsecase{
		Log: log, Config: config, DB: db, ChatRepository: chatRepo, ServerRepository: serverRepo,
		NotificationUsecase: notif, Broker: broker, Hub: hub,
	}
}

func (usecase *ChatUsecase) requireMember(ctx context.Context, serverId, userId string) error {
	count, err := usecase.ServerRepository.CheckServerMember(ctx, serverId, userId)
	if err != nil {
		return err
	}
	if count == 0 {
		return &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
	}
	return nil
}

// GetOrCreateConversation: caller+peer must be members; caller != peer.
func (usecase *ChatUsecase) GetOrCreateConversation(ctx fiber.Ctx, serverId, callerId string, payload model.GetOrCreateConversationRequest) (model.DmConversationResponse, error) {
	v := util.NewValidator()
	v.UUID("serverId", serverId)
	v.UUID("peerUserId", payload.PeerUserId)
	if valErr := v.Validate(); valErr != nil {
		return model.DmConversationResponse{}, valErr
	}
	if payload.PeerUserId == callerId {
		return model.DmConversationResponse{}, &model.BadRequestError{Code: constant.ERR_VALIDATION_CODE, Message: "Cannot start a conversation with yourself", Param: "peerUserId"}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.GetOrCreateConversation")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	if err = usecase.requireMember(ctxContext, serverId, callerId); err != nil {
		return model.DmConversationResponse{}, err
	}
	if err = usecase.requireMember(ctxContext, serverId, payload.PeerUserId); err != nil {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "Peer is not a member of this server", Param: "peerUserId"}
		return model.DmConversationResponse{}, err
	}

	now := time.Now().UTC()
	low, high := util.SortUUIDPair(callerId, payload.PeerUserId)

	var tx pgx.Tx
	tx, err = usecase.DB.Begin(ctxContext)
	if err != nil {
		return model.DmConversationResponse{}, err
	}
	defer func() { _ = tx.Rollback(ctxContext) }()

	conv := model.DmConversation{
		Id: uuid.New().String(), ServerId: serverId, UserLow: low, UserHigh: high,
		CreatedAt: now, UpdatedAt: now, CreatedBy: callerId, UpdatedBy: callerId,
	}
	if _, err = usecase.ChatRepository.InsertConversationIdempotent(ctxContext, tx, conv); err != nil {
		return model.DmConversationResponse{}, err
	}
	conv, err = usecase.ChatRepository.GetConversationByPair(ctxContext, tx, serverId, low, high)
	if err != nil {
		return model.DmConversationResponse{}, err
	}
	if err = usecase.ChatRepository.InsertConversationStateIdempotent(ctxContext, tx, conv.Id, low, serverId, high, now); err != nil {
		return model.DmConversationResponse{}, err
	}
	if err = usecase.ChatRepository.InsertConversationStateIdempotent(ctxContext, tx, conv.Id, high, serverId, low, now); err != nil {
		return model.DmConversationResponse{}, err
	}
	if err = tx.Commit(ctxContext); err != nil {
		return model.DmConversationResponse{}, err
	}

	return model.DmConversationResponse{
		Id: conv.Id, ServerId: conv.ServerId, PeerUserId: payload.PeerUserId,
		Peer: model.DmIdentity{},
	}, nil
}

func (usecase *ChatUsecase) ListMembers(ctx fiber.Ctx, serverId, callerId, q, cursorStr, limitStr string) (model.DmMemberListResponse, error) {
	v := util.NewValidator()
	v.UUID("serverId", serverId)
	if valErr := v.Validate(); valErr != nil {
		return model.DmMemberListResponse{}, valErr
	}
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.ListMembers")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	if err = usecase.requireMember(ctxContext, serverId, callerId); err != nil {
		return model.DmMemberListResponse{}, err
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = dmDefaultLimit
	} else if limit > dmMaxLimit {
		limit = dmMaxLimit
	}
	var cursor *model.DmMemberCursor
	if cursorStr != "" {
		if cur, decErr := util.DecodeCursor[model.DmMemberCursor](cursorStr); decErr == nil {
			cursor = cur
		}
	}

	minioFullUrl := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))
	rows, err := usecase.ChatRepository.ListMembers(ctxContext, callerId, serverId, q, limit+1, cursor, minioFullUrl)
	if err != nil {
		return model.DmMemberListResponse{}, err
	}
	var resp model.DmMemberListResponse
	if len(rows) > limit {
		last := rows[limit-1]
		resp.Page.NextCursor = util.EncodeCursor(model.DmMemberCursor{Nickname: last.Identity.Nickname, UserId: last.UserId})
		rows = rows[:limit]
	}
	resp.Data = rows
	return resp, nil
}

func (usecase *ChatUsecase) ListConversations(ctx fiber.Ctx, serverId, callerId, cursorStr, limitStr string) (model.DmConversationListResponse, error) {
	v := util.NewValidator()
	v.UUID("serverId", serverId)
	if valErr := v.Validate(); valErr != nil {
		return model.DmConversationListResponse{}, valErr
	}
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.ListConversations")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	if err = usecase.requireMember(ctxContext, serverId, callerId); err != nil {
		return model.DmConversationListResponse{}, err
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = dmDefaultLimit
	} else if limit > dmMaxLimit {
		limit = dmMaxLimit
	}
	var cursor *model.DmConversationCursor
	if cursorStr != "" {
		if cur, decErr := util.DecodeCursor[model.DmConversationCursor](cursorStr); decErr == nil {
			cursor = cur
		}
	}

	minioFullUrl := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))
	rows, err := usecase.ChatRepository.ListConversations(ctxContext, callerId, serverId, limit+1, cursor, minioFullUrl)
	if err != nil {
		return model.DmConversationListResponse{}, err
	}
	var resp model.DmConversationListResponse
	if len(rows) > limit {
		last := rows[limit-1]
		if last.LastMessageAt != nil {
			resp.Page.NextCursor = util.EncodeCursor(model.DmConversationCursor{LastMessageAt: *last.LastMessageAt, ConversationId: last.Id})
		}
		rows = rows[:limit]
	}
	resp.Data = rows
	return resp, nil
}

func (usecase *ChatUsecase) SendMessage(ctx fiber.Ctx, conversationId, callerId string, payload model.SendMessageRequest) (model.DmMessageResponse, error) {
	v := util.NewValidator()
	v.UUID("conversationId", conversationId)
	v.UUID("clientMessageId", payload.ClientMessageId)
	v.String("content", payload.Content).Required().MaxLen(dmMaxContentLen)
	if valErr := v.Validate(); valErr != nil {
		return model.DmMessageResponse{}, valErr
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.SendMessage")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	conv, peerId, err := usecase.requireParticipantAndMember(ctxContext, conversationId, callerId)
	if err != nil {
		return model.DmMessageResponse{}, err
	}

	now := time.Now().UTC()
	preview := util.TruncateRunes(payload.Content, dmPreviewMaxRunes)
	msg := model.DmMessage{
		Id: uuid.New().String(), ConversationId: conversationId, SenderId: callerId,
		Type: "text", Content: payload.Content, ClientMessageId: payload.ClientMessageId, CreatedAt: now,
	}

	var tx pgx.Tx
	tx, err = usecase.DB.Begin(ctxContext)
	if err != nil {
		return model.DmMessageResponse{}, err
	}
	defer func() { _ = tx.Rollback(ctxContext) }()

	var inserted bool
	inserted, err = usecase.ChatRepository.InsertMessageIdempotent(ctxContext, tx, msg)
	if err != nil {
		return model.DmMessageResponse{}, err
	}

	stored := msg
	if !inserted {
		stored, err = usecase.ChatRepository.GetMessageByClientId(ctxContext, tx, conversationId, callerId, payload.ClientMessageId)
		if err != nil {
			return model.DmMessageResponse{}, err
		}
		if err = tx.Commit(ctxContext); err != nil {
			return model.DmMessageResponse{}, err
		}
		senderIdentity, identityErr := usecase.resolveIdentity(ctxContext, conv.ServerId, callerId)
		if identityErr != nil {
			util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Warn("resolve sender identity failed", zap.Error(identityErr))
		}
		return model.DmMessageResponse{
			Id: stored.Id, ConversationId: stored.ConversationId, SenderId: stored.SenderId,
			Type: stored.Type, Content: stored.Content, ClientMessageId: stored.ClientMessageId, CreatedAt: stored.CreatedAt,
			Sender: senderIdentity,
		}, nil
	}

	if err = usecase.ChatRepository.UpdateConversationLastMessage(ctxContext, tx, conversationId, callerId, now); err != nil {
		return model.DmMessageResponse{}, err
	}
	if err = usecase.ChatRepository.BumpConversationStates(ctxContext, tx, conversationId, callerId, preview, now); err != nil {
		return model.DmMessageResponse{}, err
	}
	if err = tx.Commit(ctxContext); err != nil {
		return model.DmMessageResponse{}, err
	}

	senderIdentity, identityErr := usecase.resolveIdentity(ctxContext, conv.ServerId, callerId)
	if identityErr != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Warn("resolve sender identity failed", zap.Error(identityErr))
	}
	resp := model.DmMessageResponse{
		Id: stored.Id, ConversationId: stored.ConversationId, SenderId: stored.SenderId,
		Type: stored.Type, Content: stored.Content, ClientMessageId: stored.ClientMessageId, CreatedAt: stored.CreatedAt,
		Sender: senderIdentity,
	}

	ev := ws.Event{Type: "message.new", Payload: resp}
	if pubErr := usecase.Broker.Publish(ctxContext, []string{peerId, callerId}, ev); pubErr != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Warn("dm fanout failed", zap.Error(pubErr))
	}
	usecase.NotificationUsecase.NotifyDM(ctxContext, peerId, conversationId, resp.Sender.Username, preview)
	return resp, nil
}

func (usecase *ChatUsecase) ListMessages(ctx fiber.Ctx, conversationId, callerId, cursorStr, limitStr string) (model.DmMessageListResponse, error) {
	v := util.NewValidator()
	v.UUID("conversationId", conversationId)
	if valErr := v.Validate(); valErr != nil {
		return model.DmMessageListResponse{}, valErr
	}
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.ListMessages")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	conv, _, err := usecase.requireParticipantAndMember(ctxContext, conversationId, callerId)
	if err != nil {
		return model.DmMessageListResponse{}, err
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = dmDefaultLimit
	} else if limit > dmMaxLimit {
		limit = dmMaxLimit
	}
	var cursor *model.DmMessageCursor
	if cursorStr != "" {
		if cur, decErr := util.DecodeCursor[model.DmMessageCursor](cursorStr); decErr == nil {
			cursor = cur
		}
	}
	minioFullUrl := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))
	rows, err := usecase.ChatRepository.ListMessages(ctxContext, conversationId, conv.ServerId, limit+1, cursor, minioFullUrl)
	if err != nil {
		return model.DmMessageListResponse{}, err
	}
	var resp model.DmMessageListResponse
	if len(rows) > limit {
		last := rows[limit-1]
		resp.Page.NextCursor = util.EncodeCursor(model.DmMessageCursor{CreatedAt: last.CreatedAt, Id: last.Id})
		rows = rows[:limit]
	}
	resp.Data = rows
	return resp, nil
}

func (usecase *ChatUsecase) MarkRead(ctx fiber.Ctx, conversationId, callerId string, payload model.MarkReadRequest) error {
	v := util.NewValidator()
	v.UUID("conversationId", conversationId)
	if payload.LastReadMessageId != nil {
		v.UUID("lastReadMessageId", *payload.LastReadMessageId)
	}
	if valErr := v.Validate(); valErr != nil {
		return valErr
	}
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.MarkRead")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	_, peerId, err := usecase.requireParticipantAndMember(ctxContext, conversationId, callerId)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	readAt := now
	if payload.LastReadMessageId != nil {
		if ts, e := usecase.ChatRepository.GetMessageCreatedAt(ctxContext, conversationId, *payload.LastReadMessageId); e == nil {
			readAt = ts
		}
	}
	if err = usecase.ChatRepository.MarkRead(ctxContext, conversationId, callerId, payload.LastReadMessageId, readAt, now); err != nil {
		return err
	}
	ev := ws.Event{Type: "message.read", Payload: model.WsReadPayload{ConversationId: conversationId, UserId: callerId, LastReadAt: readAt}}
	if pubErr := usecase.Broker.Publish(ctxContext, []string{peerId}, ev); pubErr != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Warn("dm read receipt fanout failed", zap.Error(pubErr))
	}
	return nil
}

func (usecase *ChatUsecase) resolveIdentity(ctx context.Context, serverId, userId string) (model.DmIdentity, error) {
	minioFullUrl := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))
	return usecase.ChatRepository.GetMemberIdentity(ctx, serverId, userId, minioFullUrl)
}

// requireParticipantAndMember loads the conversation, asserts caller is a
// participant and still a server member. Returns the conversation + peerId.
func (usecase *ChatUsecase) requireParticipantAndMember(ctx context.Context, conversationId, callerId string) (model.DmConversation, string, error) {
	conv, err := usecase.ChatRepository.GetConversationById(ctx, conversationId)
	if err == pgx.ErrNoRows {
		return model.DmConversation{}, "", &model.NotFoundError{Code: constant.ERR_NOT_FOUND_CODE, Message: "Conversation not found", Param: "conversationId"}
	}
	if err != nil {
		return model.DmConversation{}, "", err
	}
	if conv.UserLow != callerId && conv.UserHigh != callerId {
		return model.DmConversation{}, "", &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "Not a participant of this conversation", Param: "conversationId"}
	}
	if err = usecase.requireMember(ctx, conv.ServerId, callerId); err != nil {
		return model.DmConversation{}, "", err
	}
	peerId := conv.UserHigh
	if callerId == conv.UserHigh {
		peerId = conv.UserLow
	}
	return conv, peerId, nil
}

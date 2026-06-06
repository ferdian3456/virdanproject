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

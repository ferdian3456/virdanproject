package usecase

import (
	"context"
	"fmt"
	"time"

	"firebase.google.com/go/v4/messaging"
	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/repository"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type NotificationUsecase struct {
	NotificationRepository *repository.NotificationRepository
	ServerRepository       *repository.ServerRepository
	FCMClient              *messaging.Client
	DB                     *pgxpool.Pool
	Log                    *zap.Logger
	Config                 *koanf.Koanf
}

func NewNotificationUsecase(
	notificationRepository *repository.NotificationRepository,
	serverRepository *repository.ServerRepository,
	fcmClient *messaging.Client,
	db *pgxpool.Pool,
	log *zap.Logger,
	config *koanf.Koanf,
) *NotificationUsecase {
	return &NotificationUsecase{
		NotificationRepository: notificationRepository,
		ServerRepository:       serverRepository,
		FCMClient:              fcmClient,
		DB:                     db,
		Log:                    log,
		Config:                 config,
	}
}

func (usecase *NotificationUsecase) RegisterDevice(ctx context.Context, userId string, request model.DeviceTokenRegisterRequest) error {
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-usecase").Start(ctx, "usecase.RegisterDevice")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
	)

	v := util.NewValidator()
	v.String("token", request.Token).Required().MaxLen(4096)
	if err = v.Validate(); err != nil {
		return err
	}

	if request.Platform != constant.PLATFORM_ANDROID && request.Platform != constant.PLATFORM_IOS {
		err = &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "platform must be android or ios",
			Param:   "platform",
		}
		return err
	}

	tx, err := usecase.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	err = usecase.NotificationRepository.DeleteAllUserDeviceToken(ctx, tx, userId)
	if err != nil {
		return err
	}

	now := time.Now()
	deviceToken := model.DeviceToken{
		Id:        uuid.New().String(),
		UserId:    userId,
		Token:     request.Token,
		Platform:  request.Platform,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: userId,
		UpdatedBy: userId,
	}

	err = usecase.NotificationRepository.UpsertDeviceToken(ctx, tx, deviceToken)
	if err != nil {
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, usecase.Log).Error("Failed to commit transaction", zap.Error(err))
		return err
	}
	return nil
}

func (usecase *NotificationUsecase) UnregisterDevice(ctx context.Context, userId string, request model.DeviceTokenDeleteRequest) error {
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-usecase").Start(ctx, "usecase.UnregisterDevice")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
	)

	v := util.NewValidator()
	v.String("token", request.Token).Required()
	if err = v.Validate(); err != nil {
		return err
	}

	err = usecase.NotificationRepository.DeleteDeviceToken(ctx, userId, request.Token)
	if err != nil {
		return err
	}
	return nil
}

func (usecase *NotificationUsecase) TestSend(ctx context.Context, userId string) error {
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-usecase").Start(ctx, "usecase.TestSend")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
	)

	tokens, err := usecase.NotificationRepository.ListTokensByUserId(ctx, userId)
	if err != nil {
		return err
	}

	if len(tokens) == 0 {
		err = &model.NotFoundError{
			Code:    constant.ERR_NOT_FOUND_CODE,
			Message: "No device registered for this user",
			Param:   "token",
		}
		return err
	}

	message := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: "Virdan",
			Body:  "Test notification berhasil.",
		},
		Data:    map[string]string{"type": "test"},
		Android: &messaging.AndroidConfig{Priority: "high"},
	}

	response, fcmErr := usecase.FCMClient.SendEachForMulticast(ctx, message)
	if fcmErr != nil {
		util.GetLoggerWithTraceContext(ctx, usecase.Log).Error("Failed to send FCM multicast", zap.Error(fcmErr))
		return nil
	}

	var invalidTokens []string
	for i, result := range response.Responses {
		if !result.Success && (messaging.IsUnregistered(result.Error) || messaging.IsInvalidArgument(result.Error)) {
			invalidTokens = append(invalidTokens, tokens[i])
		}
	}

	if len(invalidTokens) > 0 {
		if deleteErr := usecase.NotificationRepository.DeleteInvalidTokens(ctx, invalidTokens); deleteErr != nil {
			util.GetLoggerWithTraceContext(ctx, usecase.Log).Warn("Failed to delete invalid tokens", zap.Error(deleteErr))
		}
	}

	return nil
}

// Notify delivers notifications for a batch of domain events. Fire-and-forget goroutine: called
// after the action commits, it must never block or fail the user's request. The request ctx is
// cancelled when the HTTP response returns, so the goroutine uses a fresh Background ctx — but it
// CARRIES the request's span context, so its span is a child in the SAME trace (one trace = like
// request -> Notify), not a disconnected new root. recover() keeps a goroutine panic from crashing
// the process. IG model: the row is ALWAYS persisted (feed archive); the per-type preference gates
// only the PUSH. Per recipient it dedups to the highest-priority event (mention>reply>comment>like).
// Every error is logged with trace context; one recipient's failure continues to the next.
// Self-notif (actor==recipient) is filtered by the caller in post_usecase.
func (usecase *NotificationUsecase) Notify(ctx context.Context, events []model.NotificationEvent) {
	// Capture the request's span context BEFORE spawning — the request ctx is about to be
	// cancelled, but its SpanContext (trace_id + span_id) is immutable and safe to carry.
	parentSpanCtx := trace.SpanContextFromContext(ctx)

	go func() {
		// Fresh Background ctx (own lifecycle, not cancelled with the request) carrying the parent
		// trace, so the span below is a child of the request span — same trace_id.
		backgroundCtx := trace.ContextWithSpanContext(context.Background(), parentSpanCtx)
		backgroundCtx, cancel := context.WithTimeout(backgroundCtx, 30*time.Second)
		defer cancel()

		serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
		backgroundCtx, span := otel.Tracer(serviceName + "-usecase").Start(backgroundCtx, "usecase.Notify")
		defer span.End()

		logger := util.GetLoggerWithTraceContext(backgroundCtx, usecase.Log)

		// recover() is mandatory: a panic in a goroutine is NOT caught by the HTTP recover
		// middleware — an unrecovered goroutine panic crashes the whole process.
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic in Notify goroutine", zap.Any("recover", recovered))
			}
		}()

		// 1. Dedup per recipient — priority: mention=4 > reply=3 > comment=2 > like=1
		priority := map[string]int{"like": 1, "comment": 2, "reply": 3, "mention": 4}
		deduped := make(map[string]model.NotificationEvent, len(events))
		for _, event := range events {
			existing, exists := deduped[event.RecipientUserId]
			if !exists || priority[event.Type] > priority[existing.Type] {
				deduped[event.RecipientUserId] = event
			}
		}

		minioFullUrl := fmt.Sprintf("%s%s/%s",
			usecase.Config.String("MINIO_HTTP"),
			usecase.Config.String("MINIO_URL"),
			usecase.Config.String("MINIO_BUCKET_NAME"),
		)

		// Single err reused across the loop. Each error is handled inline (log + continue) — a
		// goroutine can't propagate it to a caller.
		var err error
		for _, event := range deduped {
			// 2. Persist the row — ALWAYS (IG model: feed archive complete regardless of push pref).
			now := time.Now()
			postId := &event.PostId
			notification := model.Notification{
				Id:              uuid.New().String(),
				RecipientUserId: event.RecipientUserId,
				ActorUserId:     event.ActorUserId,
				ActorProfileId:  event.ActorProfileId,
				Type:            event.Type,
				ServerId:        event.ServerId,
				PostId:          postId,
				CommentId:       event.CommentId,
				CreatedAt:       now,
				UpdatedAt:       now,
				CreatedBy:       event.ActorUserId,
				UpdatedBy:       event.ActorUserId,
			}
			if err = usecase.NotificationRepository.InsertNotification(backgroundCtx, notification); err != nil {
				logger.Error("notif: failed to insert row, skipping recipient",
					zap.String("recipient_user_id", event.RecipientUserId), zap.Error(err))
				continue
			}

			// 3. Push path. No registered device -> nothing to push (row already in feed).
			var tokens []string
			if tokens, err = usecase.NotificationRepository.ListTokensByUserId(backgroundCtx, event.RecipientUserId); err != nil {
				logger.Error("notif: failed to list device tokens, skipping push",
					zap.String("recipient_user_id", event.RecipientUserId), zap.Error(err))
				continue
			}
			if len(tokens) == 0 {
				continue
			}

			// 4. Per-type PUSH preference (IG model: gates push only). Fail-closed: read error ->
			//    skip push (row already saved; don't risk pushing to an opt-out user).
			var prefs model.NotificationPrefs
			if prefs, err = usecase.NotificationRepository.GetUserNotificationPrefs(backgroundCtx, event.RecipientUserId); err != nil {
				logger.Error("notif: failed to read prefs, skipping push (fail-closed)",
					zap.String("recipient_user_id", event.RecipientUserId), zap.Error(err))
				continue
			}
			if !pushEnabledForType(prefs, event.Type) {
				continue
			}

			// 5. Actor's per-server username for the push title. Profile is FK-guaranteed, so a miss
			//    is a real anomaly -> log + skip push.
			var actorUsername string
			if actorUsername, _, err = usecase.NotificationRepository.GetActorUsernameAndAvatar(backgroundCtx, event.ActorProfileId, minioFullUrl); err != nil {
				logger.Error("notif: failed to resolve actor username, skipping push",
					zap.String("actor_profile_id", event.ActorProfileId), zap.Error(err))
				continue
			}

			// 6. Send push.
			usecase.sendPush(backgroundCtx, tokens, actorUsername, event)
		}
	}()
}

func (usecase *NotificationUsecase) sendPush(ctx context.Context, tokens []string, actorUsername string, event model.NotificationEvent) {
	body := notifBodyForType(event.Type)

	data := map[string]string{
		"type":     event.Type,
		"serverId": event.ServerId,
		"postId":   event.PostId,
	}
	if event.CommentId != nil {
		data["commentId"] = *event.CommentId
	}

	message := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: actorUsername,
			Body:  body,
		},
		Data:    data,
		Android: &messaging.AndroidConfig{Priority: "high"},
	}

	response, fcmErr := usecase.FCMClient.SendEachForMulticast(ctx, message)
	if fcmErr != nil {
		util.GetLoggerWithTraceContext(ctx, usecase.Log).Error("Failed to send FCM push", zap.Error(fcmErr))
		return
	}

	var invalidTokens []string
	for i, result := range response.Responses {
		if !result.Success && (messaging.IsUnregistered(result.Error) || messaging.IsInvalidArgument(result.Error)) {
			invalidTokens = append(invalidTokens, tokens[i])
		}
	}

	if len(invalidTokens) > 0 {
		if deleteErr := usecase.NotificationRepository.DeleteInvalidTokens(ctx, invalidTokens); deleteErr != nil {
			util.GetLoggerWithTraceContext(ctx, usecase.Log).Warn("Failed to delete invalid tokens after push", zap.Error(deleteErr))
		}
	}
}

func notifBodyForType(notifType string) string {
	switch notifType {
	case "like":
		return "menyukai postinganmu."
	case "comment":
		return "mengomentari postinganmu."
	case "reply":
		return "membalas komentarmu."
	default:
		return "berinteraksi denganmu."
	}
}

// pushEnabledForType maps a notification type to its per-type push toggle. mention has no toggle in
// Fase 2 prefs -> defaults to push-on (add a mention toggle to NotificationPrefs when needed).
func pushEnabledForType(prefs model.NotificationPrefs, notifType string) bool {
	switch notifType {
	case "like":
		return prefs.NotifLike
	case "comment":
		return prefs.NotifComment
	case "reply":
		return prefs.NotifReply
	default:
		return true
	}
}

func (usecase *NotificationUsecase) GetFeed(ctx context.Context, userId string, serverId string, cursorStr string, limit int) (model.NotificationListResponse, error) {
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-usecase").Start(ctx, "usecase.GetNotificationFeed")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	v := util.NewValidator()
	v.UUID("serverId", serverId).Required()
	if err = v.Validate(); err != nil {
		return model.NotificationListResponse{}, err
	}

	var memberCount int
	memberCount, err = usecase.ServerRepository.CheckServerMember(ctx, serverId, userId)
	if err != nil {
		return model.NotificationListResponse{}, err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return model.NotificationListResponse{}, err
	}

	if limit <= 0 {
		limit = constant.DEFAULT_LIMIT
	}
	if limit > constant.MAX_LIMIT {
		limit = constant.MAX_LIMIT
	}

	var cursor *model.NotificationCursor
	if cursorStr != "" {
		cursor, err = util.DecodeCursor[model.NotificationCursor](cursorStr)
		if err != nil {
			err = &model.BadRequestError{Code: constant.ERR_VALIDATION_CODE, Message: "Invalid cursor", Param: "cursor"}
			return model.NotificationListResponse{}, err
		}
	}

	minioFullUrl := fmt.Sprintf("%s%s/%s",
		usecase.Config.String("MINIO_HTTP"),
		usecase.Config.String("MINIO_URL"),
		usecase.Config.String("MINIO_BUCKET_NAME"),
	)

	// Fetch limit+1: if we get more than limit, there is a next page. Encode the cursor here
	// (usecase), not in the repo — matches GetServerPosts.
	var items []model.NotificationResponse
	items, err = usecase.NotificationRepository.ListByRecipient(ctx, userId, serverId, cursor, limit+1, minioFullUrl)
	if err != nil {
		return model.NotificationListResponse{}, err
	}

	response := model.NotificationListResponse{Data: []model.NotificationResponse{}}
	if len(items) > limit {
		response.Data = items[:limit]
		last := items[limit-1]
		response.Page.NextCursor = util.EncodeCursor(model.NotificationCursor{
			CreatedAt: last.CreatedAt,
			Id:        last.Id,
		})
	} else {
		response.Data = items
	}

	return response, nil
}

func (usecase *NotificationUsecase) MarkRead(ctx context.Context, userId string, serverId string, notifId string) error {
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-usecase").Start(ctx, "usecase.MarkNotificationRead")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
		attribute.String("notification.id", notifId),
	)

	v := util.NewValidator()
	v.UUID("serverId", serverId).Required()
	v.UUID("id", notifId)
	if err = v.Validate(); err != nil {
		return err
	}

	var memberCount int
	memberCount, err = usecase.ServerRepository.CheckServerMember(ctx, serverId, userId)
	if err != nil {
		return err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return err
	}

	now := time.Now()
	err = usecase.NotificationRepository.MarkRead(ctx, userId, serverId, notifId, now)
	return err
}

func (usecase *NotificationUsecase) GetUnreadCount(ctx context.Context, userId string, serverId string) (model.UnreadCountResponse, error) {
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-usecase").Start(ctx, "usecase.GetUnreadNotificationCount")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	v := util.NewValidator()
	v.UUID("serverId", serverId).Required()
	if err = v.Validate(); err != nil {
		return model.UnreadCountResponse{}, err
	}

	var memberCount int
	memberCount, err = usecase.ServerRepository.CheckServerMember(ctx, serverId, userId)
	if err != nil {
		return model.UnreadCountResponse{}, err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return model.UnreadCountResponse{}, err
	}

	var count int
	count, err = usecase.NotificationRepository.CountUnread(ctx, userId, serverId)
	if err != nil {
		return model.UnreadCountResponse{}, err
	}
	return model.UnreadCountResponse{Count: count}, nil
}

// NotifyDM sends a data-only FCM push for a new DM. Always sent regardless of which
// server is active in the app — the client suppresses display when that conversation is
// in focus. Data message (no Notification block) so the client controls presentation.
func (usecase *NotificationUsecase) NotifyDM(ctx context.Context, recipientUserId, conversationId, senderUsername, preview string) {
	parentSpanCtx := trace.SpanContextFromContext(ctx)
	go func() {
		bg := trace.ContextWithSpanContext(context.Background(), parentSpanCtx)
		bg, cancel := context.WithTimeout(bg, 30*time.Second)
		defer cancel()
		bg, span := otel.Tracer(usecase.Config.String("OTEL_SERVICE_NAME")+"-usecase").Start(bg, "usecase.NotifyDM")
		defer span.End()
		logger := util.GetLoggerWithTraceContext(bg, usecase.Log)
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic in NotifyDM goroutine", zap.Any("recover", rec))
			}
		}()

		tokens, err := usecase.NotificationRepository.ListTokensByUserId(bg, recipientUserId)
		if err != nil {
			logger.Error("notifyDM: list tokens failed", zap.Error(err))
			return
		}
		if len(tokens) == 0 {
			return
		}
		prefs, err := usecase.NotificationRepository.GetUserNotificationPrefs(bg, recipientUserId)
		if err != nil {
			logger.Error("notifyDM: read prefs failed", zap.Error(err))
			return
		}
		if !pushEnabledForType(prefs, "message") {
			return
		}

		msg := &messaging.MulticastMessage{
			Tokens: tokens,
			Data: map[string]string{
				"type":           "message",
				"conversationId": conversationId,
				"senderUsername": senderUsername,
				"preview":        preview,
			},
			Android: &messaging.AndroidConfig{Priority: "high"},
		}
		resp, ferr := usecase.FCMClient.SendEachForMulticast(bg, msg)
		if ferr != nil {
			logger.Error("notifyDM: FCM send failed", zap.Error(ferr))
			return
		}
		var invalid []string
		for i, r := range resp.Responses {
			if !r.Success && (messaging.IsUnregistered(r.Error) || messaging.IsInvalidArgument(r.Error)) {
				invalid = append(invalid, tokens[i])
			}
		}
		if len(invalid) > 0 {
			if e := usecase.NotificationRepository.DeleteInvalidTokens(bg, invalid); e != nil {
				logger.Warn("notifyDM: delete invalid tokens failed", zap.Error(e))
			}
		}
	}()
}

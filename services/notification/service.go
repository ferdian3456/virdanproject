package notification

import (
	"context"
	"fmt"
	"strings"
	"time"

	firebaseMessaging "firebase.google.com/go/v4/messaging"
	"github.com/ferdian3456/virdanproject/services/server"
	"github.com/ferdian3456/virdanproject/shared"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Service struct {
	Repo       *Repository
	ServerRepo *server.Repository
	FCMClient  *firebaseMessaging.Client
	DB         *pgxpool.Pool
	Log        *zap.Logger
	Config     *koanf.Koanf
}

func NewService(repo *Repository, serverRepo *server.Repository, fcmClient *firebaseMessaging.Client, db *pgxpool.Pool, log *zap.Logger, config *koanf.Koanf) *Service {
	return &Service{
		Repo:       repo,
		ServerRepo: serverRepo,
		FCMClient:  fcmClient,
		DB:         db,
		Log:        log,
		Config:     config,
	}
}

func (service *Service) RegisterDevice(ctx context.Context, userId string, request DeviceTokenRegisterRequest) error {
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-service").Start(ctx, "service.RegisterDevice")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
	)

	v := shared.NewValidator()
	v.String("token", request.Token).Required().MaxLen(4096)
	if err = v.Validate(); err != nil {
		return err
	}

	if request.Platform != shared.PLATFORM_ANDROID && request.Platform != shared.PLATFORM_IOS {
		err = &shared.BadRequestError{
			Code:    shared.ERR_VALIDATION_CODE,
			Message: "platform must be android or ios",
			Param:   "platform",
		}
		return err
	}

	tx, err := service.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	err = service.Repo.DeleteAllUserDeviceToken(ctx, tx, userId)
	if err != nil {
		return err
	}

	now := time.Now()
	deviceToken := DeviceToken{
		Id:        uuid.New().String(),
		UserId:    userId,
		Token:     request.Token,
		Platform:  request.Platform,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: userId,
		UpdatedBy: userId,
	}

	err = service.Repo.UpsertDeviceToken(ctx, tx, deviceToken)
	if err != nil {
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, service.Log).Error("Failed to commit transaction", zap.Error(err))
		return err
	}
	return nil
}

func (service *Service) UnregisterDevice(ctx context.Context, userId string, request DeviceTokenDeleteRequest) error {
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-service").Start(ctx, "service.UnregisterDevice")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
	)

	v := shared.NewValidator()
	v.String("token", request.Token).Required()
	if err = v.Validate(); err != nil {
		return err
	}

	err = service.Repo.DeleteDeviceToken(ctx, userId, request.Token)
	if err != nil {
		return err
	}
	return nil
}

func (service *Service) TestSend(ctx context.Context, userId string) error {
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-service").Start(ctx, "service.TestSend")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
	)

	tokens, err := service.Repo.ListTokensByUserId(ctx, userId)
	if err != nil {
		return err
	}

	if len(tokens) == 0 {
		err = &shared.NotFoundError{
			Code:    shared.ERR_NOT_FOUND_CODE,
			Message: "No device registered for this user",
			Param:   "token",
		}
		return err
	}

	message := &firebaseMessaging.MulticastMessage{
		Tokens: tokens,
		Notification: &firebaseMessaging.Notification{
			Title: "Virdan",
			Body:  "Test notification berhasil.",
		},
		Data:    map[string]string{"type": "test"},
		Android: &firebaseMessaging.AndroidConfig{Priority: "high"},
	}

	response, fcmErr := service.FCMClient.SendEachForMulticast(ctx, message)
	if fcmErr != nil {
		shared.GetLoggerWithTraceContext(ctx, service.Log).Error("Failed to send FCM multicast", zap.Error(fcmErr))
		return nil
	}

	var invalidTokens []string
	for i, result := range response.Responses {
		if !result.Success && (firebaseMessaging.IsUnregistered(result.Error) || firebaseMessaging.IsInvalidArgument(result.Error)) {
			invalidTokens = append(invalidTokens, tokens[i])
		}
	}

	if len(invalidTokens) > 0 {
		if deleteErr := service.Repo.DeleteInvalidTokens(ctx, invalidTokens); deleteErr != nil {
			shared.GetLoggerWithTraceContext(ctx, service.Log).Warn("Failed to delete invalid tokens", zap.Error(deleteErr))
		}
	}

	return nil
}

func (service *Service) Notify(ctx context.Context, events []NotificationEvent) {
	parentSpanCtx := trace.SpanContextFromContext(ctx)

	// Clone every string field synchronously, while the caller's request is
	// still in flight. Several of these values (e.g. PostId, ServerId)
	// originate from fiber's ctx.Params(), whose backing buffer is only
	// valid for the lifetime of the handler and gets reused for later
	// requests once it returns. Reading them from the goroutine below
	// without cloning first can observe corrupted, overwritten bytes.
	cloned := make([]NotificationEvent, len(events))
	for i, event := range events {
		event.Type = strings.Clone(event.Type)
		event.RecipientUserId = strings.Clone(event.RecipientUserId)
		event.ActorUserId = strings.Clone(event.ActorUserId)
		event.ActorProfileId = strings.Clone(event.ActorProfileId)
		event.ServerId = strings.Clone(event.ServerId)
		event.PostId = strings.Clone(event.PostId)
		if event.CommentId != nil {
			commentID := strings.Clone(*event.CommentId)
			event.CommentId = &commentID
		}
		cloned[i] = event
	}
	events = cloned

	go func() {
		backgroundCtx := trace.ContextWithSpanContext(context.Background(), parentSpanCtx)
		backgroundCtx, cancel := context.WithTimeout(backgroundCtx, 30*time.Second)
		defer cancel()

		serviceName := service.Config.String("OTEL_SERVICE_NAME")
		backgroundCtx, span := otel.Tracer(serviceName+"-service").Start(backgroundCtx, "service.Notify")
		defer span.End()

		logger := shared.GetLoggerWithTraceContext(backgroundCtx, service.Log)

		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic in Notify goroutine", zap.Any("recover", recovered))
			}
		}()

		priority := map[string]int{"like": 1, "comment": 2, "reply": 3, "mention": 4}
		deduped := make(map[string]NotificationEvent, len(events))
		for _, event := range events {
			existing, exists := deduped[event.RecipientUserId]
			if !exists || priority[event.Type] > priority[existing.Type] {
				deduped[event.RecipientUserId] = event
			}
		}

		minioFullUrl := fmt.Sprintf("%s%s/%s",
			service.Config.String("MINIO_HTTP"),
			service.Config.String("MINIO_URL"),
			service.Config.String("MINIO_BUCKET_NAME"),
		)

		var err error
		for _, event := range deduped {
			now := time.Now()
			postId := &event.PostId
			notif := Notification{
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
			if err = service.Repo.InsertNotification(backgroundCtx, notif); err != nil {
				logger.Error("notif: failed to insert row, skipping recipient",
					zap.String("recipient_user_id", event.RecipientUserId), zap.Error(err))
				continue
			}

			var tokens []string
			if tokens, err = service.Repo.ListTokensByUserId(backgroundCtx, event.RecipientUserId); err != nil {
				logger.Error("notif: failed to list device tokens, skipping push",
					zap.String("recipient_user_id", event.RecipientUserId), zap.Error(err))
				continue
			}
			if len(tokens) == 0 {
				continue
			}

			var prefs NotificationPrefs
			if prefs, err = service.Repo.GetUserNotificationPrefs(backgroundCtx, event.RecipientUserId); err != nil {
				logger.Error("notif: failed to read prefs, skipping push (fail-closed)",
					zap.String("recipient_user_id", event.RecipientUserId), zap.Error(err))
				continue
			}
			if !pushEnabledForType(prefs, event.Type) {
				continue
			}

			var actorUsername string
			if actorUsername, _, err = service.Repo.GetActorUsernameAndAvatar(backgroundCtx, event.ActorProfileId, minioFullUrl); err != nil {
				logger.Error("notif: failed to resolve actor username, skipping push",
					zap.String("actor_profile_id", event.ActorProfileId), zap.Error(err))
				continue
			}

			service.sendPush(backgroundCtx, tokens, actorUsername, event)
		}
	}()
}

func (service *Service) sendPush(ctx context.Context, tokens []string, actorUsername string, event NotificationEvent) {
	body := notifBodyForType(event.Type)

	data := map[string]string{
		"type":     event.Type,
		"serverId": event.ServerId,
		"postId":   event.PostId,
	}
	if event.CommentId != nil {
		data["commentId"] = *event.CommentId
	}

	message := &firebaseMessaging.MulticastMessage{
		Tokens: tokens,
		Notification: &firebaseMessaging.Notification{
			Title: actorUsername,
			Body:  body,
		},
		Data:    data,
		Android: &firebaseMessaging.AndroidConfig{Priority: "high"},
	}

	response, fcmErr := service.FCMClient.SendEachForMulticast(ctx, message)
	if fcmErr != nil {
		shared.GetLoggerWithTraceContext(ctx, service.Log).Error("Failed to send FCM push", zap.Error(fcmErr))
		return
	}

	var invalidTokens []string
	for i, result := range response.Responses {
		if !result.Success && (firebaseMessaging.IsUnregistered(result.Error) || firebaseMessaging.IsInvalidArgument(result.Error)) {
			invalidTokens = append(invalidTokens, tokens[i])
		}
	}

	if len(invalidTokens) > 0 {
		if deleteErr := service.Repo.DeleteInvalidTokens(ctx, invalidTokens); deleteErr != nil {
			shared.GetLoggerWithTraceContext(ctx, service.Log).Warn("Failed to delete invalid tokens after push", zap.Error(deleteErr))
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

func pushEnabledForType(prefs NotificationPrefs, notifType string) bool {
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

func (service *Service) GetFeed(ctx context.Context, userId string, serverId string, cursorStr string, limit int) (NotificationListResponse, error) {
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-service").Start(ctx, "service.GetNotificationFeed")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	if err = v.Validate(); err != nil {
		return NotificationListResponse{}, err
	}

	var memberCount int
	memberCount, err = service.ServerRepo.CheckServerMember(ctx, serverId, userId)
	if err != nil {
		return NotificationListResponse{}, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return NotificationListResponse{}, err
	}

	if limit <= 0 {
		limit = shared.DEFAULT_LIMIT
	}
	if limit > shared.MAX_LIMIT {
		limit = shared.MAX_LIMIT
	}

	var cursor *NotificationCursor
	if cursorStr != "" {
		cursor, err = shared.DecodeCursor[NotificationCursor](cursorStr)
		if err != nil {
			err = &shared.BadRequestError{Code: shared.ERR_VALIDATION_CODE, Message: "Invalid cursor", Param: "cursor"}
			return NotificationListResponse{}, err
		}
	}

	minioFullUrl := fmt.Sprintf("%s%s/%s",
		service.Config.String("MINIO_HTTP"),
		service.Config.String("MINIO_URL"),
		service.Config.String("MINIO_BUCKET_NAME"),
	)

	var items []NotificationResponse
	items, err = service.Repo.ListByRecipient(ctx, userId, serverId, cursor, limit+1, minioFullUrl)
	if err != nil {
		return NotificationListResponse{}, err
	}

	response := NotificationListResponse{Data: []NotificationResponse{}}
	if len(items) > limit {
		response.Data = items[:limit]
		last := items[limit-1]
		response.Page.NextCursor = shared.EncodeCursor(NotificationCursor{
			CreatedAt: last.CreatedAt,
			Id:        last.Id,
		})
	} else {
		response.Data = items
	}

	return response, nil
}

func (service *Service) MarkRead(ctx context.Context, userId string, serverId string, notifId string) error {
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-service").Start(ctx, "service.MarkNotificationRead")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
		attribute.String("notification.id", notifId),
	)

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	v.UUID("id", notifId)
	if err = v.Validate(); err != nil {
		return err
	}

	var memberCount int
	memberCount, err = service.ServerRepo.CheckServerMember(ctx, serverId, userId)
	if err != nil {
		return err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return err
	}

	now := time.Now()
	err = service.Repo.MarkRead(ctx, userId, serverId, notifId, now)
	return err
}

func (service *Service) GetUnreadCount(ctx context.Context, userId string, serverId string) (UnreadCountResponse, error) {
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-service").Start(ctx, "service.GetUnreadNotificationCount")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	if err = v.Validate(); err != nil {
		return UnreadCountResponse{}, err
	}

	var memberCount int
	memberCount, err = service.ServerRepo.CheckServerMember(ctx, serverId, userId)
	if err != nil {
		return UnreadCountResponse{}, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return UnreadCountResponse{}, err
	}

	var count int
	count, err = service.Repo.CountUnread(ctx, userId, serverId)
	if err != nil {
		return UnreadCountResponse{}, err
	}
	return UnreadCountResponse{Count: count}, nil
}

func (service *Service) NotifyDM(ctx context.Context, recipientUserId, conversationId, senderUsername, preview string) {
	parentSpanCtx := trace.SpanContextFromContext(ctx)

	// Clone synchronously before spawning the goroutine: conversationId in
	// particular typically originates from fiber's ctx.Params(), whose
	// backing buffer is only valid for the handler's lifetime.
	recipientUserId = strings.Clone(recipientUserId)
	conversationId = strings.Clone(conversationId)
	senderUsername = strings.Clone(senderUsername)
	preview = strings.Clone(preview)

	go func() {
		bg := trace.ContextWithSpanContext(context.Background(), parentSpanCtx)
		bg, cancel := context.WithTimeout(bg, 30*time.Second)
		defer cancel()
		bg, span := otel.Tracer(service.Config.String("OTEL_SERVICE_NAME")+"-service").Start(bg, "service.NotifyDM")
		defer span.End()
		logger := shared.GetLoggerWithTraceContext(bg, service.Log)
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic in NotifyDM goroutine", zap.Any("recover", rec))
			}
		}()

		tokens, err := service.Repo.ListTokensByUserId(bg, recipientUserId)
		if err != nil {
			logger.Error("notifyDM: list tokens failed", zap.Error(err))
			return
		}
		if len(tokens) == 0 {
			return
		}
		prefs, err := service.Repo.GetUserNotificationPrefs(bg, recipientUserId)
		if err != nil {
			logger.Error("notifyDM: read prefs failed", zap.Error(err))
			return
		}
		if !pushEnabledForType(prefs, "message") {
			return
		}

		msg := &firebaseMessaging.MulticastMessage{
			Tokens: tokens,
			Data: map[string]string{
				"type":           "message",
				"conversationId": conversationId,
				"senderUsername": senderUsername,
				"preview":        preview,
			},
			Notification: &firebaseMessaging.Notification{
				Title: senderUsername,
				Body:  preview,
			},
			Android: &firebaseMessaging.AndroidConfig{
				Priority: "high",
				Notification: &firebaseMessaging.AndroidNotification{
					ChannelID: "virdan_high_importance",
				},
			},
			APNS: &firebaseMessaging.APNSConfig{
				Payload: &firebaseMessaging.APNSPayload{
					Aps: &firebaseMessaging.Aps{Sound: "default"},
				},
			},
		}
		resp, ferr := service.FCMClient.SendEachForMulticast(bg, msg)
		if ferr != nil {
			logger.Error("notifyDM: FCM send failed", zap.Error(ferr))
			return
		}
		var invalid []string
		for i, r := range resp.Responses {
			if !r.Success && (firebaseMessaging.IsUnregistered(r.Error) || firebaseMessaging.IsInvalidArgument(r.Error)) {
				invalid = append(invalid, tokens[i])
			}
		}
		if len(invalid) > 0 {
			if e := service.Repo.DeleteInvalidTokens(bg, invalid); e != nil {
				logger.Warn("notifyDM: delete invalid tokens failed", zap.Error(e))
			}
		}
	}()
}

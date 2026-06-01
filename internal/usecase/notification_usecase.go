package usecase

import (
	"context"
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
	"go.uber.org/zap"
)

type NotificationUsecase struct {
	NotificationRepository *repository.NotificationRepository
	FCMClient              *messaging.Client
	DB                     *pgxpool.Pool
	Log                    *zap.Logger
	Config                 *koanf.Koanf
}

func NewNotificationUsecase(
	notificationRepository *repository.NotificationRepository,
	fcmClient *messaging.Client,
	db *pgxpool.Pool,
	log *zap.Logger,
	config *koanf.Koanf,
) *NotificationUsecase {
	return &NotificationUsecase{
		NotificationRepository: notificationRepository,
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

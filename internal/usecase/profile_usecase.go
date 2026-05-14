package usecase

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/repository"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

type ProfileUsecase struct {
	ProfileRepository *repository.ProfileRepository
	ServerRepository  *repository.ServerRepository
	DB                *pgxpool.Pool
	Log               *zap.Logger
	Config            *koanf.Koanf
}

func NewProfileUsecase(
	profileRepository *repository.ProfileRepository,
	serverRepository *repository.ServerRepository,
	db *pgxpool.Pool,
	log *zap.Logger,
	config *koanf.Koanf,
) *ProfileUsecase {
	return &ProfileUsecase{
		ProfileRepository: profileRepository,
		ServerRepository:  serverRepository,
		DB:                db,
		Log:               log,
		Config:            config,
	}
}

func (usecase *ProfileUsecase) GetProfileHistory(ctx fiber.Ctx, userId string) (model.GetProfileHistoryResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.GetProfileHistory")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	var response model.GetProfileHistoryResponse

	minioFullUrl := usecase.Config.String("MINIO_HTTP") + usecase.Config.String("MINIO_URL") + "/" + usecase.Config.String("MINIO_BUCKET_NAME")

	var items []model.GetProfileHistoryResponseItem
	items, err = usecase.ProfileRepository.GetProfileHistory(ctxContext, userId, minioFullUrl)
	if err != nil {
		return response, err
	}

	response.Data = items
	return response, nil
}

func (usecase *ProfileUsecase) GetServerProfileMe(ctx fiber.Ctx, userId, serverId string) (model.ServerMemberProfileResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.GetServerProfileMe")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.ServerMemberProfileResponse

	v := util.NewValidator()
	v.UUID("serverId", serverId).Required()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
	)

	minioFullUrl := usecase.Config.String("MINIO_HTTP") + usecase.Config.String("MINIO_URL") + "/" + usecase.Config.String("MINIO_BUCKET_NAME")

	response, err = usecase.ProfileRepository.GetServerMemberProfile(ctxContext, serverId, userId, minioFullUrl)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (usecase *ProfileUsecase) UpdateServerProfile(ctx fiber.Ctx, userId, serverId string, payload model.ServerProfileUpdateRequest) (model.ServerProfileUpdateResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.UpdateServerProfile")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.ServerProfileUpdateResponse

	v := util.NewValidator()
	v.UUID("serverId", serverId).Required()
	v.String("nickname", payload.Nickname).Required().MinLen(3).MaxLen(50).Nickname()
	if payload.Bio != nil {
		v.String("bio", *payload.Bio).MaxLen(500)
	}
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
		attribute.String("profile.nickname", payload.Nickname),
	)

	var memberCount int
	memberCount, err = usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{
			Code:    constant.ERR_FORBIDDEN_CODE,
			Message: "You are not a member of this server",
			Param:   "serverId",
		}
		return response, err
	}

	var profileId string
	var profileExists bool
	profileId, profileExists, err = usecase.ProfileRepository.TryGetServerMemberProfileIdNoTx(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}
	if !profileExists {
		err = &model.NotFoundError{
			Code:    constant.ERR_NOT_FOUND_CODE,
			Message: "Profile not found in this server",
			Param:   "serverId",
		}
		return response, err
	}

	var bioPtr *string
	if payload.Bio != nil {
		trimmed := strings.TrimSpace(*payload.Bio)
		if trimmed != "" {
			bioPtr = util.ToPtr(trimmed)
		}
	}

	now := time.Now().UTC()
	err = usecase.ProfileRepository.UpdateServerProfileNickBio(ctxContext, profileId,
		payload.Nickname, bioPtr, userId, now)
	if err != nil {
		return response, err
	}

	response.ProfileId = profileId
	response.UpdatedAt = now
	return response, nil
}

func (usecase *ProfileUsecase) UpdateServerProfileAvatar(ctx fiber.Ctx, userId, serverId string) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.UpdateServerProfileAvatar")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
	)

	v := util.NewValidator()
	v.UUID("serverId", serverId).Required()
	err = v.Validate()
	if err != nil {
		return err
	}

	var memberCount int
	memberCount, err = usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{
			Code:    constant.ERR_FORBIDDEN_CODE,
			Message: "You are not a member of this server",
			Param:   "serverId",
		}
		return err
	}

	var imageFile *bytes.Reader
	var imageSize int64
	imageFile, imageSize, err = util.ExtractAndValidateImage(ctx, ctxContext, "avatar")
	if err != nil {
		return err
	}

	var tx pgx.Tx
	tx, err = usecase.DB.Begin(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to begin transaction", zap.Error(err))
		return err
	}
	defer func() { _ = tx.Rollback(ctxContext) }()

	now := time.Now().UTC()
	bucket := usecase.Config.String("MINIO_BUCKET_NAME")

	var profileId string
	profileId, err = usecase.ProfileRepository.GetProfileId(ctxContext, tx, serverId, userId)
	if err != nil {
		return err
	}

	newAvatarImageId := uuid.New().String()
	newAvatarImage := model.ProfileAvatarImage{
		Id:        newAvatarImageId,
		Bucket:    bucket,
		ObjectKey: fmt.Sprintf("profile/avatar/%s.webp", newAvatarImageId),
		MimeType:  "image/webp",
		Size:      imageSize,
		CreatedAt: now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = usecase.ProfileRepository.CreateProfileAvatarImage(ctxContext, tx, newAvatarImage)
	if err != nil {
		return err
	}

	err = usecase.ProfileRepository.UpdateProfileAvatarImageId(ctxContext, tx, profileId, util.ToPtr(newAvatarImageId), userId, now)
	if err != nil {
		return err
	}

	err = usecase.ProfileRepository.UploadObject(ctxContext, bucket,
		fmt.Sprintf("profile/avatar/%s.webp", newAvatarImageId), imageFile, imageSize)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to upload avatar object", zap.Error(err))
		return err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

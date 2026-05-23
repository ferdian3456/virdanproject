package usecase

import (
	"strings"
	"time"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/repository"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/gofiber/fiber/v3"
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

// UpdateServerProfile is a consolidated multipart endpoint: nickname +
// username + bio plus an optional profileAvatar (file) OR avatarImageId
// (existing image). All fields update in a single transaction.
func (usecase *ProfileUsecase) UpdateServerProfile(ctx fiber.Ctx, userId, serverId string) (model.ServerProfileUpdateResponse, error) {
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

	nickname := ctx.FormValue("nickname")
	username := ctx.FormValue("username")
	bio := ctx.FormValue("bio")

	v := util.NewValidator()
	v.UUID("serverId", serverId).Required()
	v.String("nickname", nickname).Required().MinLen(3).MaxLen(50).Nickname()
	v.String("username", username).Required().MinLen(3).MaxLen(22).Regex(util.UsernameRegex, util.UsernameErrorText)
	v.String("bio", bio).MaxLen(500)
	err = v.Validate()
	if err != nil {
		return response, err
	}

	username = strings.ToLower(strings.TrimSpace(username))

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
		attribute.String("profile.nickname", nickname),
		attribute.String("profile.username", username),
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

	var bioPtr *string
	trimmedBio := strings.TrimSpace(bio)
	if trimmedBio != "" {
		bioPtr = util.ToPtr(trimmedBio)
	}

	var tx pgx.Tx
	tx, err = usecase.DB.Begin(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to begin transaction", zap.Error(err))
		return response, err
	}
	defer func() { _ = tx.Rollback(ctxContext) }()

	now := time.Now().UTC()

	var profileId string
	profileId, err = usecase.ProfileRepository.GetProfileId(ctxContext, tx, serverId, userId)
	if err != nil {
		return response, err
	}

	// Avatar is optional: profileAvatar file XOR avatarImageId XOR neither
	// (leave existing avatar untouched). ResolveProfileAvatar enforces
	// mutual exclusion, validates ownership, and uploads the file when
	// present.
	var newAvatarImageId *string
	newAvatarImageId, err = util.ResolveProfileAvatar(
		ctxContext, tx, ctx,
		usecase.ProfileRepository,
		usecase.Config,
		usecase.Log,
		userId,
		now,
	)
	if err != nil {
		return response, err
	}

	if newAvatarImageId != nil {
		err = usecase.ProfileRepository.UpdateServerProfileFull(ctxContext, tx, profileId,
			nickname, username, bioPtr, newAvatarImageId, userId, now)
	} else {
		err = usecase.ProfileRepository.UpdateServerProfileNickBioTx(ctxContext, tx, profileId,
			nickname, username, bioPtr, userId, now)
	}
	if err != nil {
		return response, err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to commit transaction", zap.Error(err))
		return response, err
	}

	response.ProfileId = profileId
	response.UpdatedAt = now
	return response, nil
}


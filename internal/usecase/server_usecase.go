package usecase

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
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

type ServerUsecase struct {
	ServerRepository  *repository.ServerRepository
	ProfileRepository *repository.ProfileRepository
	DB                *pgxpool.Pool
	Log               *zap.Logger
	Config            *koanf.Koanf
}

func NewServerUsecase(
	serverRepository *repository.ServerRepository,
	profileRepository *repository.ProfileRepository,
	db *pgxpool.Pool,
	log *zap.Logger,
	config *koanf.Koanf,
) *ServerUsecase {
	return &ServerUsecase{
		ServerRepository:  serverRepository,
		ProfileRepository: profileRepository,
		DB:                db,
		Log:               log,
		Config:            config,
	}
}

func (usecase *ServerUsecase) CreateServer(ctx fiber.Ctx, userId string) (model.ServerCreateResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.CreateServer")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	var response model.ServerCreateResponse

	name := ctx.FormValue("name")
	shortName := ctx.FormValue("shortName")
	description := ctx.FormValue("description")
	categoryIdStr := ctx.FormValue("categoryId")
	isPrivateStr := ctx.FormValue("isPrivate")
	nickname := ctx.FormValue("nickname")
	bio := ctx.FormValue("bio")
	avatarImageIdRaw := ctx.FormValue("avatarImageId")

	v := util.NewValidator()
	v.String("name", name).Required().MinLen(3).MaxLen(40)
	v.String("shortName", shortName).Required().MinLen(2).MaxLen(10)
	v.String("description", description).MaxLen(500)
	v.String("categoryId", categoryIdStr).Required()
	v.String("isPrivate", isPrivateStr).Required()
	v.String("nickname", nickname).Required().MinLen(3).MaxLen(50)
	v.String("bio", bio).MaxLen(500)
	if avatarImageIdRaw != "" {
		v.UUID("avatarImageId", avatarImageIdRaw)
	}
	err = v.Validate()
	if err != nil {
		return response, err
	}

	var categoryId int
	categoryId, err = strconv.Atoi(categoryIdStr)
	if err != nil {
		err = &model.BadRequestError{Code: constant.ERR_VALIDATION_CODE, Message: "categoryId must be int", Param: "categoryId"}
		return response, err
	}
	isPrivate := isPrivateStr == "true"

	var catCount int
	catCount, err = usecase.ServerRepository.CheckServerCategories(ctxContext, categoryId)
	if err != nil {
		return response, err
	}
	if catCount == 0 {
		err = &model.NotFoundError{Code: constant.ERR_NOT_FOUND_CODE, Message: "Category not found", Param: "categoryId"}
		return response, err
	}

	var avatarImageIdPtr *string
	if avatarImageIdRaw != "" {
		var owned bool
		owned, err = usecase.ProfileRepository.CheckProfileAvatarImageOwnership(ctxContext, userId, avatarImageIdRaw)
		if err != nil {
			return response, err
		}
		if !owned {
			err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "Avatar image is not owned by you", Param: "avatarImageId"}
			return response, err
		}
		avatarImageIdPtr = util.ToPtr(avatarImageIdRaw)
	}

	var serverAvatarFile *bytes.Reader
	var serverAvatarSize int64
	var serverAvatarImageId *string
	fh, fhErr := ctx.FormFile("avatar")
	if fhErr == nil && fh != nil {
		serverAvatarFile, serverAvatarSize, err = util.ValidateImage(ctxContext, fh, "avatar")
		if err != nil {
			return response, err
		}
		serverAvatarImageId = util.ToPtr(uuid.New().String())
	}

	var tx pgx.Tx
	tx, err = usecase.DB.Begin(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to begin transaction", zap.Error(err))
		return response, err
	}
	defer func() { _ = tx.Rollback(ctxContext) }()

	now := time.Now().UTC()
	serverId := uuid.New().String()

	if serverAvatarImageId != nil {
		avatarImage := model.ServerAvatarImage{
			Id:        *serverAvatarImageId,
			Bucket:    usecase.Config.String("MINIO_BUCKET_NAME"),
			ObjectKey: fmt.Sprintf("server/avatar/%s.webp", *serverAvatarImageId),
			MimeType:  "image/webp",
			Size:      serverAvatarSize,
			CreatedAt: now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
		}
		err = usecase.ServerRepository.CreateServerAvatarImage(ctxContext, tx, avatarImage)
		if err != nil {
			return response, err
		}
	}

	var descPtr *string
	if description != "" {
		descPtr = util.ToPtr(description)
	}

	initialSettings := model.ServerSettings{IsPrivate: isPrivate}
	var settingsBytes []byte
	settingsBytes, err = sonic.Marshal(initialSettings)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to marshal server settings", zap.Error(err))
		return response, err
	}

	server := model.Server{
		Id:            serverId,
		OwnerId:       userId,
		Name:          name,
		ShortName:     shortName,
		AvatarImageId: serverAvatarImageId,
		CategoryId:    util.ToPtr(categoryId),
		Description:   descPtr,
		Settings:      settingsBytes,
		CreatedAt:     now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = usecase.ServerRepository.CreateServer(ctxContext, tx, server)
	if err != nil {
		return response, err
	}

	ownerRoleId := uuid.New().String()
	ownerRole := model.ServerRole{
		Id:          ownerRoleId,
		ServerId:    serverId,
		Name:        "Owner",
		Permissions: []byte(`{"all":true}`),
		CreatedAt:   now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = usecase.ServerRepository.CreateServerRole(ctxContext, tx, ownerRole)
	if err != nil {
		return response, err
	}

	memberRole := model.ServerRole{
		Id:          uuid.New().String(),
		ServerId:    serverId,
		Name:        "Member",
		Permissions: []byte(`{}`),
		CreatedAt:   now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = usecase.ServerRepository.CreateServerRole(ctxContext, tx, memberRole)
	if err != nil {
		return response, err
	}

	member := model.ServerMember{
		Id:           uuid.New().String(),
		ServerId:     serverId,
		UserId:       userId,
		ServerRoleId: ownerRoleId,
		JoinedAt:     now,
		CreatedAt:    now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = usecase.ServerRepository.CreateServerMember(ctxContext, tx, member)
	if err != nil {
		return response, err
	}

	var bioPtr *string
	if bio != "" {
		bioPtr = util.ToPtr(bio)
	}

	profileId := uuid.New().String()
	profile := model.ServerMemberProfile{
		Id:            profileId,
		ServerId:      serverId,
		UserId:        userId,
		Nickname:      nickname,
		Bio:           bioPtr,
		AvatarImageId: avatarImageIdPtr,
		CreatedAt:     now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = usecase.ProfileRepository.CreateServerMemberProfile(ctxContext, tx, profile)
	if err != nil {
		return response, err
	}

	if serverAvatarImageId != nil {
		bucket := usecase.Config.String("MINIO_BUCKET_NAME")
		objectKey := fmt.Sprintf("server/avatar/%s.webp", *serverAvatarImageId)
		err = usecase.ServerRepository.UploadObject(ctxContext, bucket, objectKey, serverAvatarFile, serverAvatarSize)
		if err != nil {
			util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to upload server avatar", zap.Error(err))
			return response, err
		}
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to commit transaction", zap.Error(err))
		return response, err
	}

	response.Server = model.ServerDetailResponse{
		Id:            serverId,
		Name:          name,
		ShortName:     shortName,
		CategoryId:    util.ToPtr(categoryId),
		Description:   descPtr,
		OwnerId:       userId,
		OwnerNickname: nickname,
		MemberCount:   1,
		IsMember:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if serverAvatarImageId != nil {
		minioFullUrl := usecase.Config.String("MINIO_HTTP") + usecase.Config.String("MINIO_URL")
		url := fmt.Sprintf("%s/%s/server/avatar/%s.webp", minioFullUrl, usecase.Config.String("MINIO_BUCKET_NAME"), *serverAvatarImageId)
		response.Server.AvatarImageUrl = &url
	}
	response.Identity = model.ServerMemberProfileResponse{
		ProfileId:     profileId,
		Nickname:      nickname,
		Bio:           bioPtr,
		AvatarImageId: avatarImageIdPtr,
		CreatedAt:     now, UpdatedAt: now,
	}

	return response, nil
}

func (usecase *ServerUsecase) JoinServer(ctx fiber.Ctx, userId, serverId string, payload model.ServerJoinDirectRequest) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.JoinServer")
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
	v.String("nickname", payload.Nickname).Required().MinLen(3).MaxLen(50)
	v.String("bio", util.Deref(payload.Bio, "")).MaxLen(500)
	if payload.AvatarImageId != nil && *payload.AvatarImageId != "" {
		v.UUID("avatarImageId", *payload.AvatarImageId)
	}
	err = v.Validate()
	if err != nil {
		return err
	}

	var serverInfo model.ServerCheckEligibleInfo
	serverInfo, err = usecase.ServerRepository.CheckServerEligibleForJoin(ctxContext, serverId)
	if err != nil {
		return err
	}
	if !serverInfo.Exists {
		err = &model.NotFoundError{Code: constant.ERR_NOT_FOUND_CODE, Message: "Server not found", Param: "serverId"}
		return err
	}
	if serverInfo.IsPrivate {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "Server is private. Use invite code.", Param: "serverId"}
		return err
	}

	var memberCount int
	memberCount, err = usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return err
	}
	if memberCount > 0 {
		err = &model.ConflictError{Code: constant.ERR_CONFLICT_CODE, Message: "Already a member of this server", Param: "serverId"}
		return err
	}

	if payload.AvatarImageId != nil && *payload.AvatarImageId != "" {
		var owned bool
		owned, err = usecase.ProfileRepository.CheckProfileAvatarImageOwnership(ctxContext, userId, *payload.AvatarImageId)
		if err != nil {
			return err
		}
		if !owned {
			err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "Avatar image is not owned by you", Param: "avatarImageId"}
			return err
		}
	}

	var memberRoleId string
	memberRoleId, err = usecase.ServerRepository.GetRoleByName(ctxContext, serverId, "Member")
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

	var existingProfileId string
	var profileExists bool
	existingProfileId, profileExists, err = usecase.ProfileRepository.TryGetServerMemberProfileId(ctxContext, tx, serverId, userId)
	if err != nil {
		return err
	}

	if profileExists {
		err = usecase.ProfileRepository.UpdateServerProfileFull(ctxContext, tx, existingProfileId,
			payload.Nickname, payload.Bio, payload.AvatarImageId, userId, now)
		if err != nil {
			return err
		}
	} else {
		profile := model.ServerMemberProfile{
			Id:            uuid.New().String(),
			ServerId:      serverId,
			UserId:        userId,
			Nickname:      payload.Nickname,
			Bio:           payload.Bio,
			AvatarImageId: payload.AvatarImageId,
			CreatedAt:     now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
		}
		err = usecase.ProfileRepository.CreateServerMemberProfile(ctxContext, tx, profile)
		if err != nil {
			return err
		}
	}

	member := model.ServerMember{
		Id:           uuid.New().String(),
		ServerId:     serverId,
		UserId:       userId,
		ServerRoleId: memberRoleId,
		JoinedAt:     now,
		CreatedAt:    now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = usecase.ServerRepository.CreateServerMember(ctxContext, tx, member)
	if err != nil {
		return err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

func (usecase *ServerUsecase) JoinServerFromInvite(ctx fiber.Ctx, userId string, payload model.ServerJoinByInviteRequest) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.JoinServerFromInvite")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("invite.code", payload.InviteCode),
	)

	v := util.NewValidator()
	v.String("inviteCode", payload.InviteCode).Required().MinLen(8).MaxLen(8)
	v.String("nickname", payload.Nickname).Required().MinLen(3).MaxLen(50)
	v.String("bio", util.Deref(payload.Bio, "")).MaxLen(500)
	if payload.AvatarImageId != nil && *payload.AvatarImageId != "" {
		v.UUID("avatarImageId", *payload.AvatarImageId)
	}
	err = v.Validate()
	if err != nil {
		return err
	}

	var serverId string
	serverId, err = usecase.ServerRepository.ValidateAndConsumeInvite(ctxContext, payload.InviteCode)
	if err != nil {
		return err
	}

	span.SetAttributes(attribute.String("server.id", serverId))

	var memberCount int
	memberCount, err = usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return err
	}
	if memberCount > 0 {
		err = &model.ConflictError{Code: constant.ERR_CONFLICT_CODE, Message: "Already a member of this server", Param: "inviteCode"}
		return err
	}

	if payload.AvatarImageId != nil && *payload.AvatarImageId != "" {
		var owned bool
		owned, err = usecase.ProfileRepository.CheckProfileAvatarImageOwnership(ctxContext, userId, *payload.AvatarImageId)
		if err != nil {
			return err
		}
		if !owned {
			err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "Avatar image is not owned by you", Param: "avatarImageId"}
			return err
		}
	}

	var memberRoleId string
	memberRoleId, err = usecase.ServerRepository.GetRoleByName(ctxContext, serverId, "Member")
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

	var existingProfileId string
	var profileExists bool
	existingProfileId, profileExists, err = usecase.ProfileRepository.TryGetServerMemberProfileId(ctxContext, tx, serverId, userId)
	if err != nil {
		return err
	}

	if profileExists {
		err = usecase.ProfileRepository.UpdateServerProfileFull(ctxContext, tx, existingProfileId,
			payload.Nickname, payload.Bio, payload.AvatarImageId, userId, now)
		if err != nil {
			return err
		}
	} else {
		profile := model.ServerMemberProfile{
			Id:       uuid.New().String(),
			ServerId: serverId, UserId: userId,
			Nickname: payload.Nickname, Bio: payload.Bio, AvatarImageId: payload.AvatarImageId,
			CreatedAt: now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
		}
		err = usecase.ProfileRepository.CreateServerMemberProfile(ctxContext, tx, profile)
		if err != nil {
			return err
		}
	}

	member := model.ServerMember{
		Id:           uuid.New().String(),
		ServerId:     serverId, UserId: userId,
		ServerRoleId: memberRoleId, JoinedAt: now,
		CreatedAt: now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = usecase.ServerRepository.CreateServerMember(ctxContext, tx, member)
	if err != nil {
		return err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

func (usecase *ServerUsecase) GetServerInfoForInvite(ctx fiber.Ctx, inviteCode string) (model.ServerInfoForInviteResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.GetServerInfoForInvite")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.ServerInfoForInviteResponse

	v := util.NewValidator()
	v.String("inviteCode", inviteCode).Required().MinLen(8).MaxLen(8)
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("invite.code", inviteCode))

	minioFullUrl := usecase.Config.String("MINIO_HTTP") + usecase.Config.String("MINIO_URL") + "/" + usecase.Config.String("MINIO_BUCKET_NAME")
	response, err = usecase.ServerRepository.GetServerInfoForInvite(ctxContext, inviteCode, minioFullUrl)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (usecase *ServerUsecase) GetServerById(ctx fiber.Ctx, userId, serverId string) (model.ServerDetailResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.GetServerById")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.ServerDetailResponse

	v := util.NewValidator()
	v.UUID("serverId", serverId).Required()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	minioFullUrl := usecase.Config.String("MINIO_HTTP") + usecase.Config.String("MINIO_URL") + "/" + usecase.Config.String("MINIO_BUCKET_NAME")
	response, err = usecase.ServerRepository.GetServerById(ctxContext, serverId, userId, minioFullUrl)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (usecase *ServerUsecase) UpdateServerName(ctx fiber.Ctx, userId, serverId string, payload model.ServerUpdateNameRequest) (model.ServerUpdateResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.UpdateServerName")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.ServerUpdateResponse

	v := util.NewValidator()
	v.UUID("serverId", serverId).Required()
	v.String("name", payload.Name).Required().MinLen(3).MaxLen(40)
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	var ownerCount int
	ownerCount, err = usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}
	if ownerCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not the owner of this server", Param: "serverId"}
		return response, err
	}

	now := time.Now().UTC()
	err = usecase.ServerRepository.UpdateServerName(ctxContext, serverId, payload.Name, userId, now)
	if err != nil {
		return response, err
	}

	response.Id = serverId
	response.UpdatedAt = now
	return response, nil
}

func (usecase *ServerUsecase) UpdateServerShortName(ctx fiber.Ctx, userId, serverId string, payload model.ServerUpdateShortNameRequest) (model.ServerUpdateResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.UpdateServerShortName")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.ServerUpdateResponse

	v := util.NewValidator()
	v.UUID("serverId", serverId).Required()
	v.String("shortName", payload.ShortName).Required().MinLen(2).MaxLen(10)
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	var ownerCount int
	ownerCount, err = usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}
	if ownerCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not the owner of this server", Param: "serverId"}
		return response, err
	}

	now := time.Now().UTC()
	err = usecase.ServerRepository.UpdateServerShortName(ctxContext, serverId, payload.ShortName, userId, now)
	if err != nil {
		return response, err
	}

	response.Id = serverId
	response.UpdatedAt = now
	return response, nil
}

func (usecase *ServerUsecase) UpdateServerCategory(ctx fiber.Ctx, userId, serverId string, payload model.ServerUpdateCategoryRequest) (model.ServerUpdateResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.UpdateServerCategory")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.ServerUpdateResponse

	v := util.NewValidator()
	v.UUID("serverId", serverId).Required()
	v.Int("categoryId", payload.CategoryId).Required().Positive()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
		attribute.Int("server.category_id", payload.CategoryId),
	)

	var ownerCount int
	ownerCount, err = usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}
	if ownerCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not the owner of this server", Param: "serverId"}
		return response, err
	}

	var categoryExists bool
	categoryExists, err = usecase.ServerRepository.CheckCategoryActive(ctxContext, payload.CategoryId)
	if err != nil {
		return response, err
	}
	if !categoryExists {
		err = &model.NotFoundError{Code: constant.ERR_NOT_FOUND_CODE, Message: "Category not found or inactive", Param: "categoryId"}
		return response, err
	}

	now := time.Now().UTC()
	err = usecase.ServerRepository.UpdateServerCategory(ctxContext, serverId, payload.CategoryId, userId, now)
	if err != nil {
		return response, err
	}

	response.Id = serverId
	response.UpdatedAt = now
	return response, nil
}

func (usecase *ServerUsecase) UpdateServerDescription(ctx fiber.Ctx, userId, serverId string, payload model.ServerUpdateDescriptionRequest) (model.ServerUpdateResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.UpdateServerDescription")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.ServerUpdateResponse

	v := util.NewValidator()
	v.UUID("serverId", serverId).Required()
	v.String("description", payload.Description).MaxLen(2000)
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	var ownerCount int
	ownerCount, err = usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}
	if ownerCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not the owner of this server", Param: "serverId"}
		return response, err
	}

	now := time.Now().UTC()
	var descPtr *string
	if payload.Description != "" {
		descPtr = util.ToPtr(payload.Description)
	}
	err = usecase.ServerRepository.UpdateServerDescription(ctxContext, serverId, descPtr, userId, now)
	if err != nil {
		return response, err
	}

	response.Id = serverId
	response.UpdatedAt = now
	return response, nil
}

func (usecase *ServerUsecase) UpdateServerSettings(ctx fiber.Ctx, userId, serverId string, payload model.ServerUpdateSettingsRequest) (model.ServerUpdateResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.UpdateServerSettings")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.ServerUpdateResponse

	v := util.NewValidator()
	v.UUID("serverId", serverId).Required()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
		attribute.Bool("server.is_private", payload.IsPrivate),
	)

	var ownerCount int
	ownerCount, err = usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}
	if ownerCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not the owner of this server", Param: "serverId"}
		return response, err
	}

	settings := model.ServerSettings{IsPrivate: payload.IsPrivate}
	var settingsBytes []byte
	settingsBytes, err = sonic.Marshal(settings)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to marshal server settings", zap.Error(err))
		return response, err
	}

	now := time.Now().UTC()
	err = usecase.ServerRepository.UpdateServerSettings(ctxContext, serverId, settingsBytes, userId, now)
	if err != nil {
		return response, err
	}

	response.Id = serverId
	response.UpdatedAt = now
	return response, nil
}

func (usecase *ServerUsecase) UpdateServerAvatar(ctx fiber.Ctx, userId, serverId string) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.UpdateServerAvatar")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	v := util.NewValidator()
	v.UUID("serverId", serverId).Required()
	err = v.Validate()
	if err != nil {
		return err
	}

	var ownerCount int
	ownerCount, err = usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return err
	}
	if ownerCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not the owner of this server"}
		return err
	}

	fh, err := ctx.FormFile("avatar")
	if err != nil {
		err = &model.BadRequestError{Code: constant.ERR_VALIDATION_CODE, Message: "Avatar image is required", Param: "avatar"}
		return err
	}
	var imageFile *bytes.Reader
	var imageSize int64
	imageFile, imageSize, err = util.ValidateImage(ctxContext, fh, "avatar")
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

	var oldImageId *string
	oldImageId, err = usecase.ServerRepository.GetServerAvatarImageId(ctxContext, tx, serverId)
	if err != nil {
		return err
	}

	newImageId := uuid.New().String()
	newImage := model.ServerAvatarImage{
		Id: newImageId, Bucket: bucket,
		ObjectKey: fmt.Sprintf("server/avatar/%s.webp", newImageId),
		MimeType:  "image/webp", Size: imageSize,
		CreatedAt: now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = usecase.ServerRepository.CreateServerAvatarImage(ctxContext, tx, newImage)
	if err != nil {
		return err
	}

	err = usecase.ServerRepository.UpdateServerAvatarImage(ctxContext, tx, serverId, util.ToPtr(newImageId), userId, now)
	if err != nil {
		return err
	}

	if oldImageId != nil {
		err = usecase.ServerRepository.DeleteServerAvatarImage(ctxContext, tx, *oldImageId)
		if err != nil {
			return err
		}
	}

	err = usecase.ServerRepository.UploadObject(ctxContext, bucket,
		fmt.Sprintf("server/avatar/%s.webp", newImageId), imageFile, imageSize)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to upload new server avatar", zap.Error(err))
		return err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

func (usecase *ServerUsecase) UpdateServerBanner(ctx fiber.Ctx, userId, serverId string) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.UpdateServerBanner")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	v := util.NewValidator()
	v.UUID("serverId", serverId).Required()
	err = v.Validate()
	if err != nil {
		return err
	}

	var ownerCount int
	ownerCount, err = usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return err
	}
	if ownerCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not the owner of this server"}
		return err
	}

	fh, err := ctx.FormFile("banner")
	if err != nil {
		err = &model.BadRequestError{Code: constant.ERR_VALIDATION_CODE, Message: "Banner image is required", Param: "banner"}
		return err
	}
	var imageFile *bytes.Reader
	var imageSize int64
	imageFile, imageSize, err = util.ValidateImage(ctxContext, fh, "banner")
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

	var oldImageId *string
	oldImageId, err = usecase.ServerRepository.GetServerBannerImageId(ctxContext, tx, serverId)
	if err != nil {
		return err
	}

	newImageId := uuid.New().String()
	newImage := model.ServerBannerImage{
		Id: newImageId, Bucket: bucket,
		ObjectKey: fmt.Sprintf("server/banner/%s.webp", newImageId),
		MimeType:  "image/webp", Size: imageSize,
		CreatedAt: now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = usecase.ServerRepository.CreateServerBannerImage(ctxContext, tx, newImage)
	if err != nil {
		return err
	}

	err = usecase.ServerRepository.UpdateServerBannerImage(ctxContext, tx, serverId, util.ToPtr(newImageId), userId, now)
	if err != nil {
		return err
	}

	if oldImageId != nil {
		err = usecase.ServerRepository.DeleteServerBannerImage(ctxContext, tx, *oldImageId)
		if err != nil {
			return err
		}
	}

	err = usecase.ServerRepository.UploadObject(ctxContext, bucket,
		fmt.Sprintf("server/banner/%s.webp", newImageId), imageFile, imageSize)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to upload new server banner", zap.Error(err))
		return err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

func (usecase *ServerUsecase) DeleteServer(ctx fiber.Ctx, userId, serverId string) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.DeleteServer")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	v := util.NewValidator()
	v.UUID("serverId", serverId).Required()
	err = v.Validate()
	if err != nil {
		return err
	}

	var ownerCount int
	ownerCount, err = usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return err
	}
	if ownerCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not the owner of this server", Param: "serverId"}
		return err
	}

	err = usecase.ServerRepository.DeleteServerHard(ctxContext, serverId)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *ServerUsecase) LeaveServer(ctx fiber.Ctx, userId, serverId string) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.LeaveServer")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

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
		err = &model.NotFoundError{Code: constant.ERR_NOT_FOUND_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return err
	}

	var ownerCount int
	ownerCount, err = usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return err
	}
	if ownerCount > 0 {
		err = &model.ConflictError{Code: constant.ERR_CONFLICT_CODE, Message: "Owner cannot leave. Delete server or transfer ownership.", Param: "serverId"}
		return err
	}

	err = usecase.ServerRepository.DeleteServerMember(ctxContext, serverId, userId)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *ServerUsecase) CreateInviteLink(ctx fiber.Ctx, userId, serverId string, payload model.ServerInviteLinkRequest) (model.ServerInviteLinkResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.CreateInviteLink")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.ServerInviteLinkResponse

	v := util.NewValidator()
	v.UUID("serverId", serverId).Required()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	var memberCount int
	memberCount, err = usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "Not a member of this server", Param: "serverId"}
		return response, err
	}

	if payload.MaxUses <= 0 {
		payload.MaxUses = 10
	}
	if payload.MaxUses > 100 {
		err = &model.BadRequestError{Code: constant.ERR_VALIDATION_CODE, Message: "Max uses cannot exceed 100", Param: "maxUses"}
		return response, err
	}

	var code string
	code, err = util.GenerateInviteCode()
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to generate invite code", zap.Error(err))
		return response, err
	}

	now := time.Now().UTC()
	invite := model.ServerInvite{
		Id:        uuid.New().String(),
		ServerId:  serverId,
		Code:      code,
		MaxUses:   payload.MaxUses,
		UsedCount: 0,
		ExpiresAt: payload.ExpiresAt,
		IsActive:  true,
		CreatedAt: now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = usecase.ServerRepository.CreateServerInvites(ctxContext, invite)
	if err != nil {
		return response, err
	}

	response.Code = code
	response.InviteUrl = fmt.Sprintf("%s/api/servers/invites/%s", usecase.Config.String("APP_BASE_URL"), code)
	response.MaxUses = payload.MaxUses
	response.ExpiresAt = payload.ExpiresAt
	return response, nil
}

func (usecase *ServerUsecase) GetDiscoveryServer(ctx fiber.Ctx, userId, cursor, limitStr, categoryStr string) (model.DiscoveryServerResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.GetDiscoveryServer")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.DiscoveryServerResponse

	span.SetAttributes(attribute.String("user.id", userId))

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	var cursorObj *model.ServerDiscoveryCursor
	if cursor != "" {
		cursorObj, err = util.DecodeCursor[model.ServerDiscoveryCursor](cursor)
		if err != nil {
			err = &model.BadRequestError{Code: constant.ERR_VALIDATION_CODE, Message: "Invalid cursor", Param: "cursor"}
			return response, err
		}
	}

	var categoryId int
	if categoryStr != "" {
		categoryId, err = strconv.Atoi(categoryStr)
		if err != nil {
			err = &model.BadRequestError{Code: constant.ERR_VALIDATION_CODE, Message: "categoryId must be int", Param: "categoryId"}
			return response, err
		}
	}

	minioFullUrl := usecase.Config.String("MINIO_HTTP") + usecase.Config.String("MINIO_URL") + "/" + usecase.Config.String("MINIO_BUCKET_NAME")
	var servers []model.ServerInfoResponse
	servers, err = usecase.ServerRepository.GetServerDiscovery(ctxContext, limit+1, categoryId, cursorObj, minioFullUrl)
	if err != nil {
		return response, err
	}

	if len(servers) > limit {
		last := servers[limit-1]
		response.Page.NextCursor = util.EncodeCursor(model.ServerDiscoveryCursor{CreatedAt: last.CreatedAt, Id: last.Id})
		servers = servers[:limit]
	}
	response.Data = servers

	return response, nil
}

func (usecase *ServerUsecase) GetUserServer(ctx fiber.Ctx, userId, cursor, limitStr string) (model.ServerUserListResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.GetUserServer")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.ServerUserListResponse

	span.SetAttributes(attribute.String("user.id", userId))

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	var cursorObj *model.ServerUserCursor
	if cursor != "" {
		cursorObj, err = util.DecodeCursor[model.ServerUserCursor](cursor)
		if err != nil {
			err = &model.BadRequestError{Code: constant.ERR_VALIDATION_CODE, Message: "Invalid cursor", Param: "cursor"}
			return response, err
		}
	}

	minioFullUrl := usecase.Config.String("MINIO_HTTP") + usecase.Config.String("MINIO_URL") + "/" + usecase.Config.String("MINIO_BUCKET_NAME")
	var servers []model.ServerUserResponse
	servers, err = usecase.ServerRepository.GetUserServers(ctxContext, userId, limit+1, cursorObj, minioFullUrl)
	if err != nil {
		return response, err
	}

	if len(servers) > limit {
		last := servers[limit-1]
		response.Page.NextCursor = util.EncodeCursor(model.ServerUserCursor{JoinedAt: last.JoinedAt, ServerId: last.Id})
		servers = servers[:limit]
	}
	response.Data = servers

	return response, nil
}

func (usecase *ServerUsecase) GetCategoryServer(ctx fiber.Ctx, cursor, limitStr string) (model.ServerCategoryListResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.GetCategoryServer")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.ServerCategoryListResponse

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var cursorId int
	if cursor != "" {
		cursorId, err = strconv.Atoi(cursor)
		if err != nil {
			err = &model.BadRequestError{Code: constant.ERR_VALIDATION_CODE, Message: "Invalid cursor", Param: "cursor"}
			return response, err
		}
	}

	var categories []model.ServerCategoryResponse
	categories, err = usecase.ServerRepository.GetServerCategories(ctxContext, limit+1, cursorId)
	if err != nil {
		return response, err
	}

	if len(categories) > limit {
		last := categories[limit-1]
		response.Page.NextCursor = strconv.Itoa(last.Id)
		categories = categories[:limit]
	}
	response.Data = categories

	return response, nil
}

package server

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/ferdian3456/virdanproject/shared"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

type Service struct {
	Repo   *Repository
	DB     *pgxpool.Pool
	Log    *zap.Logger
	Config *koanf.Koanf
}

func NewService(repo *Repository, db *pgxpool.Pool, log *zap.Logger, config *koanf.Koanf) *Service {
	return &Service{
		Repo:   repo,
		DB:     db,
		Log:    log,
		Config: config,
	}
}

func (service *Service) CreateServer(ctx fiber.Ctx, userId string) (ServerCreateResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.CreateServer")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	var response ServerCreateResponse

	name := ctx.FormValue("name")
	shortName := ctx.FormValue("shortName")
	description := ctx.FormValue("description")
	categoryIdStr := ctx.FormValue("categoryId")
	isPrivateStr := ctx.FormValue("isPrivate")
	nickname := ctx.FormValue("nickname")
	username := ctx.FormValue("username")
	bio := ctx.FormValue("bio")

	v := shared.NewValidator()
	v.String("name", name).Required().MinLen(3).MaxLen(40)
	v.String("shortName", shortName).Required().MinLen(2).MaxLen(10)
	v.String("description", description).MaxLen(500)
	v.String("categoryId", categoryIdStr).Required()
	v.String("isPrivate", isPrivateStr).Required()
	v.String("nickname", nickname).Required().MinLen(3).MaxLen(50)
	v.String("username", username).Required().MinLen(3).MaxLen(22).Regex(shared.UsernameRegex, shared.UsernameErrorText)
	v.String("bio", bio).MaxLen(150)
	err = v.Validate()
	if err != nil {
		return response, err
	}

	username = strings.ToLower(strings.TrimSpace(username))

	var categoryId int
	categoryId, err = strconv.Atoi(categoryIdStr)
	if err != nil {
		err = &shared.BadRequestError{Code: shared.ERR_VALIDATION_CODE, Message: "categoryId must be int", Param: "categoryId"}
		return response, err
	}
	isPrivate := isPrivateStr == "true"

	var catCount int
	catCount, err = service.Repo.CheckServerCategories(ctxContext, categoryId)
	if err != nil {
		return response, err
	}
	if catCount == 0 {
		err = &shared.NotFoundError{Code: shared.ERR_NOT_FOUND_CODE, Message: "Category not found", Param: "categoryId"}
		return response, err
	}

	var serverAvatarFile *bytes.Reader
	var serverAvatarSize int64
	var serverAvatarImageId *string
	fh, fhErr := ctx.FormFile("serverAvatar")
	if fhErr == nil && fh != nil {
		serverAvatarFile, serverAvatarSize, _, _, err = shared.ValidateImage(ctxContext, fh, "serverAvatar", shared.MAX_IMAGE_SIZE, 512, 512, true)
		if err != nil {
			return response, err
		}
		serverAvatarImageId = shared.ToPtr(uuid.New().String())
	}

	var tx pgx.Tx
	tx, err = service.DB.Begin(ctxContext)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to begin transaction", zap.Error(err))
		return response, err
	}
	defer func() { _ = tx.Rollback(ctxContext) }()

	now := time.Now().UTC()
	serverId := uuid.New().String()

	var avatarImageIdPtr *string
	avatarImageIdPtr, err = shared.ResolveProfileAvatar(
		ctxContext, tx, ctx,
		service.Repo,
		service.Config,
		service.Log,
		userId,
		now,
	)
	if err != nil {
		return response, err
	}

	if serverAvatarImageId != nil {
		avatarImage := ServerAvatarImage{
			Id:        *serverAvatarImageId,
			Bucket:    service.Config.String("MINIO_BUCKET_NAME"),
			ObjectKey: fmt.Sprintf("server/avatar/%s.webp", *serverAvatarImageId),
			MimeType:  "image/webp",
			Size:      serverAvatarSize,
			CreatedAt: now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
		}
		err = service.Repo.CreateServerAvatarImage(ctxContext, tx, avatarImage)
		if err != nil {
			return response, err
		}
	}

	var descPtr *string
	if description != "" {
		descPtr = shared.ToPtr(description)
	}

	initialSettings := ServerSettings{IsPrivate: isPrivate}
	var settingsBytes []byte
	settingsBytes, err = sonic.Marshal(initialSettings)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to marshal server settings", zap.Error(err))
		return response, err
	}

	srv := Server{
		Id:            serverId,
		OwnerId:       userId,
		Name:          name,
		ShortName:     shortName,
		AvatarImageId: serverAvatarImageId,
		CategoryId:    shared.ToPtr(categoryId),
		Description:   descPtr,
		Settings:      settingsBytes,
		CreatedAt:     now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = service.Repo.CreateServer(ctxContext, tx, srv)
	if err != nil {
		return response, err
	}

	ownerRoleId := uuid.New().String()
	ownerRole := ServerRole{
		Id:          ownerRoleId,
		ServerId:    serverId,
		Name:        "Owner",
		Permissions: []byte(`{"all":true}`),
		CreatedAt:   now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = service.Repo.CreateServerRole(ctxContext, tx, ownerRole)
	if err != nil {
		return response, err
	}

	adminRole := ServerRole{
		Id:          uuid.New().String(),
		ServerId:    serverId,
		Name:        AdminRole,
		Permissions: []byte(`{}`),
		CreatedAt:   now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = service.Repo.CreateServerRole(ctxContext, tx, adminRole)
	if err != nil {
		return response, err
	}

	memberRole := ServerRole{
		Id:          uuid.New().String(),
		ServerId:    serverId,
		Name:        "Member",
		Permissions: []byte(`{}`),
		CreatedAt:   now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = service.Repo.CreateServerRole(ctxContext, tx, memberRole)
	if err != nil {
		return response, err
	}

	member := ServerMember{
		Id:           uuid.New().String(),
		ServerId:     serverId,
		UserId:       userId,
		ServerRoleId: ownerRoleId,
		JoinedAt:     now,
		CreatedAt:    now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = service.Repo.CreateServerMember(ctxContext, tx, member)
	if err != nil {
		return response, err
	}

	var bioPtr *string
	if bio != "" {
		bioPtr = shared.ToPtr(bio)
	}

	profileId := uuid.New().String()
	profile := ServerMemberProfile{
		Id:            profileId,
		ServerId:      serverId,
		UserId:        userId,
		Nickname:      nickname,
		Username:      username,
		Bio:           bioPtr,
		AvatarImageId: avatarImageIdPtr,
		CreatedAt:     now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = service.Repo.CreateServerMemberProfile(ctxContext, tx, profile)
	if err != nil {
		return response, err
	}

	if serverAvatarImageId != nil {
		bucket := service.Config.String("MINIO_BUCKET_NAME")
		objectKey := fmt.Sprintf("server/avatar/%s.webp", *serverAvatarImageId)
		err = service.Repo.UploadObject(ctxContext, bucket, objectKey, serverAvatarFile, serverAvatarSize)
		if err != nil {
			shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to upload server avatar", zap.Error(err))
			return response, err
		}
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to commit transaction", zap.Error(err))
		return response, err
	}

	response.Server = ServerDetailResponse{
		Id:            serverId,
		Name:          name,
		ShortName:     shortName,
		CategoryId:    shared.ToPtr(categoryId),
		Description:   descPtr,
		OwnerId:       userId,
		OwnerNickname: nickname,
		MemberCount:   1,
		IsMember:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if serverAvatarImageId != nil {
		minioFullUrl := service.Config.String("MINIO_HTTP") + service.Config.String("MINIO_URL")
		url := fmt.Sprintf("%s/%s/server/avatar/%s.webp", minioFullUrl, service.Config.String("MINIO_BUCKET_NAME"), *serverAvatarImageId)
		response.Server.AvatarUrl = &url
	}
	response.Identity = ServerMemberProfileResponse{
		ProfileId:     profileId,
		Nickname:      nickname,
		Username:      username,
		Bio:           bioPtr,
		AvatarImageId: avatarImageIdPtr,
		CreatedAt:     now, UpdatedAt: now,
	}

	return response, nil
}

func (service *Service) JoinServer(ctx fiber.Ctx, userId, serverId string) error {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.JoinServer")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
	)

	nickname := ctx.FormValue("nickname")
	username := ctx.FormValue("username")
	bio := ctx.FormValue("bio")

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	v.String("nickname", nickname).Required().MinLen(3).MaxLen(50)
	v.String("username", username).Required().MinLen(3).MaxLen(22).Regex(shared.UsernameRegex, shared.UsernameErrorText)
	v.String("bio", bio).MaxLen(150)
	err = v.Validate()
	if err != nil {
		return err
	}

	username = strings.ToLower(strings.TrimSpace(username))

	var bioPtr *string
	if bio != "" {
		bioPtr = shared.ToPtr(bio)
	}

	var serverInfo ServerCheckEligibleInfo
	serverInfo, err = service.Repo.CheckServerEligibleForJoin(ctxContext, serverId)
	if err != nil {
		return err
	}
	if !serverInfo.Exists {
		err = &shared.NotFoundError{Code: shared.ERR_NOT_FOUND_CODE, Message: "Server not found", Param: "serverId"}
		return err
	}
	if serverInfo.IsPrivate {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "Server is private. Use invite code.", Param: "serverId"}
		return err
	}

	var memberCount int
	memberCount, err = service.Repo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return err
	}
	if memberCount > 0 {
		err = &shared.ConflictError{Code: shared.ERR_CONFLICT_CODE, Message: "Already a member of this server", Param: "serverId"}
		return err
	}

	var memberRoleId string
	memberRoleId, err = service.Repo.GetRoleByName(ctxContext, serverId, "Member")
	if err != nil {
		return err
	}

	var tx pgx.Tx
	tx, err = service.DB.Begin(ctxContext)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to begin transaction", zap.Error(err))
		return err
	}
	defer func() { _ = tx.Rollback(ctxContext) }()

	now := time.Now().UTC()

	var avatarImageIdPtr *string
	avatarImageIdPtr, err = shared.ResolveProfileAvatar(
		ctxContext, tx, ctx,
		service.Repo,
		service.Config,
		service.Log,
		userId,
		now,
	)
	if err != nil {
		return err
	}

	var existingProfileId string
	var profileExists bool
	existingProfileId, profileExists, err = service.Repo.TryGetServerMemberProfileId(ctxContext, tx, serverId, userId)
	if err != nil {
		return err
	}

	if profileExists {
		err = service.Repo.UpdateServerProfileFull(ctxContext, tx, existingProfileId,
			nickname, username, bioPtr, avatarImageIdPtr, userId, now)
		if err != nil {
			return err
		}
	} else {
		profile := ServerMemberProfile{
			Id:            uuid.New().String(),
			ServerId:      serverId,
			UserId:        userId,
			Nickname:      nickname,
			Username:      username,
			Bio:           bioPtr,
			AvatarImageId: avatarImageIdPtr,
			CreatedAt:     now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
		}
		err = service.Repo.CreateServerMemberProfile(ctxContext, tx, profile)
		if err != nil {
			return err
		}
	}

	member := ServerMember{
		Id:           uuid.New().String(),
		ServerId:     serverId,
		UserId:       userId,
		ServerRoleId: memberRoleId,
		JoinedAt:     now,
		CreatedAt:    now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = service.Repo.CreateServerMember(ctxContext, tx, member)
	if err != nil {
		return err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

func (service *Service) JoinServerFromInvite(ctx fiber.Ctx, userId string, payload ServerJoinByInviteRequest) error {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.JoinServerFromInvite")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("invite.code", payload.InviteCode),
	)

	v := shared.NewValidator()
	v.String("inviteCode", payload.InviteCode).Required().MinLen(8).MaxLen(8)
	v.String("nickname", payload.Nickname).Required().MinLen(3).MaxLen(50)
	v.String("username", payload.Username).Required().MinLen(3).MaxLen(22).Regex(shared.UsernameRegex, shared.UsernameErrorText)
	v.String("bio", shared.Deref(payload.Bio, "")).MaxLen(500)
	if payload.AvatarImageId != nil && *payload.AvatarImageId != "" {
		v.UUID("avatarImageId", *payload.AvatarImageId)
	}
	err = v.Validate()
	if err != nil {
		return err
	}

	payload.Username = strings.ToLower(strings.TrimSpace(payload.Username))

	var serverId string
	serverId, err = service.Repo.ValidateAndConsumeInvite(ctxContext, payload.InviteCode)
	if err != nil {
		return err
	}

	span.SetAttributes(attribute.String("server.id", serverId))

	var memberCount int
	memberCount, err = service.Repo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return err
	}
	if memberCount > 0 {
		err = &shared.ConflictError{Code: shared.ERR_CONFLICT_CODE, Message: "Already a member of this server", Param: "inviteCode"}
		return err
	}

	if payload.AvatarImageId != nil && *payload.AvatarImageId != "" {
		var owned bool
		owned, err = service.Repo.CheckProfileAvatarImageOwnership(ctxContext, userId, *payload.AvatarImageId)
		if err != nil {
			return err
		}
		if !owned {
			err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "Avatar image is not owned by you", Param: "avatarImageId"}
			return err
		}
	}

	var memberRoleId string
	memberRoleId, err = service.Repo.GetRoleByName(ctxContext, serverId, "Member")
	if err != nil {
		return err
	}

	var tx pgx.Tx
	tx, err = service.DB.Begin(ctxContext)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to begin transaction", zap.Error(err))
		return err
	}
	defer func() { _ = tx.Rollback(ctxContext) }()

	now := time.Now().UTC()

	var existingProfileId string
	var profileExists bool
	existingProfileId, profileExists, err = service.Repo.TryGetServerMemberProfileId(ctxContext, tx, serverId, userId)
	if err != nil {
		return err
	}

	if profileExists {
		err = service.Repo.UpdateServerProfileFull(ctxContext, tx, existingProfileId,
			payload.Nickname, payload.Username, payload.Bio, payload.AvatarImageId, userId, now)
		if err != nil {
			return err
		}
	} else {
		profile := ServerMemberProfile{
			Id:       uuid.New().String(),
			ServerId: serverId, UserId: userId,
			Nickname: payload.Nickname, Username: payload.Username, Bio: payload.Bio, AvatarImageId: payload.AvatarImageId,
			CreatedAt: now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
		}
		err = service.Repo.CreateServerMemberProfile(ctxContext, tx, profile)
		if err != nil {
			return err
		}
	}

	member := ServerMember{
		Id:       uuid.New().String(),
		ServerId: serverId, UserId: userId,
		ServerRoleId: memberRoleId, JoinedAt: now,
		CreatedAt: now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = service.Repo.CreateServerMember(ctxContext, tx, member)
	if err != nil {
		return err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

func (service *Service) GetServerInfoForInvite(ctx fiber.Ctx, inviteCode string) (ServerInfoForInviteResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetServerInfoForInvite")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response ServerInfoForInviteResponse

	v := shared.NewValidator()
	v.String("inviteCode", inviteCode).Required().MinLen(8).MaxLen(8)
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("invite.code", inviteCode))

	minioFullUrl := service.Config.String("MINIO_HTTP") + service.Config.String("MINIO_URL") + "/" + service.Config.String("MINIO_BUCKET_NAME")
	response, err = service.Repo.GetServerInfoForInvite(ctxContext, inviteCode, minioFullUrl)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (service *Service) GetServerById(ctx fiber.Ctx, userId, serverId string) (ServerDetailResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetServerById")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response ServerDetailResponse

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	minioFullUrl := service.Config.String("MINIO_HTTP") + service.Config.String("MINIO_URL") + "/" + service.Config.String("MINIO_BUCKET_NAME")
	response, err = service.Repo.GetServerById(ctxContext, serverId, userId, minioFullUrl)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (service *Service) UpdateServerName(ctx fiber.Ctx, userId, serverId string, payload ServerUpdateNameRequest) (ServerUpdateResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.UpdateServerName")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response ServerUpdateResponse

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	v.String("name", payload.Name).Required().MinLen(3).MaxLen(40)
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	var ownerCount int
	ownerCount, err = service.Repo.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}
	if ownerCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not the owner of this server", Param: "serverId"}
		return response, err
	}

	now := time.Now().UTC()
	err = service.Repo.UpdateServerName(ctxContext, serverId, payload.Name, userId, now)
	if err != nil {
		return response, err
	}

	response.Id = serverId
	response.UpdatedAt = now
	return response, nil
}

func (service *Service) UpdateServerShortName(ctx fiber.Ctx, userId, serverId string, payload ServerUpdateShortNameRequest) (ServerUpdateResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.UpdateServerShortName")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response ServerUpdateResponse

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	v.String("shortName", payload.ShortName).Required().MinLen(2).MaxLen(10)
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	var ownerCount int
	ownerCount, err = service.Repo.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}
	if ownerCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not the owner of this server", Param: "serverId"}
		return response, err
	}

	now := time.Now().UTC()
	err = service.Repo.UpdateServerShortName(ctxContext, serverId, payload.ShortName, userId, now)
	if err != nil {
		return response, err
	}

	response.Id = serverId
	response.UpdatedAt = now
	return response, nil
}

func (service *Service) UpdateServerCategory(ctx fiber.Ctx, userId, serverId string, payload ServerUpdateCategoryRequest) (ServerUpdateResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.UpdateServerCategory")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response ServerUpdateResponse

	v := shared.NewValidator()
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
	ownerCount, err = service.Repo.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}
	if ownerCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not the owner of this server", Param: "serverId"}
		return response, err
	}

	var categoryExists bool
	categoryExists, err = service.Repo.CheckCategoryActive(ctxContext, payload.CategoryId)
	if err != nil {
		return response, err
	}
	if !categoryExists {
		err = &shared.NotFoundError{Code: shared.ERR_NOT_FOUND_CODE, Message: "Category not found or inactive", Param: "categoryId"}
		return response, err
	}

	now := time.Now().UTC()
	err = service.Repo.UpdateServerCategory(ctxContext, serverId, payload.CategoryId, userId, now)
	if err != nil {
		return response, err
	}

	response.Id = serverId
	response.UpdatedAt = now
	return response, nil
}

func (service *Service) UpdateServerDescription(ctx fiber.Ctx, userId, serverId string, payload ServerUpdateDescriptionRequest) (ServerUpdateResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.UpdateServerDescription")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response ServerUpdateResponse

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	v.String("description", payload.Description).MaxLen(2000)
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	var ownerCount int
	ownerCount, err = service.Repo.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}
	if ownerCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not the owner of this server", Param: "serverId"}
		return response, err
	}

	now := time.Now().UTC()
	var descPtr *string
	if payload.Description != "" {
		descPtr = shared.ToPtr(payload.Description)
	}
	err = service.Repo.UpdateServerDescription(ctxContext, serverId, descPtr, userId, now)
	if err != nil {
		return response, err
	}

	response.Id = serverId
	response.UpdatedAt = now
	return response, nil
}

func (service *Service) UpdateServerSettings(ctx fiber.Ctx, userId, serverId string, payload ServerUpdateSettingsRequest) (ServerUpdateResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.UpdateServerSettings")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response ServerUpdateResponse

	v := shared.NewValidator()
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
	ownerCount, err = service.Repo.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}
	if ownerCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not the owner of this server", Param: "serverId"}
		return response, err
	}

	settings := ServerSettings(payload)
	var settingsBytes []byte
	settingsBytes, err = sonic.Marshal(settings)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to marshal server settings", zap.Error(err))
		return response, err
	}

	now := time.Now().UTC()
	err = service.Repo.UpdateServerSettings(ctxContext, serverId, settingsBytes, userId, now)
	if err != nil {
		return response, err
	}

	response.Id = serverId
	response.UpdatedAt = now
	return response, nil
}

func (service *Service) UpdateServerAvatar(ctx fiber.Ctx, userId, serverId string) error {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.UpdateServerAvatar")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	err = v.Validate()
	if err != nil {
		return err
	}

	var ownerCount int
	ownerCount, err = service.Repo.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return err
	}
	if ownerCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not the owner of this server"}
		return err
	}

	fh, err := ctx.FormFile("avatar")
	if err != nil {
		err = &shared.BadRequestError{Code: shared.ERR_VALIDATION_CODE, Message: "Avatar image is required", Param: "avatar"}
		return err
	}
	var imageFile *bytes.Reader
	var imageSize int64
	imageFile, imageSize, _, _, err = shared.ValidateImage(ctxContext, fh, "avatar", shared.MAX_IMAGE_SIZE, 512, 512, true)
	if err != nil {
		return err
	}

	var tx pgx.Tx
	tx, err = service.DB.Begin(ctxContext)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to begin transaction", zap.Error(err))
		return err
	}
	defer func() { _ = tx.Rollback(ctxContext) }()

	now := time.Now().UTC()
	bucket := service.Config.String("MINIO_BUCKET_NAME")

	var oldImageId *string
	oldImageId, err = service.Repo.GetServerAvatarImageId(ctxContext, tx, serverId)
	if err != nil {
		return err
	}

	newImageId := uuid.New().String()
	newImage := ServerAvatarImage{
		Id: newImageId, Bucket: bucket,
		ObjectKey: fmt.Sprintf("server/avatar/%s.webp", newImageId),
		MimeType:  "image/webp", Size: imageSize,
		CreatedAt: now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = service.Repo.CreateServerAvatarImage(ctxContext, tx, newImage)
	if err != nil {
		return err
	}

	err = service.Repo.UpdateServerAvatarImage(ctxContext, tx, serverId, shared.ToPtr(newImageId), userId, now)
	if err != nil {
		return err
	}

	if oldImageId != nil {
		err = service.Repo.DeleteServerAvatarImage(ctxContext, tx, *oldImageId)
		if err != nil {
			return err
		}
	}

	err = service.Repo.UploadObject(ctxContext, bucket,
		fmt.Sprintf("server/avatar/%s.webp", newImageId), imageFile, imageSize)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to upload new server avatar", zap.Error(err))
		return err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

func (service *Service) UpdateServerBanner(ctx fiber.Ctx, userId, serverId string) error {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.UpdateServerBanner")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	err = v.Validate()
	if err != nil {
		return err
	}

	var ownerCount int
	ownerCount, err = service.Repo.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return err
	}
	if ownerCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not the owner of this server"}
		return err
	}

	fh, err := ctx.FormFile("banner")
	if err != nil {
		err = &shared.BadRequestError{Code: shared.ERR_VALIDATION_CODE, Message: "Banner image is required", Param: "banner"}
		return err
	}
	var imageFile *bytes.Reader
	var imageSize int64
	imageFile, imageSize, _, _, err = shared.ValidateImage(ctxContext, fh, "banner", shared.MAX_IMAGE_SIZE, 1920, 1080, false)
	if err != nil {
		return err
	}

	var tx pgx.Tx
	tx, err = service.DB.Begin(ctxContext)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to begin transaction", zap.Error(err))
		return err
	}
	defer func() { _ = tx.Rollback(ctxContext) }()

	now := time.Now().UTC()
	bucket := service.Config.String("MINIO_BUCKET_NAME")

	var oldImageId *string
	oldImageId, err = service.Repo.GetServerBannerImageId(ctxContext, tx, serverId)
	if err != nil {
		return err
	}

	newImageId := uuid.New().String()
	newImage := ServerBannerImage{
		Id: newImageId, Bucket: bucket,
		ObjectKey: fmt.Sprintf("server/banner/%s.webp", newImageId),
		MimeType:  "image/webp", Size: imageSize,
		CreatedAt: now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = service.Repo.CreateServerBannerImage(ctxContext, tx, newImage)
	if err != nil {
		return err
	}

	err = service.Repo.UpdateServerBannerImage(ctxContext, tx, serverId, shared.ToPtr(newImageId), userId, now)
	if err != nil {
		return err
	}

	if oldImageId != nil {
		err = service.Repo.DeleteServerBannerImage(ctxContext, tx, *oldImageId)
		if err != nil {
			return err
		}
	}

	err = service.Repo.UploadObject(ctxContext, bucket,
		fmt.Sprintf("server/banner/%s.webp", newImageId), imageFile, imageSize)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to upload new server banner", zap.Error(err))
		return err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

func (service *Service) DeleteServer(ctx fiber.Ctx, userId, serverId string) error {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.DeleteServer")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	err = v.Validate()
	if err != nil {
		return err
	}

	var ownerCount int
	ownerCount, err = service.Repo.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return err
	}
	if ownerCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not the owner of this server", Param: "serverId"}
		return err
	}

	err = service.Repo.DeleteServerHard(ctxContext, serverId)
	if err != nil {
		return err
	}

	return nil
}

func (service *Service) LeaveServer(ctx fiber.Ctx, userId, serverId string) error {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.LeaveServer")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	err = v.Validate()
	if err != nil {
		return err
	}

	var memberCount int
	memberCount, err = service.Repo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return err
	}
	if memberCount == 0 {
		err = &shared.NotFoundError{Code: shared.ERR_NOT_FOUND_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return err
	}

	var ownerCount int
	ownerCount, err = service.Repo.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return err
	}
	if ownerCount > 0 {
		var memberTotal int
		memberTotal, err = service.Repo.CountServerMembers(ctxContext, serverId)
		if err != nil {
			return err
		}
		if memberTotal > 1 {
			err = &shared.ConflictError{Code: shared.ERR_CONFLICT_CODE, Message: "Owner cannot leave while other members exist. Transfer ownership or delete the server.", Param: "serverId"}
			return err
		}
		err = service.Repo.DeleteServerHard(ctxContext, serverId)
		if err != nil {
			return err
		}
		return nil
	}

	err = service.Repo.DeleteServerMember(ctxContext, serverId, userId)
	if err != nil {
		return err
	}

	return nil
}

func (service *Service) CreateInviteLink(ctx fiber.Ctx, userId, serverId string, payload ServerInviteLinkRequest) (ServerInviteLinkResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.CreateInviteLink")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response ServerInviteLinkResponse

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	var memberCount int
	memberCount, err = service.Repo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "Not a member of this server", Param: "serverId"}
		return response, err
	}

	if payload.MaxUses <= 0 {
		payload.MaxUses = 10
	}
	if payload.MaxUses > 100 {
		err = &shared.BadRequestError{Code: shared.ERR_VALIDATION_CODE, Message: "Max uses cannot exceed 100", Param: "maxUses"}
		return response, err
	}

	var code string
	code, err = shared.GenerateInviteCode()
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to generate invite code", zap.Error(err))
		return response, err
	}

	now := time.Now().UTC()
	invite := ServerInvite{
		Id:        uuid.New().String(),
		ServerId:  serverId,
		Code:      code,
		MaxUses:   payload.MaxUses,
		UsedCount: 0,
		ExpiresAt: payload.ExpiresAt,
		IsActive:  true,
		CreatedAt: now, UpdatedAt: now, CreatedBy: userId, UpdatedBy: userId,
	}
	err = service.Repo.CreateServerInvites(ctxContext, invite)
	if err != nil {
		return response, err
	}

	response.Code = code
	response.InviteUrl = fmt.Sprintf("%s/api/servers/invites/%s", service.Config.String("APP_BASE_URL"), code)
	response.MaxUses = payload.MaxUses
	response.ExpiresAt = payload.ExpiresAt
	return response, nil
}

func (service *Service) GetDiscoveryServer(ctx fiber.Ctx, userId, cursor, limitStr, categoryStr string) (DiscoveryServerResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetDiscoveryServer")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response DiscoveryServerResponse

	span.SetAttributes(attribute.String("user.id", userId))

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	var cursorObj *ServerDiscoveryCursor
	if cursor != "" {
		cursorObj, err = shared.DecodeCursor[ServerDiscoveryCursor](cursor)
		if err != nil {
			err = &shared.BadRequestError{Code: shared.ERR_VALIDATION_CODE, Message: "Invalid cursor", Param: "cursor"}
			return response, err
		}
	}

	var categoryId *int
	if categoryStr != "" {
		parsed, parseErr := strconv.Atoi(categoryStr)
		if parseErr != nil {
			err = &shared.BadRequestError{Code: shared.ERR_VALIDATION_CODE, Message: "categoryId must be int", Param: "categoryId"}
			return response, err
		}
		categoryId = &parsed
	}

	minioFullUrl := service.Config.String("MINIO_HTTP") + service.Config.String("MINIO_URL") + "/" + service.Config.String("MINIO_BUCKET_NAME")
	var servers []ServerInfoResponse
	servers, err = service.Repo.GetServerDiscovery(ctxContext, userId, limit+1, categoryId, cursorObj, minioFullUrl)
	if err != nil {
		return response, err
	}

	if len(servers) > limit {
		last := servers[limit-1]
		response.Page.NextCursor = shared.EncodeCursor(ServerDiscoveryCursor{CreatedAt: last.CreatedAt, Id: last.Id})
		servers = servers[:limit]
	}
	response.Data = servers

	return response, nil
}

func (service *Service) GetUserServer(ctx fiber.Ctx, userId, cursor, limitStr string) (ServerUserListResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetUserServer")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response ServerUserListResponse

	span.SetAttributes(attribute.String("user.id", userId))

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	var cursorObj *ServerUserCursor
	if cursor != "" {
		cursorObj, err = shared.DecodeCursor[ServerUserCursor](cursor)
		if err != nil {
			err = &shared.BadRequestError{Code: shared.ERR_VALIDATION_CODE, Message: "Invalid cursor", Param: "cursor"}
			return response, err
		}
	}

	minioFullUrl := service.Config.String("MINIO_HTTP") + service.Config.String("MINIO_URL") + "/" + service.Config.String("MINIO_BUCKET_NAME")
	var servers []ServerUserResponse
	servers, err = service.Repo.GetUserServers(ctxContext, userId, limit+1, cursorObj, minioFullUrl)
	if err != nil {
		return response, err
	}

	if len(servers) > limit {
		last := servers[limit-1]
		response.Page.NextCursor = shared.EncodeCursor(ServerUserCursor{JoinedAt: last.JoinedAt, ServerId: last.Id})
		servers = servers[:limit]
	}
	response.Data = servers

	return response, nil
}

func (service *Service) GetCategoryServer(ctx fiber.Ctx, cursor, limitStr string) (ServerCategoryListResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetCategoryServer")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response ServerCategoryListResponse

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var cursorId int
	if cursor != "" {
		cursorId, err = strconv.Atoi(cursor)
		if err != nil {
			err = &shared.BadRequestError{Code: shared.ERR_VALIDATION_CODE, Message: "Invalid cursor", Param: "cursor"}
			return response, err
		}
	}

	var categories []ServerCategoryResponse
	categories, err = service.Repo.GetServerCategories(ctxContext, limit+1, cursorId)
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

func (service *Service) KickMember(ctx fiber.Ctx, serverId, targetUserId, callerId string) error {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.KickMember")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", callerId),
		attribute.String("server.id", serverId),
		attribute.String("target.user.id", targetUserId),
	)

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	v.UUID("userId", targetUserId).Required()
	err = v.Validate()
	if err != nil {
		return err
	}

	if callerId == targetUserId {
		err = &shared.BadRequestError{Code: shared.ERR_VALIDATION_CODE, Message: "Use leave server to remove yourself", Param: "userId"}
		return err
	}

	callerRole, err := service.Repo.GetMemberRoleName(ctxContext, serverId, callerId)
	if err != nil {
		return err
	}
	if callerRole != OwnerRole && callerRole != AdminRole {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You do not have permission to kick members", Param: "serverId"}
		return err
	}

	targetRole, err := service.Repo.GetMemberRoleName(ctxContext, serverId, targetUserId)
	if err != nil {
		return err
	}
	if targetRole == "" {
		err = &shared.NotFoundError{Code: shared.ERR_NOT_FOUND_CODE, Message: "Target user is not a member of this server", Param: "userId"}
		return err
	}
	if targetRole == OwnerRole {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "Owner cannot be kicked", Param: "userId"}
		return err
	}
	if callerRole == AdminRole && targetRole == AdminRole {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "Admin cannot kick another admin", Param: "userId"}
		return err
	}

	err = service.Repo.DeleteServerMember(ctxContext, serverId, targetUserId)
	if err != nil {
		return err
	}

	return nil
}

func (service *Service) AssignMemberRole(ctx fiber.Ctx, serverId, targetUserId, callerId string, payload AssignMemberRoleRequest) error {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.AssignMemberRole")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", callerId),
		attribute.String("server.id", serverId),
		attribute.String("target.user.id", targetUserId),
	)

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	v.UUID("userId", targetUserId).Required()
	err = v.Validate()
	if err != nil {
		return err
	}

	if payload.Role != AdminRole && payload.Role != MemberRole {
		err = &shared.BadRequestError{Code: shared.ERR_VALIDATION_CODE, Message: "Role must be Admin or Member", Param: "role"}
		return err
	}

	if callerId == targetUserId {
		err = &shared.BadRequestError{Code: shared.ERR_VALIDATION_CODE, Message: "You cannot change your own role", Param: "userId"}
		return err
	}

	ownerCount, err := service.Repo.CheckServerOwnership(ctxContext, serverId, callerId)
	if err != nil {
		return err
	}
	if ownerCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "Only the owner can assign roles", Param: "serverId"}
		return err
	}

	targetRole, err := service.Repo.GetMemberRoleName(ctxContext, serverId, targetUserId)
	if err != nil {
		return err
	}
	if targetRole == "" {
		err = &shared.NotFoundError{Code: shared.ERR_NOT_FOUND_CODE, Message: "Target user is not a member of this server", Param: "userId"}
		return err
	}

	roleId, err := service.Repo.GetRoleByName(ctxContext, serverId, payload.Role)
	if err != nil {
		return err
	}
	if roleId == "" {
		err = &shared.NotFoundError{Code: shared.ERR_NOT_FOUND_CODE, Message: "Role not found in this server", Param: "role"}
		return err
	}

	now := time.Now().UTC()
	err = service.Repo.UpdateMemberRole(ctxContext, serverId, targetUserId, roleId, callerId, now)
	if err != nil {
		return err
	}

	return nil
}

func (service *Service) TransferOwnership(ctx fiber.Ctx, serverId, callerId string, payload TransferOwnershipRequest) error {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.TransferOwnership")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", callerId),
		attribute.String("server.id", serverId),
		attribute.String("target.user.id", payload.NewOwnerId),
	)

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	v.UUID("newOwnerId", payload.NewOwnerId).Required()
	err = v.Validate()
	if err != nil {
		return err
	}

	if callerId == payload.NewOwnerId {
		err = &shared.BadRequestError{Code: shared.ERR_VALIDATION_CODE, Message: "You are already the owner", Param: "newOwnerId"}
		return err
	}

	ownerCount, err := service.Repo.CheckServerOwnership(ctxContext, serverId, callerId)
	if err != nil {
		return err
	}
	if ownerCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "Only the owner can transfer ownership", Param: "serverId"}
		return err
	}

	targetRole, err := service.Repo.GetMemberRoleName(ctxContext, serverId, payload.NewOwnerId)
	if err != nil {
		return err
	}
	if targetRole == "" {
		err = &shared.NotFoundError{Code: shared.ERR_NOT_FOUND_CODE, Message: "New owner must be a member of this server", Param: "newOwnerId"}
		return err
	}

	ownerRoleId, err := service.Repo.GetRoleByName(ctxContext, serverId, OwnerRole)
	if err != nil {
		return err
	}
	adminRoleId, err := service.Repo.GetRoleByName(ctxContext, serverId, AdminRole)
	if err != nil {
		return err
	}
	if ownerRoleId == "" || adminRoleId == "" {
		err = &shared.NotFoundError{Code: shared.ERR_NOT_FOUND_CODE, Message: "Server roles are not properly configured", Param: "serverId"}
		return err
	}

	now := time.Now().UTC()
	err = service.Repo.TransferServerOwnership(ctxContext, serverId, callerId, payload.NewOwnerId, ownerRoleId, adminRoleId, callerId, now)
	if err != nil {
		return err
	}

	return nil
}

func (service *Service) GetServerMembers(ctx fiber.Ctx, serverId, userId, cursor, limitStr string) (ServerMemberListResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetServerMembers")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response ServerMemberListResponse
	response.Data = []ServerMemberItem{}

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
	)

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > shared.MAX_LIMIT {
		limit = shared.DEFAULT_LIMIT
	}

	memberCount, err := service.Repo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return response, err
	}

	var cursorObj *ServerMemberCursor
	if cursor != "" {
		cursorObj, err = shared.DecodeCursor[ServerMemberCursor](cursor)
		if err != nil {
			err = &shared.BadRequestError{Code: shared.ERR_VALIDATION_CODE, Message: "Invalid cursor", Param: "cursor"}
			return response, err
		}
	}

	minioFullUrl := service.Config.String("MINIO_HTTP") + service.Config.String("MINIO_URL") + "/" + service.Config.String("MINIO_BUCKET_NAME")

	members, err := service.Repo.GetServerMembers(ctxContext, serverId, limit+1, cursorObj, minioFullUrl)
	if err != nil {
		return response, err
	}

	if len(members) > limit {
		last := members[limit-1]
		response.Page.NextCursor = shared.EncodeCursor(ServerMemberCursor{
			JoinedAt: last.JoinedAt,
			UserId:   last.UserId,
		})
		members = members[:limit]
	}
	response.Data = members

	return response, nil
}

func (service *Service) GetMyRoleInServer(ctx fiber.Ctx, serverId, userId string) (string, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetMyRoleInServer")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
	)

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	err = v.Validate()
	if err != nil {
		return "", err
	}

	roleName, err := service.Repo.GetMemberRoleName(ctxContext, serverId, userId)
	if err != nil {
		return "", err
	}
	if roleName == "" {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return "", err
	}

	return roleName, nil
}

func (service *Service) GetProfileHistory(ctx fiber.Ctx, userId string) (GetProfileHistoryResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetProfileHistory")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	var response GetProfileHistoryResponse

	minioFullUrl := service.Config.String("MINIO_HTTP") + service.Config.String("MINIO_URL") + "/" + service.Config.String("MINIO_BUCKET_NAME")

	var items []GetProfileHistoryResponseItem
	items, err = service.Repo.GetProfileHistory(ctxContext, userId, minioFullUrl)
	if err != nil {
		return response, err
	}

	response.Data = items
	return response, nil
}

func (service *Service) GetServerProfileMe(ctx fiber.Ctx, userId, serverId string) (ServerMemberProfileResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetServerProfileMe")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response ServerMemberProfileResponse

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("server.id", serverId),
	)

	minioFullUrl := service.Config.String("MINIO_HTTP") + service.Config.String("MINIO_URL") + "/" + service.Config.String("MINIO_BUCKET_NAME")

	response, err = service.Repo.GetServerMemberProfile(ctxContext, serverId, userId, minioFullUrl)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (service *Service) UpdateServerProfile(ctx fiber.Ctx, userId, serverId string) (ServerProfileUpdateResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.UpdateServerProfile")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response ServerProfileUpdateResponse

	nickname := ctx.FormValue("nickname")
	username := ctx.FormValue("username")
	bio := ctx.FormValue("bio")

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	v.String("nickname", nickname).Required().MinLen(3).MaxLen(50).Nickname()
	v.String("username", username).Required().MinLen(3).MaxLen(22).Regex(shared.UsernameRegex, shared.UsernameErrorText)
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
	memberCount, err = service.Repo.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{
			Code:    shared.ERR_FORBIDDEN_CODE,
			Message: "You are not a member of this server",
			Param:   "serverId",
		}
		return response, err
	}

	var bioPtr *string
	trimmedBio := strings.TrimSpace(bio)
	if trimmedBio != "" {
		bioPtr = shared.ToPtr(trimmedBio)
	}

	var tx pgx.Tx
	tx, err = service.DB.Begin(ctxContext)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to begin transaction", zap.Error(err))
		return response, err
	}
	defer func() { _ = tx.Rollback(ctxContext) }()

	now := time.Now().UTC()

	var profileId string
	profileId, err = service.Repo.GetProfileId(ctxContext, tx, serverId, userId)
	if err != nil {
		return response, err
	}

	var newAvatarImageId *string
	newAvatarImageId, err = shared.ResolveProfileAvatar(
		ctxContext, tx, ctx,
		service.Repo,
		service.Config,
		service.Log,
		userId,
		now,
	)
	if err != nil {
		return response, err
	}

	if newAvatarImageId != nil {
		err = service.Repo.UpdateServerProfileFull(ctxContext, tx, profileId,
			nickname, username, bioPtr, newAvatarImageId, userId, now)
	} else {
		err = service.Repo.UpdateServerProfileNickBioTx(ctxContext, tx, profileId,
			nickname, username, bioPtr, userId, now)
	}
	if err != nil {
		return response, err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("Failed to commit transaction", zap.Error(err))
		return response, err
	}

	response.ProfileId = profileId
	response.UpdatedAt = now
	return response, nil
}

func (service *Service) GetServerProfileByUserId(ctx fiber.Ctx, requesterUserId, serverId, targetUserId string) (ServerMemberProfileResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetServerProfileByUserId")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response ServerMemberProfileResponse

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	v.UUID("userId", targetUserId).Required()
	err = v.Validate()
	if err != nil {
		return response, err
	}

	span.SetAttributes(
		attribute.String("user.id", requesterUserId),
		attribute.String("target.user.id", targetUserId),
		attribute.String("server.id", serverId),
	)

	var memberCount int
	memberCount, err = service.Repo.CheckServerMember(ctxContext, serverId, requesterUserId)
	if err != nil {
		return response, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{
			Code:    shared.ERR_FORBIDDEN_CODE,
			Message: "You are not a member of this server",
			Param:   "serverId",
		}
		return response, err
	}

	minioFullUrl := service.Config.String("MINIO_HTTP") + service.Config.String("MINIO_URL") + "/" + service.Config.String("MINIO_BUCKET_NAME")

	response, err = service.Repo.GetServerMemberProfile(ctxContext, serverId, targetUserId, minioFullUrl)
	if err != nil {
		return response, err
	}

	return response, nil
}

package usecase

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
)

type ServerUsecase struct {
	ServerRepository *repository.ServerRepository
	DB               *pgxpool.Pool
	Log              *zap.Logger
	Config           *koanf.Koanf
}

func NewServerUsecase(serverRepository *repository.ServerRepository, db *pgxpool.Pool, zap *zap.Logger, koanf *koanf.Koanf) *ServerUsecase {
	return &ServerUsecase{
		ServerRepository: serverRepository,
		DB:               db,
		Log:              zap,
		Config:           koanf,
	}
}

func (usecase *ServerUsecase) CreateInviteLink(ctx fiber.Ctx, userId uuid.UUID, payload model.ServerInviteLinkRequest) (model.ServerInviteLinkResponse, error) {
	response := model.ServerInviteLinkResponse{}
	serverIdParams := ctx.Params("serverId")

	serverId, err := uuid.Parse(serverIdParams)
	if err != nil {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	if payload.ExpiresInMinutes <= 0 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Expires in minutes must be greater than 0",
			Param:   "expiresInMinutes",
		}
	} else if payload.ExpiresInMinutes > 10080 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Expires in minutes must be lower or equal than 10080 or one week",
			Param:   "expiresInMinute",
		}
	}

	if payload.MaxUses <= 0 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Max uses must be greater than 0",
			Param:   "maxUses",
		}
	} else if payload.MaxUses > 100 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Max uses must be lower or equal than 100",
			Param:   "maxUses",
		}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.CreateInviteLink")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("server.id", serverIdParams),
	)

	var inviteCode string

	for i := 0; i < 10; i++ {
		inviteCode, err = util.GenerateInviteCode()
		if err != nil {
			usecase.Log.Error("Failed to generate invite code", zap.Error(err))
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return response, err
		}

		exists, err := usecase.ServerRepository.CheckInviteCodes(ctxContext, inviteCode)
		if err != nil {
			return response, err
		}

		if exists == 1 {
			continue
		}
	}

	now := time.Now().UTC()
	expiresAt := time.Now().Add(time.Minute * time.Duration(payload.ExpiresInMinutes)).UTC()
	serverInvitesId := uuid.New()

	serverInvites := model.ServerInvites{
		Id:              serverInvitesId,
		ServerId:        serverId,
		Code:            inviteCode,
		MaxUses:         payload.MaxUses,
		UsedCount:       0,
		ExpiresDatetime: expiresAt,
		IsActive:        true,
		CreateDatetime:  now,
		UpdateDatetime:  now,
		CreateUserId:    userId,
		UpdateUserId:    userId,
	}

	err = usecase.ServerRepository.CreateServerInvites(ctxContext, serverInvites)
	if err != nil {
		return response, err
	}

	response.InviteCode = inviteCode
	response.ExpiresAt = expiresAt

	return response, nil
}

func (usecase *ServerUsecase) JoinServerFromInvite(ctx fiber.Ctx, userId uuid.UUID, payload model.ServerJoinRequest) error {
	if payload.InviteCode == "" {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invite code is required to not be empty",
			Param:   "inviteCode",
		}
	} else if len(payload.InviteCode) != 8 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invite code must be 8 characters",
			Param:   "inviteCode",
		}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.JoinServerFromInvite")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("invite.code", payload.InviteCode),
	)

	usecase.Log.Debug("got here?")
	serverId, err := usecase.ServerRepository.CheckInviteCodesAndRetrieveServerId(ctxContext, payload.InviteCode)
	if err != nil {
		usecase.Log.Debug("got here??")
		return err
	}

	if serverId == uuid.Nil {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invite code is not exists, expired or used up",
			Param:   "inviteCode",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "JoinServerFromInvite")
		return err
	}

	exists, err := usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		usecase.Log.Debug("got here 1")
		return err
	}

	if exists == 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Unable to join server because user is already a member",
			Param:   "serverId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "JoinServerFromInvite")
		return err
	}

	now := time.Now().UTC()

	// Check if "Member" role already exists for this server (outside transaction)
	// If exists, reuse the role ID; otherwise create a new one inside transaction
	serverRoleId, err := usecase.ServerRepository.GetRoleByName(ctxContext, serverId, model.MemberRole)
	if err != nil {
		return err
	}

	commited := false

	// start transaction
	tx, err := usecase.DB.Begin(ctxContext)
	if err != nil {
		usecase.Log.Error("Failed to begin transaction", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	defer func() {
		if !commited {
			_ = tx.Rollback(ctxContext)
		}
	}()

	// If role doesn't exist, create it
	if serverRoleId == uuid.Nil {
		serverRoleId = uuid.New()

		serverRole := model.ServerRole{
			Id:             serverRoleId,
			ServerId:       serverId,
			Name:           model.MemberRole,
			Permissions:    sonic.NoCopyRawMessage("{}"),
			CreateDatetime: now,
			UpdateDatetime: now,
			CreateUserId:   userId,
			UpdateUserId:   userId,
		}

		err = usecase.ServerRepository.CreateServerRole(ctxContext, tx, serverRole)
		if err != nil {
			return err
		}
	}

	serverMemberId := uuid.New()

	serverMember := model.ServerMember{
		Id:             serverMemberId,
		ServerId:       serverId,
		UserId:         userId,
		ServerRoleId:   serverRoleId,
		Status:         model.MemberStatusActive,
		JoinedDatetime: now,
		LeftDatetime:   nil,
		CreateDatetime: now,
		UpdateDatetime: now,
		CreateUserId:   userId,
		UpdateUserId:   userId,
	}

	err = usecase.ServerRepository.CreateServerMember(ctxContext, tx, serverMember)
	if err != nil {
		return err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		usecase.Log.Error("Failed to commit transaction", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	commited = true

	return nil
}

func (usecase *ServerUsecase) GetServerInfoForInvite(ctx fiber.Ctx, inviteCode string) (model.ServerInfoForInviteResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.GetServerInfoForInvite")
	defer span.End()

	span.SetAttributes(
		attribute.String("invite.code", inviteCode),
	)

	server, err := usecase.ServerRepository.GetServerInfoForInvite(ctxContext, inviteCode)
	if err != nil {
		return server, err
	}

	if server.ServerName == "" {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invite code is not exists",
			Param:   "inviteCode",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "GetServerInfoForInvite")
		return server, err
	}

	MINIO_URL := usecase.Config.String("MINIO_URL")
	MINIO_BUCKET_NAME := usecase.Config.String("MINIO_BUCKET_NAME")
	MINIO_HTTP := usecase.Config.String("MINIO_HTTP")

	if server.AvatarImageId != nil {
		*server.AvatarImageId = fmt.Sprintf("%s%s/%s/%s.webp", MINIO_HTTP, MINIO_URL, MINIO_BUCKET_NAME, *server.AvatarImageId)
	}

	if server.BannerImageId != nil {
		*server.BannerImageId = fmt.Sprintf("%s%s/%s/%s.webp", MINIO_HTTP, MINIO_URL, MINIO_BUCKET_NAME, *server.BannerImageId)

	}

	return server, nil
}

// func (usecase *ServerUsecase) CreateServer(ctx fiber.Ctx, userId uuid.UUID) error {
// 	ctxContext := ctx.Context()

// 	fieldName := "avatar"
// 	fileHeader, err := ctx.FormFile(fieldName)
// 	if err != nil {
// 		return err
// 	}

// 	var imageFile *bytes.Reader
// 	var imageSize int64
// 	var avatarImageId *uuid.UUID

// 	if fileHeader.Size != 0 {
// 		imageFile, imageSize, err = util.ValidateImage(fileHeader, fieldName)
// 		if err != nil {
// 			return err
// 		}

// 		id := uuid.New()
// 		avatarImageId = &id
// 	}

// 	fieldName = "banner"
// 	fileHeader, err = ctx.FormFile(fieldName)
// 	if err != nil {
// 		return err
// 	}

// 	var imageFile1 *bytes.Reader
// 	var imageSize1 int64
// 	var bannerImageId *uuid.UUID

// 	if fileHeader.Size != 0 {
// 		imageFile1, imageSize1, err = util.ValidateImage(fileHeader, fieldName)
// 		if err != nil {
// 			return err
// 		}

// 		id := uuid.New()
// 		bannerImageId = &id
// 	}

// 	name := ctx.FormValue("name")

// 	if name == "" {
// 		return &model.ValidationError{
// 			Code:    constant.ERR_VALIDATION_CODE,
// 			Message: "Name is required to not be empty",
// 			Param:   "name",
// 		}
// 	} else if len(name) < 5 {
// 		return &model.ValidationError{
// 			Code:    constant.ERR_VALIDATION_CODE,
// 			Message: "Name must be at least 4 characters",
// 			Param:   "name",
// 		}
// 	} else if len(name) > 40 {
// 		return &model.ValidationError{
// 			Code:    constant.ERR_VALIDATION_CODE,
// 			Message: "Name must be at most 40 characters",
// 			Param:   "name",
// 		}
// 	}

// 	shortName := ctx.FormValue("shortName")

// 	if shortName == "" {
// 		return &model.ValidationError{
// 			Code:    constant.ERR_VALIDATION_CODE,
// 			Message: "Short name is required to not be empty",
// 			Param:   "shortName",
// 		}
// 	} else if len(name) < 5 {
// 		return &model.ValidationError{
// 			Code:    constant.ERR_VALIDATION_CODE,
// 			Message: "Short name must be at least 5 characters",
// 			Param:   "shortName",
// 		}
// 	} else if len(name) > 10 {
// 		return &model.ValidationError{
// 			Code:    constant.ERR_VALIDATION_CODE,
// 			Message: "Short name must be at most 10 characters",
// 			Param:   "shortName",
// 		}
// 	}

// 	categoryIdString := ctx.FormValue("categoryId")
// 	var categoryIdInt int
// 	if categoryIdString != "" {
// 		categoryIdInt, err = strconv.Atoi(categoryIdString)
// 		if err != nil {
// 			return &model.ValidationError{
// 				Code:    constant.ERR_VALIDATION_CODE,
// 				Message: "Category id must be a number",
// 				Param:   "categoryId",
// 			}
// 		}

// 		exists, err := usecase.ServerRepository.CheckServerCategories(ctxContext, categoryIdInt)
// 		if err != nil {
// 			return err
// 		}

// 		if exists == 1 {
// 			return &model.ValidationError{
// 				Code:    constant.ERR_VALIDATION_CODE,
// 				Message: "Category id is not found",
// 				Param:   "categoryId",
// 			}
// 		}
// 	}

// 	description := ctx.FormValue("description")

// 	// validate settings

// 	serverId := uuid.New()
// 	now := time.Now().UTC()

// 	serverAvatarImage := model.ServerAvatarImage{}
// 	if avatarImageId != nil {
// 		serverAvatarImage.Id = *avatarImageId
// 		serverAvatarImage.ServerId = serverId
// 		serverAvatarImage.Bucket = ""
// 		serverAvatarImage.ObjectKey = ""
// 		serverAvatarImage.MimeType = ""
// 		serverAvatarImage.CreateDatetime = now
// 		serverAvatarImage.UpdateDatetime = now
// 		serverAvatarImage.CreateUserId = userId
// 		serverAvatarImage.UpdateUserId = userId
// 	}

// 	serverBannerImage := model.ServerBannerImage{}

// 	if bannerImageId != nil {
// 		serverBannerImage.Id = *bannerImageId
// 		serverBannerImage.ServerId = serverId
// 		serverBannerImage.Bucket = ""
// 		serverBannerImage.ObjectKey = ""
// 		serverBannerImage.MimeType = ""
// 		serverBannerImage.CreateDatetime = now
// 		serverBannerImage.UpdateDatetime = now
// 		serverBannerImage.CreateUserId = userId
// 		serverBannerImage.UpdateUserId = userId
// 	}

// 	server := model.Server{
// 		Id:             serverId,
// 		OwnerId:        userId,
// 		Name:           name,
// 		ShortName:      shortName,
// 		CategoryId:     &categoryIdInt,
// 		AvatarImageId:  avatarImageId,
// 		BannerImageId:  bannerImageId,
// 		Description:    &description,
// 		Settings:       sonic.NoCopyRawMessage("{}"),
// 		CreateDatetime: now,
// 		UpdateDatetime: now,
// 		CreateUserId:   userId,
// 		UpdateUserId:   userId,
// 	}

// 	serverRoleId := uuid.New()

// 	serverRole := model.ServerRole{
// 		Id:             serverRoleId,
// 		ServerId:       serverId,
// 		Name:           model.OwnerRole,
// 		Permissions:    sonic.NoCopyRawMessage("{}"), // TODO
// 		CreateDatetime: now,
// 		UpdateDatetime: now,
// 		CreateUserId:   userId,
// 		UpdateUserId:   userId,
// 	}

// 	serverMemberId := uuid.New()

// 	serverMember := model.ServerMember{
// 		Id:             serverMemberId,
// 		ServerId:       serverId,
// 		UserId:         userId,
// 		ServerRoleId:   serverRoleId,
// 		Status:         model.MemberStatusActive,
// 		JoinedAt:       now,
// 		LeftAt:         nil,
// 		CreateDatetime: now,
// 		UpdateDatetime: now,
// 		CreateUserId:   userId,
// 		UpdateUserId:   userId,
// 	}

// 	commited := false

// 	// start transaction
// 	tx, err := usecase.DB.Begin(ctxContext)
// 	if err != nil {
// 		return err
// 	}

// 	defer func() {
// 		if !commited {
// 			_ = tx.Rollback(ctxContext)
// 		}
// 	}()

// 	err = usecase.ServerRepository.CreateServer(ctxContext, tx, server)
// 	if err != nil {
// 		return err
// 	}

// 	err = usecase.ServerRepository.CreateServerRole(ctxContext, tx, serverRole)
// 	if err != nil {
// 		return err
// 	}

// 	err = usecase.ServerRepository.CreateServerMember(ctxContext, tx, serverMember)
// 	if err != nil {
// 		return err
// 	}

// 	if avatarImageId != nil {
// 		err = usecase.ServerRepository.CreateServerAvatarImage(ctxContext, tx, serverAvatarImage)
// 		if err != nil {
// 			return err
// 		}
// 		err = usecase.ServerRepository.UploadObject(ctxContext, "", "", imageFile, imageSize)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	if bannerImageId != nil {
// 		err = usecase.ServerRepository.CreateServerBannerImage(ctxContext, tx, serverBannerImage)
// 		if err != nil {
// 			return err
// 		}
// 		err = usecase.ServerRepository.UploadObject(ctxContext, "", "", imageFile1, imageSize1)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	err = tx.Commit(ctxContext)
// 	if err != nil {
// 		return err
// 	}

// 	commited = true

// 	return nil
// }

func (usecase *ServerUsecase) CreateServer(ctx fiber.Ctx, userId uuid.UUID) (model.ServerCreateResponse, error) {
	response := model.ServerCreateResponse{}
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.CreateServer")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
	)

	fieldName := "avatar"
	fileHeader, err := ctx.FormFile(fieldName)
	if err != nil {
		if err.Error() != "there is no uploaded file associated with the given key" {
			usecase.Log.Debug("check error: ", zap.Error(err))
			return response, err
		}
		// Reset error since file is optional
		err = nil
		fileHeader = nil
	}

	var imageFile *bytes.Reader
	var imageSize int64
	var avatarImageId *uuid.UUID

	if fileHeader != nil && fileHeader.Size != 0 {
		imageFile, imageSize, err = util.ValidateImage(fileHeader, fieldName)
		if err != nil {
			if validationErr, ok := err.(*model.ValidationError); ok {
				util.RecordValidationError(ctxContext, usecase.Log, span, validationErr, "CreateServer")
			}
			return response, err
		}

		id := uuid.New()
		avatarImageId = &id
	}

	name := ctx.FormValue("name")
	shortName := ctx.FormValue("shortName")
	description := ctx.FormValue("description")
	categoryId := ctx.FormValue("categoryId")
	isPrivate := ctx.FormValue("isPrivate")
	isPrivateBool, err := strconv.ParseBool(isPrivate)
	if err != nil {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Is private is invalid",
			Param:   "isPrivate",
		}
	}

	if name == "" {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Name is required to not be empty",
			Param:   "name",
		}
	} else if len(name) < 5 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Name must be at least 4 characters",
			Param:   "name",
		}
	} else if len(name) > 40 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Name must be at most 40 characters",
			Param:   "name",
		}
	}

	if shortName == "" {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Short name is required to not be empty",
			Param:   "shortName",
		}
	} else if len(shortName) < 5 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Short name must be at least 5 characters",
			Param:   "shortName",
		}
	} else if len(shortName) > 10 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Short name must be at most 10 characters",
			Param:   "shortName",
		}
	}
	categoryIdInt, err := strconv.Atoi(categoryId)
	if err != nil {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Category id is invalid",
			Param:   "categoryId",
		}
	}

	if categoryIdInt != 0 {
		exists, err := usecase.ServerRepository.CheckServerCategories(ctxContext, categoryIdInt)
		if err != nil {
			return response, err
		}

		if exists != 1 {
			err := &model.ValidationError{
				Code:    constant.ERR_VALIDATION_CODE,
				Message: "Category id is not found",
				Param:   "categoryId",
			}
			util.RecordValidationError(ctxContext, usecase.Log, span, err, "CreateServer")
			return response, err
		}
	}

	serverId := uuid.New()
	now := time.Now().UTC()

	settings := model.ServerSettingsCreateRequest{
		IsPrivate: isPrivateBool,
	}

	settingsBytes, err := json.Marshal(settings)
	if err != nil {
		usecase.Log.Error("Failed to marshal settings", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	serverAvatarImage := model.ServerAvatarImage{}
	bucketName := usecase.Config.String("MINIO_BUCKET_NAME")

	if avatarImageId != nil {
		serverAvatarImage = model.ServerAvatarImage{
			Id:             *avatarImageId,
			Bucket:         bucketName,
			ObjectKey:      fmt.Sprintf("server/avatar/%s.webp", *avatarImageId),
			MimeType:       "webp",
			Size:           imageSize,
			CreateDatetime: now,
			UpdateDatetime: now,
			CreateUserId:   userId,
			UpdateUserId:   userId,
		}
	}

	server := model.Server{
		Id:             serverId,
		OwnerId:        userId,
		Name:           name,
		ShortName:      shortName,
		CategoryId:     &categoryIdInt,
		AvatarImageId:  avatarImageId,
		BannerImageId:  nil,
		Description:    &description,
		Settings:       sonic.NoCopyRawMessage(settingsBytes),
		CreateDatetime: now,
		UpdateDatetime: now,
		CreateUserId:   userId,
		UpdateUserId:   userId,
	}

	serverRoleId := uuid.New()

	serverRole := model.ServerRole{
		Id:             serverRoleId,
		ServerId:       serverId,
		Name:           model.OwnerRole,
		Permissions:    sonic.NoCopyRawMessage(`{"*": true}`), // TODO
		CreateDatetime: now,
		UpdateDatetime: now,
		CreateUserId:   userId,
		UpdateUserId:   userId,
	}

	serverMemberId := uuid.New()

	serverMember := model.ServerMember{
		Id:             serverMemberId,
		ServerId:       serverId,
		UserId:         userId,
		ServerRoleId:   serverRoleId,
		Status:         model.MemberStatusActive,
		JoinedDatetime: now,
		LeftDatetime:   nil,
		CreateDatetime: now,
		UpdateDatetime: now,
		CreateUserId:   userId,
		UpdateUserId:   userId,
	}

	response.Id = serverId
	response.OwnerId = userId
	response.Name = name
	response.ShortName = shortName
	response.Description = &description
	response.CategoryId = &categoryIdInt
	response.Settings = settingsBytes
	response.CreateDatetime = now
	response.UpdateDatetime = now
	response.CreateUserId = userId
	response.UpdateUserId = userId

	commited := false

	// start transaction
	tx, err := usecase.DB.Begin(ctxContext)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	defer func() {
		if !commited {
			_ = tx.Rollback(ctxContext)
		}
	}()

	if avatarImageId != nil {
		err = usecase.ServerRepository.CreateServerAvatarImage(ctxContext, tx, serverAvatarImage)
		if err != nil {
			return response, err
		}

		err = usecase.ServerRepository.UploadObject(ctxContext, bucketName, serverAvatarImage.ObjectKey, imageFile, imageSize)
		if err != nil {
			return response, err
		}
	}

	err = usecase.ServerRepository.CreateServer(ctxContext, tx, server)
	if err != nil {
		return response, err
	}

	err = usecase.ServerRepository.CreateServerRole(ctxContext, tx, serverRole)
	if err != nil {
		return response, err
	}

	err = usecase.ServerRepository.CreateServerMember(ctxContext, tx, serverMember)
	if err != nil {
		return response, err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	commited = true

	return response, nil
}

func (usecase *ServerUsecase) GetDiscoveryServer(ctx fiber.Ctx, userId uuid.UUID) (model.DiscoveryServerResponse, error) {
	response := model.DiscoveryServerResponse{}

	limit := fiber.Query[int](ctx, "limit", constant.DEFAULT_LIMIT)
	categoryId := fiber.Query[int](ctx, "categoryId", 0)
	cursor := ctx.Query("cursor", "")

	if limit < 0 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Limit must be greater or equal than 0",
			Param:   "limit",
		}
	} else if limit > constant.MAX_LIMIT {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: fmt.Sprintf("Limit is exceeded max limit: %d", constant.MAX_LIMIT),
			Param:   "limit",
		}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.GetDiscoveryServer")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.Int("limit", limit),
		attribute.Int("categoryId", categoryId),
		attribute.String("cursor", cursor),
	)

	var serverDiscoveryCursor model.ServerDiscoveryCursor
	if cursor != "" {
		b, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return response, err
		}

		err = sonic.Unmarshal(b, &serverDiscoveryCursor)
		if err != nil {
			return response, err
		}
	}

	MINIO_FULL_URL := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))
	serverInfo, err := usecase.ServerRepository.GetServerDiscovery(ctxContext, limit+1, categoryId, &serverDiscoveryCursor, MINIO_FULL_URL)
	if err != nil {
		return response, err
	}

	// Initialize with empty array
	response.Data = []model.ServerInfoResponse{}

	if len(serverInfo) > limit {
		// Ada data lagi, return limit items dan buat cursor
		response.Data = serverInfo[:limit]

		last := serverInfo[limit-1]

		// Create cursor properly using ServerDiscoveryCursor
		discoveryCursor := model.ServerDiscoveryCursor{
			Id:             last.Id.String(),
			CreateDatetime: last.CreateDatetime,
		}

		b, err := sonic.Marshal(discoveryCursor)
		if err != nil {
			return response, err
		}

		response.Page.NextCursor = base64.RawURLEncoding.EncodeToString(b)
	} else {
		// Tidak ada data lagi, return semua data tanpa cursor
		if len(serverInfo) > 0 {
			response.Data = serverInfo
		}
		// Jika kosong, Data sudah []empty array dari inisialisasi
	}

	return response, nil
}

func (usecase *ServerUsecase) GetUserServer(ctx fiber.Ctx, userId uuid.UUID) (model.ServerUserListResponse, error) {
	response := model.ServerUserListResponse{}

	limit := fiber.Query[int](ctx, "limit", constant.DEFAULT_LIMIT)
	cursor := ctx.Query("cursor", "")

	if limit < 0 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Limit must be greater or equal than 0",
			Param:   "limit",
		}
	} else if limit > constant.MAX_LIMIT {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: fmt.Sprintf("Limit is exceeded max limit: %d", constant.MAX_LIMIT),
			Param:   "limit",
		}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.GetUserServer")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.Int("limit", limit),
		attribute.String("cursor", cursor),
	)

	var serverUserCursor model.ServerUserCursor
	if cursor != "" {
		b, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return response, err
		}

		err = sonic.Unmarshal(b, &serverUserCursor)
		if err != nil {
			return response, err
		}
	}

	MINIO_FULL_URL := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))

	// Fetch limit + 1 untuk cek apakah ada data lagi
	serverUser, err := usecase.ServerRepository.GetUserServer(ctxContext, limit+1, &serverUserCursor, userId, MINIO_FULL_URL)
	if err != nil {
		return response, err
	}

	// Initialize with empty array
	response.Data = []model.ServerUserResponse{}

	if len(serverUser) > limit {
		// Ada data lagi, return limit items dan buat cursor
		response.Data = serverUser[:limit]

		last := serverUser[limit-1]

		// Create cursor properly using ServerUserCursor
		lastCursor := model.ServerUserCursor{
			ServerId:       last.Id.String(),
			JoinedDatetime: last.JoinedDatetime,
		}

		b, err := sonic.Marshal(lastCursor)
		if err != nil {
			return response, err
		}

		response.Page.NextCursor = base64.RawURLEncoding.EncodeToString(b)
	} else {
		// Tidak ada data lagi, return semua data tanpa cursor
		if len(serverUser) > 0 {
			response.Data = serverUser
		}
		// Jika kosong, Data sudah []empty array dari inisialisasi
	}

	return response, nil
}

func (usecase *ServerUsecase) JoinServer(ctx fiber.Ctx, userId uuid.UUID) error {
	serverIdParams := ctx.Params("serverId")

	serverId, err := uuid.Parse(serverIdParams)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.JoinServer")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("server.id", serverIdParams),
	)

	exists, err := usecase.ServerRepository.CheckServerEligible(ctxContext, serverId)
	if err != nil {
		return err
	}

	if exists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Unable to join server because server is not exists or private",
			Param:   "serverId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "JoinServer")
		return err
	}

	exists, err = usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return err
	}

	if exists == 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Unable to join server because user is already a member",
			Param:   "serverId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "JoinServer")
		return err
	}

	now := time.Now().UTC()

	// Check if "Member" role already exists for this server (outside transaction)
	// If exists, reuse the role ID; otherwise create a new one inside transaction
	serverRoleId, err := usecase.ServerRepository.GetRoleByName(ctxContext, serverId, model.MemberRole)
	if err != nil {
		return err
	}

	commited := false

	// start transaction
	tx, err := usecase.DB.Begin(ctxContext)
	if err != nil {
		usecase.Log.Error("Failed to begin transaction", zap.Error(err))
		return err
	}

	defer func() {
		if !commited {
			_ = tx.Rollback(ctxContext)
		}
	}()

	// If role doesn't exist, create it
	if serverRoleId == uuid.Nil {
		serverRoleId = uuid.New()

		serverRole := model.ServerRole{
			Id:             serverRoleId,
			ServerId:       serverId,
			Name:           model.MemberRole,
			Permissions:    sonic.NoCopyRawMessage("{}"),
			CreateDatetime: now,
			UpdateDatetime: now,
			CreateUserId:   userId,
			UpdateUserId:   userId,
		}

		err = usecase.ServerRepository.CreateServerRole(ctxContext, tx, serverRole)
		if err != nil {
			return err
		}
	}

	serverMemberId := uuid.New()

	serverMember := model.ServerMember{
		Id:             serverMemberId,
		ServerId:       serverId,
		UserId:         userId,
		ServerRoleId:   serverRoleId,
		Status:         model.MemberStatusActive,
		JoinedDatetime: now,
		LeftDatetime:   nil,
		CreateDatetime: now,
		UpdateDatetime: now,
		CreateUserId:   userId,
		UpdateUserId:   userId,
	}

	err = usecase.ServerRepository.CreateServerMember(ctxContext, tx, serverMember)
	if err != nil {
		return err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		usecase.Log.Error("Failed to commit transaction", zap.Error(err))
		return err
	}

	commited = true

	return nil
}

// func (usecase *ServerUsecase) GetServerById(ctx fiber.Ctx) (model.ServerResponse, error) {
// 	serverIdParams := ctx.Params("serverId")

// 	response := model.ServerResponse{}

// 	serverId, err := uuid.Parse(serverIdParams)
// 	if err != nil {
// 		return response, &model.ValidationError{
// 			Code:    constant.ERR_VALIDATION_CODE,
// 			Message: "Invalid server id",
// 			Param:   "serverId",
// 		}
// 	}

// 	exists, err := usecase.ServerRepository.CheckServerEligible(ctx.Context(), serverId)
// 	if err != nil {
// 		return err
// 	}

// 	if exists == 1 {
// 		return response, &model.ValidationError{
// 			Code:    constant.ERR_VALIDATION_CODE,
// 			Message: "Unable to view server detail because server is not exists or private",
// 			Param:   "serverId",
// 		}
// 	}

// }

func (usecase *ServerUsecase) UpdateServerName(ctx fiber.Ctx, userId uuid.UUID, serverIdParam string, payload model.ServerUpdateNameRequest) (model.ServerUpdateResponse, error) {
	response := model.ServerUpdateResponse{}

	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	if payload.Name == "" {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Name is required to not be empty",
			Param:   "name",
		}
	} else if len(payload.Name) < 5 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Name must be at least 4 characters",
			Param:   "name",
		}
	} else if len(payload.Name) > 40 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Name must be at most 40 characters",
			Param:   "name",
		}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.UpdateServerName")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("server.id", serverIdParam),
		attribute.String("server.name", payload.Name),
	)

	exists, err := usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}

	if exists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the owner of this server",
			Param:   "serverId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "UpdateServerName")
		return response, err
	}

	now := time.Now().UTC()

	err = usecase.ServerRepository.UpdateServerName(ctxContext, serverId, payload.Name, userId, now)
	if err != nil {
		return response, err
	}

	response, err = usecase.ServerRepository.GetServerDetail(ctxContext, serverId)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (usecase *ServerUsecase) UpdateServerShortName(ctx fiber.Ctx, userId uuid.UUID, serverIdParam string, payload model.ServerUpdateShortNameRequest) (model.ServerUpdateResponse, error) {
	response := model.ServerUpdateResponse{}

	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	if payload.ShortName == "" {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Short name is required to not be empty",
			Param:   "shortName",
		}
	} else if len(payload.ShortName) < 5 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Short name must be at least 5 characters",
			Param:   "shortName",
		}
	} else if len(payload.ShortName) > 10 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Short name must be at most 10 characters",
			Param:   "shortName",
		}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.UpdateServerShortName")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("server.id", serverIdParam),
		attribute.String("server.shortName", payload.ShortName),
	)

	exists, err := usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}

	if exists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the owner of this server",
			Param:   "serverId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "UpdateServerShortName")
		return response, err
	}

	now := time.Now().UTC()

	err = usecase.ServerRepository.UpdateServerShortName(ctxContext, serverId, payload.ShortName, userId, now)
	if err != nil {
		return response, err
	}

	response, err = usecase.ServerRepository.GetServerDetail(ctxContext, serverId)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (usecase *ServerUsecase) UpdateServerCategory(ctx fiber.Ctx, userId uuid.UUID, serverIdParam string, payload model.ServerUpdateCategoryRequest) (model.ServerUpdateResponse, error) {
	response := model.ServerUpdateResponse{}

	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.UpdateServerCategory")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("server.id", serverIdParam),
	)

	if payload.CategoryId != nil {
		exists, err := usecase.ServerRepository.CheckServerCategories(ctxContext, *payload.CategoryId)
		if err != nil {
			return response, err
		}

		if exists != 1 {
			err := &model.ValidationError{
				Code:    constant.ERR_VALIDATION_CODE,
				Message: "Category id is not found",
				Param:   "categoryId",
			}
			util.RecordValidationError(ctxContext, usecase.Log, span, err, "UpdateServerCategory")
			return response, err
		}
	}

	exists, err := usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}

	if exists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the owner of this server",
			Param:   "serverId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "UpdateServerCategory")
		return response, err
	}

	now := time.Now().UTC()

	err = usecase.ServerRepository.UpdateServerCategory(ctxContext, serverId, payload.CategoryId, userId, now)
	if err != nil {
		return response, err
	}

	response, err = usecase.ServerRepository.GetServerDetail(ctxContext, serverId)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (usecase *ServerUsecase) UpdateServerDescription(ctx fiber.Ctx, userId uuid.UUID, serverIdParam string, payload model.ServerUpdateDescriptionRequest) (model.ServerUpdateResponse, error) {
	response := model.ServerUpdateResponse{}

	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.UpdateServerDescription")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("server.id", serverIdParam),
	)

	exists, err := usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}

	if exists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the owner of this server",
			Param:   "serverId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "UpdateServerDescription")
		return response, err
	}

	now := time.Now().UTC()

	err = usecase.ServerRepository.UpdateServerDescription(ctxContext, serverId, payload.Description, userId, now)
	if err != nil {
		return response, err
	}

	response, err = usecase.ServerRepository.GetServerDetail(ctxContext, serverId)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (usecase *ServerUsecase) DeleteServer(ctx fiber.Ctx, userId uuid.UUID, serverIdParam string) error {
	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.DeleteServer")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("server.id", serverIdParam),
	)

	exists, err := usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		usecase.Log.Error("Failed to check server ownership", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if exists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the owner of this server",
			Param:   "serverId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "DeleteServer")
		return err
	}

	err = usecase.ServerRepository.DeleteServer(ctxContext, serverId)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *ServerUsecase) UpdateServerAvatar(ctx fiber.Ctx, userId uuid.UUID, serverIdParam string) error {
	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.UpdateServerAvatar")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("server.id", serverIdParam),
	)

	exists, err := usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		usecase.Log.Error("Failed to check server ownership", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if exists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the owner of this server",
			Param:   "serverId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "UpdateServerAvatar")
		return err
	}

	fieldName := "avatar"
	fileHeader, err := ctx.FormFile(fieldName)
	if err != nil {
		return err
	}

	var imageFile *bytes.Reader
	var imageSize int64
	var avatarImageId *uuid.UUID

	if fileHeader.Size != 0 {
		imageFile, imageSize, err = util.ValidateImage(fileHeader, fieldName)
		if err != nil {
			return err
		}

		id := uuid.New()
		avatarImageId = &id
	} else {
		avatarImageId = nil
	}

	serverAvatarImage := model.ServerAvatarImage{}
	now := time.Now().UTC()

	bucketName := usecase.Config.String("MINIO_BUCKET_NAME")

	if avatarImageId != nil {
		serverAvatarImage.Id = *avatarImageId
		serverAvatarImage.Bucket = bucketName
		serverAvatarImage.ObjectKey = fmt.Sprintf("server/avatar/%s.webp", *avatarImageId)
		serverAvatarImage.MimeType = "webp"
		serverAvatarImage.Size = imageSize
		serverAvatarImage.CreateDatetime = now
		serverAvatarImage.UpdateDatetime = now
		serverAvatarImage.CreateUserId = userId
		serverAvatarImage.UpdateUserId = userId
	}

	commited := false

	// start transaction
	tx, err := usecase.DB.Begin(ctxContext)
	if err != nil {
		usecase.Log.Error("Failed to begin transaction", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	defer func() {
		if !commited {
			_ = tx.Rollback(ctxContext)
		}
	}()

	// Get old avatar image ID to delete later
	serverDetail, err := usecase.ServerRepository.GetServerDetail(ctxContext, serverId)
	if err != nil {
		return err
	}

	var oldAvatarImageId *uuid.UUID
	if serverDetail.AvatarImageId != nil {
		oldAvatarImageId = serverDetail.AvatarImageId
	}

	if avatarImageId != nil {
		// Create new avatar image FIRST before updating server
		err = usecase.ServerRepository.CreateServerAvatarImage(ctxContext, tx, serverAvatarImage)
		if err != nil {
			return err
		}

		// Now update server to reference the new image
		err = usecase.ServerRepository.UpdateServerAvatarImage(ctxContext, tx, serverId, avatarImageId, userId, now)
		if err != nil {
			return err
		}

		err = usecase.ServerRepository.UploadObject(ctxContext, bucketName, serverAvatarImage.ObjectKey, imageFile, imageSize)
		if err != nil {
			return err
		}

		// Delete old avatar image if exists
		if oldAvatarImageId != nil {
			fileName, err := usecase.ServerRepository.GetServerAvatar(ctxContext, tx, *oldAvatarImageId)
			if err != nil {
				return err
			}
			err = usecase.ServerRepository.DeleteServerAvatarImage(ctxContext, tx, *oldAvatarImageId)
			if err != nil {
				return err
			}
			if fileName != "" {
				err = usecase.ServerRepository.RemoveServerAvatarObject(ctxContext, bucketName, fileName)
				if err != nil {
					return err
				}
			}
		}
	} else {
		// Update server to remove avatar image reference
		err = usecase.ServerRepository.UpdateServerAvatarImage(ctxContext, tx, serverId, avatarImageId, userId, now)
		if err != nil {
			return err
		}

		// Delete old avatar image if exists
		if oldAvatarImageId != nil {
			fileName, err := usecase.ServerRepository.GetServerAvatar(ctxContext, tx, *oldAvatarImageId)
			if err != nil {
				return err
			}
			err = usecase.ServerRepository.DeleteServerAvatarImage(ctxContext, tx, *oldAvatarImageId)
			if err != nil {
				return err
			}
			if fileName != "" {
				err = usecase.ServerRepository.RemoveServerAvatarObject(ctxContext, bucketName, fileName)
				if err != nil {
					return err
				}
			}
		}
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		usecase.Log.Error("Failed to commit transaction", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	commited = true

	return nil
}

func (usecase *ServerUsecase) UpdateServerBanner(ctx fiber.Ctx, userId uuid.UUID, serverIdParam string) error {
	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.UpdateServerBanner")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("server.id", serverIdParam),
	)

	exists, err := usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		usecase.Log.Error("Failed to check server ownership", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if exists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the owner of this server",
			Param:   "serverId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "UpdateServerBanner")
		return err
	}

	fieldName := "banner"
	fileHeader, err := ctx.FormFile(fieldName)
	if err != nil {
		return err
	}

	var imageFile *bytes.Reader
	var imageSize int64
	var bannerImageId *uuid.UUID

	if fileHeader.Size != 0 {
		imageFile, imageSize, err = util.ValidateImage(fileHeader, fieldName)
		if err != nil {
			return err
		}

		id := uuid.New()
		bannerImageId = &id
	} else {
		bannerImageId = nil
	}

	serverBannerImage := model.ServerBannerImage{}
	now := time.Now().UTC()

	bucketName := usecase.Config.String("MINIO_BUCKET_NAME")

	if bannerImageId != nil {
		serverBannerImage.Id = *bannerImageId
		serverBannerImage.Bucket = bucketName
		serverBannerImage.ObjectKey = fmt.Sprintf("server/banner/%s.webp", *bannerImageId)
		serverBannerImage.MimeType = "webp"
		serverBannerImage.Size = imageSize
		serverBannerImage.CreateDatetime = now
		serverBannerImage.UpdateDatetime = now
		serverBannerImage.CreateUserId = userId
		serverBannerImage.UpdateUserId = userId
	}

	commited := false

	// start transaction
	tx, err := usecase.DB.Begin(ctxContext)
	if err != nil {
		usecase.Log.Error("Failed to begin transaction", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	defer func() {
		if !commited {
			_ = tx.Rollback(ctxContext)
		}
	}()

	// Get old banner image ID to delete later
	serverDetail, err := usecase.ServerRepository.GetServerDetail(ctxContext, serverId)
	if err != nil {
		return err
	}

	var oldBannerImageId *uuid.UUID
	if serverDetail.BannerImageId != nil {
		oldBannerImageId = serverDetail.BannerImageId
	}

	if bannerImageId != nil {
		// Create new banner image FIRST before updating server
		err = usecase.ServerRepository.CreateServerBannerImage(ctxContext, tx, serverBannerImage)
		if err != nil {
			return err
		}

		// Now update server to reference the new image
		err = usecase.ServerRepository.UpdateServerBannerImage(ctxContext, tx, serverId, bannerImageId, userId, now)
		if err != nil {
			return err
		}

		err = usecase.ServerRepository.UploadObject(ctxContext, bucketName, serverBannerImage.ObjectKey, imageFile, imageSize)
		if err != nil {
			return err
		}

		// Delete old banner image if exists
		if oldBannerImageId != nil {
			fileName, err := usecase.ServerRepository.GetServerBanner(ctxContext, tx, *oldBannerImageId)
			if err != nil {
				return err
			}
			err = usecase.ServerRepository.DeleteServerBannerImage(ctxContext, tx, *oldBannerImageId)
			if err != nil {
				return err
			}
			if fileName != "" {
				err = usecase.ServerRepository.RemoveServerBannerObject(ctxContext, bucketName, fileName)
				if err != nil {
					return err
				}
			}
		}
	} else {
		// Update server to remove banner image reference
		err = usecase.ServerRepository.UpdateServerBannerImage(ctxContext, tx, serverId, bannerImageId, userId, now)
		if err != nil {
			return err
		}

		// Delete old banner image if exists
		if oldBannerImageId != nil {
			fileName, err := usecase.ServerRepository.GetServerBanner(ctxContext, tx, *oldBannerImageId)
			if err != nil {
				return err
			}
			err = usecase.ServerRepository.DeleteServerBannerImage(ctxContext, tx, *oldBannerImageId)
			if err != nil {
				return err
			}
			if fileName != "" {
				err = usecase.ServerRepository.RemoveServerBannerObject(ctxContext, bucketName, fileName)
				if err != nil {
					return err
				}
			}
		}
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		usecase.Log.Error("Failed to commit transaction", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	commited = true

	return nil
}

func (usecase *ServerUsecase) UpdateServerSettings(ctx fiber.Ctx, userId uuid.UUID, serverIdParam string, payload model.ServerSettingsCreateRequest) error {
	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.UpdateServerSettings")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("server.id", serverIdParam),
	)

	exists, err := usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		usecase.Log.Error("Failed to check server ownership", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if exists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the owner of this server",
			Param:   "serverId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "UpdateServerSettings")
		return err
	}

	settingsBytes, err := json.Marshal(payload)
	if err != nil {
		usecase.Log.Error("Failed to marshal settings", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	now := time.Now().UTC()

	err = usecase.ServerRepository.UpdateServerSettings(ctxContext, serverId, settingsBytes, userId, now)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *ServerUsecase) GetServerById(ctx fiber.Ctx) (model.ServerDetailResponse, error) {
	response := model.ServerDetailResponse{}

	serverIdParam := ctx.Params("id")

	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "id",
		}
	}

	// Get userId from context (requires authentication)
	userId := ctx.Locals("userId").(uuid.UUID)

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.GetServerById")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("server.id", serverIdParam),
	)

	MINIO_FULL_URL := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))

	// Single query that checks both server existence AND membership (for private servers)
	serverDetail, err := usecase.ServerRepository.GetServerById(ctxContext, serverId, userId, MINIO_FULL_URL)
	if err != nil {
		return response, err
	}

	if serverDetail.Id == uuid.Nil {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Server not found or you don't have permission to access it",
			Param:   "id",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "GetServerById")
		return response, err
	}

	// Clear isPrivate flag from response (it's internal only)
	serverDetail.IsPrivate = nil

	return serverDetail, nil
}

func (usecase *ServerUsecase) GetCategoryServer(ctx fiber.Ctx) (model.ServerCategoryListResponse, error) {
	response := model.ServerCategoryListResponse{}

	limit := fiber.Query[int](ctx, "limit", constant.DEFAULT_LIMIT)
	cursor := ctx.Query("cursor", "")

	if limit < 0 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Limit must be greater or equal than 0",
			Param:   "limit",
		}
	} else if limit > constant.MAX_LIMIT {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: fmt.Sprintf("Limit is exceeded max limit: %d", constant.MAX_LIMIT),
			Param:   "limit",
		}
	}

	var serverCategoryCursor model.ServerCategoryCursor
	if cursor != "" {
		b, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return response, err
		}

		err = sonic.Unmarshal(b, &serverCategoryCursor)
		if err != nil {
			return response, err
		}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.GetCategoryServer")
	defer span.End()

	span.SetAttributes(
		attribute.Int("limit", limit),
		attribute.String("cursor", cursor),
	)

	// Fetch limit + 1 untuk cek apakah ada data lagi
	categories, err := usecase.ServerRepository.GetServerCategories(ctxContext, limit+1, &serverCategoryCursor)
	if err != nil {
		return response, err
	}

	// Initialize with empty array
	response.Data = []model.ServerCategoryResponse{}

	if len(categories) > limit {
		// Ada data lagi, return limit items dan buat cursor
		response.Data = categories[:limit]

		last := categories[limit-1]

		// Create cursor properly using ServerCategoryCursor
		categoryCursor := model.ServerCategoryCursor{
			Id: last.Id,
		}

		b, err := sonic.Marshal(categoryCursor)
		if err != nil {
			return response, err
		}

		response.Page.NextCursor = base64.RawURLEncoding.EncodeToString(b)
	} else {
		// Tidak ada data lagi, return semua data tanpa cursor
		if len(categories) > 0 {
			response.Data = categories
		}
		// Jika kosong, Data sudah []empty array dari inisialisasi
	}

	return response, nil
}

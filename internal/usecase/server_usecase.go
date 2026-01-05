package usecase

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/repository"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
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

func (usecase *ServerUsecase) CreateInviteLink(ctx *fiber.Ctx, userId uuid.UUID, payload model.ServerInviteLinkRequest) (model.ServerInviteLinkResponse, error) {
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
	var inviteCode string

	for i := 0; i < 10; i++ {
		inviteCode, err = util.GenerateInviteCode()
		if err != nil {
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

func (usecase *ServerUsecase) JoinServerFromInvite(ctx *fiber.Ctx, userId uuid.UUID, payload model.ServerJoinRequest) error {
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
	usecase.Log.Debug("got here?")
	serverId, err := usecase.ServerRepository.CheckInviteCodesAndRetrieveServerId(ctxContext, payload.InviteCode)
	if err != nil {
		usecase.Log.Debug("got here??")

		return err
	}

	if serverId == uuid.Nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invite code is not exists, expired or used up",
			Param:   "inviteCode",
		}
	}

	exists, err := usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		usecase.Log.Debug("got here 1")
		return err
	}

	if exists == 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Unable to join server because user is already a member",
			Param:   "serverId",
		}
	}

	now := time.Now().UTC()

	serverRoleId := uuid.New()

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

	serverMemberId := uuid.New()

	serverMember := model.ServerMember{
		Id:             serverMemberId,
		ServerId:       serverId,
		UserId:         userId,
		ServerRoleId:   uuid.Nil,
		Status:         model.MemberStatusActive,
		JoinedDatetime: now,
		LeftDatetime:   nil,
		CreateDatetime: now,
		UpdateDatetime: now,
		CreateUserId:   userId,
		UpdateUserId:   userId,
	}

	serverMember.ServerRoleId = serverRoleId

	commited := false

	// start transaction
	tx, err := usecase.DB.Begin(ctxContext)
	if err != nil {
		return err
	}

	defer func() {
		if !commited {
			_ = tx.Rollback(ctxContext)
		}
	}()

	err = usecase.ServerRepository.CreateServerRole(ctxContext, tx, serverRole)
	if err != nil {
		return err
	}

	err = usecase.ServerRepository.CreateServerMember(ctxContext, tx, serverMember)
	if err != nil {
		return err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		return err
	}

	commited = true

	return nil
}

func (usecase *ServerUsecase) GetServerInfoForInvite(ctx *fiber.Ctx, inviteCode string) (model.ServerInfoForInviteResponse, error) {
	server, err := usecase.ServerRepository.GetServerInfoForInvite(ctx.Context(), inviteCode)
	if err != nil {
		return server, err
	}

	if server.ServerName == "" {
		return server, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invite code is not exists",
			Param:   "inviteCode",
		}
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

// func (usecase *ServerUsecase) CreateServer(ctx *fiber.Ctx, userId uuid.UUID) error {
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

func (usecase *ServerUsecase) CreateServer(ctx *fiber.Ctx, userId uuid.UUID, payload model.ServerCreateRequest) error {
	if payload.Name == "" {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Name is required to not be empty",
			Param:   "name",
		}
	} else if len(payload.Name) < 5 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Name must be at least 4 characters",
			Param:   "name",
		}
	} else if len(payload.Name) > 40 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Name must be at most 40 characters",
			Param:   "name",
		}
	}

	if payload.ShortName == "" {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Short name is required to not be empty",
			Param:   "shortName",
		}
	} else if len(payload.ShortName) < 5 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Short name must be at least 5 characters",
			Param:   "shortName",
		}
	} else if len(payload.ShortName) > 10 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Short name must be at most 10 characters",
			Param:   "shortName",
		}
	}

	ctxContext := ctx.Context()

	if payload.CategoryId != nil {
		exists, err := usecase.ServerRepository.CheckServerCategories(ctxContext, *payload.CategoryId)
		if err != nil {
			return err
		}

		if exists != 1 {
			return &model.ValidationError{
				Code:    constant.ERR_VALIDATION_CODE,
				Message: "Category id is not found",
				Param:   "categoryId",
			}
		}
	}

	serverId := uuid.New()
	now := time.Now().UTC()

	settings := model.ServerSettingsCreateRequest{
		IsPrivate: payload.Settings.IsPrivate,
	}

	settingsBytes, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	server := model.Server{
		Id:             serverId,
		OwnerId:        userId,
		Name:           payload.Name,
		ShortName:      payload.ShortName,
		CategoryId:     payload.CategoryId,
		AvatarImageId:  nil,
		BannerImageId:  nil,
		Description:    payload.Description,
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

	commited := false

	// start transaction
	tx, err := usecase.DB.Begin(ctxContext)
	if err != nil {
		return err
	}

	defer func() {
		if !commited {
			_ = tx.Rollback(ctxContext)
		}
	}()

	err = usecase.ServerRepository.CreateServer(ctxContext, tx, server)
	if err != nil {
		return err
	}

	err = usecase.ServerRepository.CreateServerRole(ctxContext, tx, serverRole)
	if err != nil {
		return err
	}

	err = usecase.ServerRepository.CreateServerMember(ctxContext, tx, serverMember)
	if err != nil {
		return err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		return err
	}

	commited = true

	return nil
}

func (usecase *ServerUsecase) GetDiscoveryServer(ctx *fiber.Ctx, userId uuid.UUID) (model.DiscoveryServerResponse, error) {
	response := model.DiscoveryServerResponse{}

	limit := ctx.QueryInt("limit", constant.DEFAULT_LIMIT)
	categoryId := ctx.QueryInt("categoryId", 0)
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
	serverInfo, err := usecase.ServerRepository.GetServerDiscovery(ctx.Context(), limit+1, categoryId, &serverDiscoveryCursor, MINIO_FULL_URL)
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

func (usecase *ServerUsecase) GetUserServer(ctx *fiber.Ctx, userId uuid.UUID) (model.ServerUserListResponse, error) {
	response := model.ServerUserListResponse{}

	limit := ctx.QueryInt("limit", constant.DEFAULT_LIMIT)
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
	serverUser, err := usecase.ServerRepository.GetUserServer(ctx.Context(), limit+1, &serverUserCursor, userId, MINIO_FULL_URL)
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

func (usecase *ServerUsecase) JoinServer(ctx *fiber.Ctx, userId uuid.UUID) error {
	serverIdParams := ctx.Params("serverId")

	serverId, err := uuid.Parse(serverIdParams)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	exists, err := usecase.ServerRepository.CheckServerEligible(ctx.Context(), serverId)
	if err != nil {
		return err
	}

	if exists != 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Unable to join server because server is not exists or private",
			Param:   "serverId",
		}
	}

	exists, err = usecase.ServerRepository.CheckServerMember(ctx.Context(), serverId, userId)
	if err != nil {
		return err
	}

	if exists == 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Unable to join server because user is already a member",
			Param:   "serverId",
		}
	}

	now := time.Now().UTC()
	serverRoleId := uuid.New()

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

	ctxContext := ctx.Context()

	commited := false

	// start transaction
	tx, err := usecase.DB.Begin(ctxContext)
	if err != nil {
		return err
	}

	defer func() {
		if !commited {
			_ = tx.Rollback(ctxContext)
		}
	}()

	err = usecase.ServerRepository.CreateServerRole(ctxContext, tx, serverRole)
	if err != nil {
		return err
	}

	err = usecase.ServerRepository.CreateServerMember(ctxContext, tx, serverMember)
	if err != nil {
		return err
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		return err
	}

	commited = true

	return nil
}

// func (usecase *ServerUsecase) GetServerById(ctx *fiber.Ctx) (model.ServerResponse, error) {
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

func (usecase *ServerUsecase) UpdateServerName(ctx *fiber.Ctx, userId uuid.UUID, serverIdParam string, payload model.ServerUpdateNameRequest) error {
	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	if payload.Name == "" {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Name is required to not be empty",
			Param:   "name",
		}
	} else if len(payload.Name) < 5 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Name must be at least 4 characters",
			Param:   "name",
		}
	} else if len(payload.Name) > 40 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Name must be at most 40 characters",
			Param:   "name",
		}
	}

	ctxContext := ctx.Context()

	exists, err := usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return err
	}

	if exists != 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the owner of this server",
			Param:   "serverId",
		}
	}

	now := time.Now().UTC()

	err = usecase.ServerRepository.UpdateServerName(ctxContext, serverId, payload.Name, userId, now)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *ServerUsecase) UpdateServerShortName(ctx *fiber.Ctx, userId uuid.UUID, serverIdParam string, payload model.ServerUpdateShortNameRequest) error {
	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	if payload.ShortName == "" {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Short name is required to not be empty",
			Param:   "shortName",
		}
	} else if len(payload.ShortName) < 5 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Short name must be at least 5 characters",
			Param:   "shortName",
		}
	} else if len(payload.ShortName) > 10 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Short name must be at most 10 characters",
			Param:   "shortName",
		}
	}

	ctxContext := ctx.Context()

	exists, err := usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return err
	}

	if exists != 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the owner of this server",
			Param:   "serverId",
		}
	}

	now := time.Now().UTC()

	err = usecase.ServerRepository.UpdateServerShortName(ctxContext, serverId, payload.ShortName, userId, now)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *ServerUsecase) UpdateServerCategory(ctx *fiber.Ctx, userId uuid.UUID, serverIdParam string, payload model.ServerUpdateCategoryRequest) error {
	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	ctxContext := ctx.Context()

	if payload.CategoryId != nil {
		exists, err := usecase.ServerRepository.CheckServerCategories(ctxContext, *payload.CategoryId)
		if err != nil {
			return err
		}

		if exists != 1 {
			return &model.ValidationError{
				Code:    constant.ERR_VALIDATION_CODE,
				Message: "Category id is not found",
				Param:   "categoryId",
			}
		}
	}

	exists, err := usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return err
	}

	if exists != 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the owner of this server",
			Param:   "serverId",
		}
	}

	now := time.Now().UTC()

	err = usecase.ServerRepository.UpdateServerCategory(ctxContext, serverId, payload.CategoryId, userId, now)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *ServerUsecase) UpdateServerDescription(ctx *fiber.Ctx, userId uuid.UUID, serverIdParam string, payload model.ServerUpdateDescriptionRequest) error {
	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	ctxContext := ctx.Context()

	exists, err := usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return err
	}

	if exists != 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the owner of this server",
			Param:   "serverId",
		}
	}

	now := time.Now().UTC()

	err = usecase.ServerRepository.UpdateServerDescription(ctxContext, serverId, payload.Description, userId, now)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *ServerUsecase) DeleteServer(ctx *fiber.Ctx, userId uuid.UUID, serverIdParam string) error {
	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	ctxContext := ctx.Context()

	exists, err := usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return err
	}

	if exists != 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the owner of this server",
			Param:   "serverId",
		}
	}

	err = usecase.ServerRepository.DeleteServer(ctxContext, serverId)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *ServerUsecase) UpdateServerAvatar(ctx *fiber.Ctx, userId uuid.UUID, serverIdParam string) error {
	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	ctxContext := ctx.Context()

	exists, err := usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return err
	}

	if exists != 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the owner of this server",
			Param:   "serverId",
		}
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
		serverAvatarImage.ServerId = serverId
		serverAvatarImage.Bucket = bucketName
		serverAvatarImage.ObjectKey = fmt.Sprintf("server/avatar/%s.webp", *avatarImageId)
		serverAvatarImage.MimeType = "webp"
		serverAvatarImage.CreateDatetime = now
		serverAvatarImage.UpdateDatetime = now
		serverAvatarImage.CreateUserId = userId
		serverAvatarImage.UpdateUserId = userId
	}

	commited := false

	// start transaction
	tx, err := usecase.DB.Begin(ctxContext)
	if err != nil {
		return err
	}

	defer func() {
		if !commited {
			_ = tx.Rollback(ctxContext)
		}
	}()

	fileName, err := usecase.ServerRepository.GetServerAvatar(ctxContext, tx, serverId)
	if err != nil {
		return err
	}

	err = usecase.ServerRepository.UpdateServerAvatarImage(ctxContext, tx, serverId, avatarImageId, userId, now)
	if err != nil {
		return err
	}

	if avatarImageId != nil {
		err = usecase.ServerRepository.CreateServerAvatarImage(ctxContext, tx, serverAvatarImage)
		if err != nil {
			return err
		}
		err = usecase.ServerRepository.UploadObject(ctxContext, bucketName, serverAvatarImage.ObjectKey, imageFile, imageSize)
		if err != nil {
			return err
		}
	} else {
		err = usecase.ServerRepository.DeleteServerAvatarImage(ctxContext, tx, serverId)
		if err != nil {
			return err
		}

		err = usecase.ServerRepository.RemoveServerAvatarObject(ctxContext, bucketName, fileName)
		if err != nil {
			return err
		}
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		return err
	}

	commited = true

	return nil
}

func (usecase *ServerUsecase) UpdateServerBanner(ctx *fiber.Ctx, userId uuid.UUID, serverIdParam string) error {
	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	ctxContext := ctx.Context()

	exists, err := usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return err
	}

	if exists != 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the owner of this server",
			Param:   "serverId",
		}
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
		serverBannerImage.ServerId = serverId
		serverBannerImage.Bucket = bucketName
		serverBannerImage.ObjectKey = fmt.Sprintf("server/banner/%s.webp", *bannerImageId)
		serverBannerImage.MimeType = "webp"
		serverBannerImage.CreateDatetime = now
		serverBannerImage.UpdateDatetime = now
		serverBannerImage.CreateUserId = userId
		serverBannerImage.UpdateUserId = userId
	}

	commited := false

	// start transaction
	tx, err := usecase.DB.Begin(ctxContext)
	if err != nil {
		return err
	}

	defer func() {
		if !commited {
			_ = tx.Rollback(ctxContext)
		}
	}()

	fileName, err := usecase.ServerRepository.GetServerBanner(ctxContext, tx, serverId)
	if err != nil {
		return err
	}

	err = usecase.ServerRepository.UpdateServerBannerImage(ctxContext, tx, serverId, bannerImageId, userId, now)
	if err != nil {
		return err
	}

	if bannerImageId != nil {
		err = usecase.ServerRepository.CreateServerBannerImage(ctxContext, tx, serverBannerImage)
		if err != nil {
			return err
		}
		err = usecase.ServerRepository.UploadObject(ctxContext, bucketName, serverBannerImage.ObjectKey, imageFile, imageSize)
		if err != nil {
			return err
		}
	} else {
		err = usecase.ServerRepository.DeleteServerBannerImage(ctxContext, tx, serverId)
		if err != nil {
			return err
		}

		err = usecase.ServerRepository.RemoveServerBannerObject(ctxContext, bucketName, fileName)
		if err != nil {
			return err
		}
	}

	err = tx.Commit(ctxContext)
	if err != nil {
		return err
	}

	commited = true

	return nil
}

func (usecase *ServerUsecase) UpdateServerSettings(ctx *fiber.Ctx, userId uuid.UUID, serverIdParam string, payload model.ServerSettingsCreateRequest) error {
	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	ctxContext := ctx.Context()

	exists, err := usecase.ServerRepository.CheckServerOwnership(ctxContext, serverId, userId)
	if err != nil {
		return err
	}

	if exists != 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the owner of this server",
			Param:   "serverId",
		}
	}

	settingsBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	err = usecase.ServerRepository.UpdateServerSettings(ctxContext, serverId, settingsBytes, userId, now)
	if err != nil {
		return err
	}

	return nil
}

package usecase

import (
	"bytes"
	"encoding/base64"
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

type PostUsecase struct {
	PostRepository *repository.PostRepository
	DB             *pgxpool.Pool
	Log            *zap.Logger
	Config         *koanf.Koanf
}

func NewPostUsecase(postRepository *repository.PostRepository, db *pgxpool.Pool, zap *zap.Logger, koanf *koanf.Koanf) *PostUsecase {
	return &PostUsecase{
		PostRepository: postRepository,
		DB:             db,
		Log:            zap,
		Config:         koanf,
	}
}

func (usecase *PostUsecase) CreatePost(ctx *fiber.Ctx, serverId uuid.UUID, userId uuid.UUID) error {
	ctxContext := ctx.Context()

	// Check if user is a member of the server
	exists, err := usecase.PostRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return err
	}

	if exists != 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not a member of this server",
			Param:   "serverId",
		}
	}

	// Validate and get image file
	fieldName := "image"
	fileHeader, err := ctx.FormFile(fieldName)
	if err != nil {
		return err
	}

	var imageFile *bytes.Reader
	var imageSize int64
	var postImageId *uuid.UUID

	if fileHeader.Size != 0 {
		imageFile, imageSize, err = util.ValidateImage(fileHeader, fieldName)
		if err != nil {
			return err
		}

		id := uuid.New()
		postImageId = &id
	} else {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Image is required",
			Param:   "image",
		}
	}

	// Validate caption
	caption := ctx.FormValue("caption")
	if caption == "" {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Caption is required",
			Param:   "caption",
		}
	}

	now := time.Now().UTC()
	postId := uuid.New()

	// Create post image struct
	bucketName := usecase.Config.String("MINIO_BUCKET_NAME")

	serverPostImage := model.ServerPostImages{
		Id:             *postImageId,
		Bucket:         bucketName,
		ObjectKey:      fmt.Sprintf("server/post/%s.webp", *postImageId),
		MimeType:       "webp",
		Size:           imageSize,
		CreateDatetime: now,
		UpdateDatetime: now,
		CreateUserId:   userId,
		UpdateUserId:   userId,
	}

	// Create post struct
	serverPost := model.ServerPosts{
		Id:             postId,
		ServerId:       serverId,
		AuthorId:       userId,
		PostImageId:    *postImageId,
		Caption:        caption,
		CreateDatetime: now,
		UpdateDatetime: now,
		CreateUserId:   userId,
		UpdateUserId:   userId,
	}

	commited := false

	// Start transaction
	tx, err := usecase.DB.Begin(ctxContext)
	if err != nil {
		return err
	}

	defer func() {
		if !commited {
			_ = tx.Rollback(ctxContext)
		}
	}()

	// Upload image to MinIO
	err = usecase.PostRepository.UploadPostObject(ctxContext, bucketName, serverPostImage.ObjectKey, imageFile, imageSize)
	if err != nil {
		return err
	}

	// Insert post image to database
	err = usecase.PostRepository.CreateServerPostImage(ctxContext, tx, serverPostImage)
	if err != nil {
		return err
	}

	// Insert post to database
	err = usecase.PostRepository.CreateServerPost(ctxContext, tx, serverPost)
	if err != nil {
		return err
	}

	// Commit transaction
	err = tx.Commit(ctxContext)
	if err != nil {
		return err
	}

	commited = true

	return nil
}

func (usecase *PostUsecase) UpdatePostCaption(ctx *fiber.Ctx, serverIdParam string, postIdParam string, userId uuid.UUID, payload model.ServerPostUpdateCaptionRequest) error {
	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	postId, err := uuid.Parse(postIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid post id",
			Param:   "postId",
		}
	}

	// Validate caption
	if payload.Caption == "" {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Caption is required",
			Param:   "caption",
		}
	}

	ctxContext := ctx.Context()

	// Check if user is a member of the server
	serverMemberExists, err := usecase.PostRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return err
	}

	if serverMemberExists != 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not a member of this server",
			Param:   "serverId",
		}
	}

	// Check if user is the author of the post
	postOwnerExists, err := usecase.PostRepository.CheckPostOwnership(ctxContext, postId, userId)
	if err != nil {
		return err
	}

	if postOwnerExists != 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the author of this post",
			Param:   "postId",
		}
	}

	now := time.Now().UTC()

	err = usecase.PostRepository.UpdatePostCaption(ctxContext, postId, payload.Caption, userId, now)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *PostUsecase) DeletePost(ctx *fiber.Ctx, serverIdParam string, postIdParam string, userId uuid.UUID) error {
	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	postId, err := uuid.Parse(postIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid post id",
			Param:   "postId",
		}
	}

	ctxContext := ctx.Context()

	// Check if user is a member of the server
	serverMemberExists, err := usecase.PostRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return err
	}

	if serverMemberExists != 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not a member of this server",
			Param:   "serverId",
		}
	}

	// Check if user is the author of the post
	postOwnerExists, err := usecase.PostRepository.CheckPostOwnership(ctxContext, postId, userId)
	if err != nil {
		return err
	}

	if postOwnerExists != 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the author of this post",
			Param:   "postId",
		}
	}

	commited := false

	// Start transaction
	tx, err := usecase.DB.Begin(ctxContext)
	if err != nil {
		return err
	}

	defer func() {
		if !commited {
			_ = tx.Rollback(ctxContext)
		}
	}()

	// Get post image info before deleting
	postImageId, objectKey, err := usecase.PostRepository.GetPostImage(ctxContext, tx, postId)
	if err != nil {
		return err
	}

	if postImageId == uuid.Nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Post not found",
			Param:   "postId",
		}
	}

	// Delete post (CASCADE will delete comments and likes)
	err = usecase.PostRepository.DeletePost(ctxContext, postId)
	if err != nil {
		return err
	}

	// Delete post image
	err = usecase.PostRepository.DeletePostImage(ctxContext, tx, postImageId)
	if err != nil {
		return err
	}

	// Commit transaction first
	err = tx.Commit(ctxContext)
	if err != nil {
		return err
	}

	commited = true

	// Delete from MinIO after successful commit
	bucketName := usecase.Config.String("MINIO_BUCKET_NAME")
	err = usecase.PostRepository.DeletePostObject(ctxContext, bucketName, objectKey)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *PostUsecase) GetServerPosts(ctx *fiber.Ctx, serverIdParam string, userId uuid.UUID) (model.ServerPostListResponse, error) {
	response := model.ServerPostListResponse{}

	limit := ctx.QueryInt("limit", constant.DEFAULT_LIMIT)
	cursor := ctx.Query("cursor", "")

	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

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

	// Check if user is a member of the server
	serverMemberExists, err := usecase.PostRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}

	if serverMemberExists != 1 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not a member of this server",
			Param:   "serverId",
		}
	}

	var serverPostCursor model.ServerPostCursor
	if cursor != "" {
		b, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return response, err
		}

		err = sonic.Unmarshal(b, &serverPostCursor)
		if err != nil {
			return response, err
		}
	}

	MINIO_FULL_URL := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))

	// Fetch limit + 1 untuk cek apakah ada data lagi
	serverPosts, err := usecase.PostRepository.GetServerPosts(ctxContext, limit+1, serverId, &serverPostCursor, MINIO_FULL_URL)
	if err != nil {
		return response, err
	}

	// Add URL prefix to post images
	for i := range serverPosts {
		serverPosts[i].PostImageUrl = fmt.Sprintf("%s/%s.webp", MINIO_FULL_URL, serverPosts[i].PostImageUrl)
	}

	// Initialize with empty array
	response.Data = []model.ServerPostResponse{}

	if len(serverPosts) > limit {
		// Ada data lagi, return limit items dan buat cursor
		response.Data = serverPosts[:limit]

		last := serverPosts[limit-1]

		// Create cursor properly using ServerPostCursor
		postCursor := model.ServerPostCursor{
			Id:             last.PostId,
			CreateDatetime: last.CreateDatetime,
		}

		b, err := sonic.Marshal(postCursor)
		if err != nil {
			return response, err
		}

		response.Page.NextCursor = base64.RawURLEncoding.EncodeToString(b)
	} else {
		// Tidak ada data lagi, return semua data tanpa cursor
		if len(serverPosts) > 0 {
			response.Data = serverPosts
		}
		// Jika kosong, Data sudah []empty array dari inisialisasi
	}

	return response, nil
}

func (usecase *PostUsecase) LikePost(ctx *fiber.Ctx, postIdParam string, userId uuid.UUID) error {
	postId, err := uuid.Parse(postIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid post id",
			Param:   "postId",
		}
	}

	ctxContext := ctx.Context()

	// Check if user is a member of the server where the post belongs (single query)
	serverMemberExists, err := usecase.PostRepository.CheckPostServerMember(ctxContext, postId, userId)
	if err != nil {
		return err
	}

	if serverMemberExists != 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not a member of this server",
			Param:   "postId",
		}
	}

	// Check if user already liked this post
	likeExists, err := usecase.PostRepository.CheckPostLike(ctxContext, postId, userId)
	if err != nil {
		return err
	}

	if likeExists == 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You already liked this post",
			Param:   "postId",
		}
	}

	now := time.Now().UTC()

	postLike := model.ServerPostLikes{
		PostId:         postId,
		UserId:         userId,
		CreateDatetime: now,
		UpdateDatetime: now,
		CreateUserId:   userId,
		UpdateUserId:   userId,
	}

	err = usecase.PostRepository.CreatePostLike(ctxContext, postLike)
	if err != nil {
		return err
	}

	return nil
}

func (usecase *PostUsecase) UnlikePost(ctx *fiber.Ctx, postIdParam string, userId uuid.UUID) error {
	postId, err := uuid.Parse(postIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid post id",
			Param:   "postId",
		}
	}

	ctxContext := ctx.Context()

	// Check if user is a member of the server where the post belongs (single query)
	serverMemberExists, err := usecase.PostRepository.CheckPostServerMember(ctxContext, postId, userId)
	if err != nil {
		return err
	}

	if serverMemberExists != 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not a member of this server",
			Param:   "postId",
		}
	}

	// Check if user already liked this post
	likeExists, err := usecase.PostRepository.CheckPostLike(ctxContext, postId, userId)
	if err != nil {
		return err
	}

	if likeExists != 1 {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You haven't liked this post yet",
			Param:   "postId",
		}
	}

	err = usecase.PostRepository.DeletePostLike(ctxContext, postId, userId)
	if err != nil {
		return err
	}

	return nil
}

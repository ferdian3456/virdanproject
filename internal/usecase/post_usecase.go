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
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

func (usecase *PostUsecase) CreatePost(ctx fiber.Ctx, serverId uuid.UUID, userId uuid.UUID) (model.ServerPostResponse, error) {
	response := model.ServerPostResponse{}
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.CreatePost")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("server.id", serverId.String()),
	)

	// Check if user is a member of the server
	exists, err := usecase.PostRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}

	if exists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not a member of this server",
			Param:   "serverId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "CreatePost")
		return response, err
	}

	// Validate and get image file
	fieldName := "image"
	fileHeader, err := ctx.FormFile(fieldName)
	if err != nil {
		return response, err
	}

	var imageFile *bytes.Reader
	var imageSize int64
	var postImageId *uuid.UUID

	if fileHeader.Size != 0 {
		imageFile, imageSize, err = util.ValidateImage(fileHeader, fieldName)
		if err != nil {
			if validationErr, ok := err.(*model.ValidationError); ok {
				util.RecordValidationError(ctxContext, usecase.Log, span, validationErr, "CreatePost")
			}
			return response, err
		}

		id := uuid.New()
		postImageId = &id
	} else {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Image is required",
			Param:   "image",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "CreatePost")
		return response, err
	}

	// Validate caption
	caption := ctx.FormValue("caption")
	if caption == "" {
		return response, &model.ValidationError{
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
		usecase.Log.Error("Failed to begin transaction", zap.Error(err))
		return response, err
	}

	defer func() {
		if !commited {
			_ = tx.Rollback(ctxContext)
		}
	}()

	// Upload image to MinIO
	err = usecase.PostRepository.UploadPostObject(ctxContext, bucketName, serverPostImage.ObjectKey, imageFile, imageSize)
	if err != nil {
		return response, err
	}

	// Insert post image to database
	err = usecase.PostRepository.CreateServerPostImage(ctxContext, tx, serverPostImage)
	if err != nil {
		return response, err
	}

	// Insert post to database
	err = usecase.PostRepository.CreateServerPost(ctxContext, tx, serverPost)
	if err != nil {
		return response, err
	}

	// Commit transaction
	err = tx.Commit(ctxContext)
	if err != nil {
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to commit transaction", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return response, err
	}

	commited = true

	// Fetch full post object after creation
	MINIO_FULL_URL := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))
	response, err = usecase.PostRepository.GetPost(ctxContext, postId, userId, MINIO_FULL_URL)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (usecase *PostUsecase) GetServerPostForMe(ctx fiber.Ctx, serverIdParam string, userId uuid.UUID) (model.ServerPostForMeResponse, error) {
	response := model.ServerPostForMeResponse{}

	limit := fiber.Query(ctx, "limit", constant.DEFAULT_LIMIT)
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
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.GetServerPostForMe")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("server.id", serverIdParam),
		attribute.Int("limit", limit),
		attribute.String("cursor", cursor),
	)

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
	serverPosts, err := usecase.PostRepository.GetServerPostForMe(ctxContext, limit+1, serverId, userId, &serverPostCursor, MINIO_FULL_URL)
	if err != nil {
		return response, err
	}

	// Initialize with empty array
	response.Data = []model.ServerPostForMe{}

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

func (usecase *PostUsecase) UpdatePostCaption(ctx fiber.Ctx, serverIdParam string, postIdParam string, userId uuid.UUID, payload model.ServerPostUpdateCaptionRequest) (model.ServerPostResponse, error) {
	response := model.ServerPostResponse{}

	serverId, err := uuid.Parse(serverIdParam)
	if err != nil {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid server id",
			Param:   "serverId",
		}
	}

	postId, err := uuid.Parse(postIdParam)
	if err != nil {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid post id",
			Param:   "postId",
		}
	}

	// Validate caption
	if payload.Caption == "" {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Caption is required",
			Param:   "caption",
		}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.UpdatePostCaption")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("server.id", serverIdParam),
		attribute.String("post.id", postIdParam),
	)

	// Check if user is a member of the server
	serverMemberExists, err := usecase.PostRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return response, err
	}

	if serverMemberExists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not a member of this server",
			Param:   "serverId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "UpdatePostCaption")
		return response, err
	}

	// Check if user is the author of the post
	postOwnerExists, err := usecase.PostRepository.CheckPostOwnership(ctxContext, postId, userId)
	if err != nil {
		return response, err
	}

	if postOwnerExists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the author of this post",
			Param:   "postId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "UpdatePostCaption")
		return response, err
	}

	now := time.Now().UTC()

	err = usecase.PostRepository.UpdatePostCaption(ctxContext, postId, payload.Caption, userId, now)
	if err != nil {
		return response, err
	}

	// Fetch full post object after update
	MINIO_FULL_URL := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))
	response, err = usecase.PostRepository.GetPost(ctxContext, postId, userId, MINIO_FULL_URL)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (usecase *PostUsecase) DeletePost(ctx fiber.Ctx, serverIdParam string, postIdParam string, userId uuid.UUID) error {
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
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.DeletePost")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("server.id", serverIdParam),
		attribute.String("post.id", postIdParam),
	)

	// Check if user is a member of the server
	serverMemberExists, err := usecase.PostRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		usecase.Log.Error("Failed to check server member", zap.Error(err))
		return err
	}

	if serverMemberExists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not a member of this server",
			Param:   "serverId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "DeletePost")
		return err
	}

	// Check if user is the author of the post
	postOwnerExists, err := usecase.PostRepository.CheckPostOwnership(ctxContext, postId, userId)
	if err != nil {
		return err
	}

	if postOwnerExists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the author of this post",
			Param:   "postId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "DeletePost")
		return err
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
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Post not found",
			Param:   "postId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "DeletePost")
		return err
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
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("Failed to commit transaction", zap.Error(err))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
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

func (usecase *PostUsecase) GetServerPosts(ctx fiber.Ctx, serverIdParam string, userId uuid.UUID) (model.ServerPostListResponse, error) {
	response := model.ServerPostListResponse{}

	limit := fiber.Query[int](ctx, "limit", constant.DEFAULT_LIMIT)
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
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.GetServerPosts")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("server.id", serverIdParam),
		attribute.Int("limit", limit),
		attribute.String("cursor", cursor),
	)

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
	serverPosts, err := usecase.PostRepository.GetServerPosts(ctxContext, limit+1, serverId, userId, &serverPostCursor, MINIO_FULL_URL)
	if err != nil {
		return response, err
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

func (usecase *PostUsecase) GetPost(ctx fiber.Ctx, postIdParam string, userId uuid.UUID) (model.ServerPostResponse, error) {
	var response model.ServerPostResponse

	postId, err := uuid.Parse(postIdParam)
	if err != nil {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid post id",
			Param:   "postId",
		}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.GetPost")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("post.id", postIdParam),
	)

	// Check if user is a member of the server where the post belongs (single query)
	serverMemberExists, err := usecase.PostRepository.CheckPostServerMember(ctxContext, postId, userId)
	if err != nil {
		return response, err
	}

	if serverMemberExists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not a member of this server",
			Param:   "postId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "GetPost")
		return response, err
	}

	MINIO_FULL_URL := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))

	response, err = usecase.PostRepository.GetPost(ctxContext, postId, userId, MINIO_FULL_URL)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (usecase *PostUsecase) LikePost(ctx fiber.Ctx, postIdParam string, userId uuid.UUID) (model.PostLikeResponse, error) {
	response := model.PostLikeResponse{}

	postId, err := uuid.Parse(postIdParam)
	if err != nil {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid post id",
			Param:   "postId",
		}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.LikePost")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("post.id", postIdParam),
	)

	// Check if user is a member of the server where the post belongs (single query)
	serverMemberExists, err := usecase.PostRepository.CheckPostServerMember(ctxContext, postId, userId)
	if err != nil {
		return response, err
	}

	if serverMemberExists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not a member of this server",
			Param:   "postId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "LikePost")
		return response, err
	}

	// Check if user already liked this post
	likeExists, err := usecase.PostRepository.CheckPostLike(ctxContext, postId, userId)
	if err != nil {
		return response, err
	}

	// Idempotent: only insert if not already liked
	if likeExists != 1 {
		now := time.Now().UTC()

		postLike := model.ServerPostLikes{
			Id:             uuid.New(),
			PostId:         postId,
			UserId:         userId,
			CreateDatetime: now,
			UpdateDatetime: now,
			CreateUserId:   userId,
			UpdateUserId:   userId,
		}

		err = usecase.PostRepository.CreatePostLike(ctxContext, postLike)
		if err != nil {
			return response, err
		}
	}

	// Get like count (single query, no MinIO needed)
	likeCount, err := usecase.PostRepository.GetPostLikeCount(ctxContext, postId)
	if err != nil {
		return response, err
	}

	response.LikeCount = likeCount
	return response, nil
}

func (usecase *PostUsecase) UnlikePost(ctx fiber.Ctx, postIdParam string, userId uuid.UUID) (model.PostLikeResponse, error) {
	response := model.PostLikeResponse{}

	postId, err := uuid.Parse(postIdParam)
	if err != nil {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid post id",
			Param:   "postId",
		}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.UnlikePost")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("post.id", postIdParam),
	)

	// Check if user is a member of the server where the post belongs (single query)
	serverMemberExists, err := usecase.PostRepository.CheckPostServerMember(ctxContext, postId, userId)
	if err != nil {
		return response, err
	}

	if serverMemberExists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not a member of this server",
			Param:   "postId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "UnlikePost")
		return response, err
	}

	// Check if user already liked this post
	likeExists, err := usecase.PostRepository.CheckPostLike(ctxContext, postId, userId)
	if err != nil {
		return response, err
	}

	// Idempotent: only delete if already liked
	if likeExists == 1 {
		err = usecase.PostRepository.DeletePostLike(ctxContext, postId, userId)
		if err != nil {
			return response, err
		}
	}

	// Get like count (single query, no MinIO needed)
	likeCount, err := usecase.PostRepository.GetPostLikeCount(ctxContext, postId)
	if err != nil {
		return response, err
	}

	response.LikeCount = likeCount
	return response, nil
}

func (usecase *PostUsecase) CreateComment(ctx fiber.Ctx, postIdParam string, userId uuid.UUID, payload model.ServerCommentCreateRequest) (model.ServerCommentResponse, error) {
	response := model.ServerCommentResponse{}

	postId, err := uuid.Parse(postIdParam)
	if err != nil {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid post id",
			Param:   "postId",
		}
	}

	// Validate content length
	if payload.Content == "" {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Content is required",
			Param:   "content",
		}
	} else if len(payload.Content) < 1 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Content must be at least 1 character",
			Param:   "content",
		}
	} else if len(payload.Content) > 1000 {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Content must be at most 1000 characters",
			Param:   "content",
		}
	}

	// Parse parentId if provided and validate it's a valid UUID
	var parentCommentId *uuid.UUID
	if payload.ParentId != nil {
		parsedParentId, err := uuid.Parse(*payload.ParentId)
		if err != nil {
			return response, &model.ValidationError{
				Code:    constant.ERR_VALIDATION_CODE,
				Message: "Invalid parent comment id",
				Param:   "parentId",
			}
		}
		parentCommentId = &parsedParentId
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.CreateComment")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("post.id", postIdParam),
	)

	// Check if user is a member of the server where the post belongs (single query)
	serverMemberExists, err := usecase.PostRepository.CheckPostServerMember(ctxContext, postId, userId)
	if err != nil {
		return response, err
	}

	if serverMemberExists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not a member of this server",
			Param:   "postId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "CreateComment")
		return response, err
	}

	// If parentId is provided, check if parent comment exists and belongs to the same post
	if parentCommentId != nil {
		parentExists, err := usecase.PostRepository.CheckCommentExists(ctxContext, *parentCommentId, postId)
		if err != nil {
			return response, err
		}

		if parentExists != 1 {
			err := &model.ValidationError{
				Code:    constant.ERR_VALIDATION_CODE,
				Message: "Parent comment not found",
				Param:   "parentId",
			}
			util.RecordValidationError(ctxContext, usecase.Log, span, err, "CreateComment")
			return response, err
		}
	}

	now := time.Now().UTC()
	commentId := uuid.New()

	comment := model.ServerPostComments{
		Id:             commentId,
		PostId:         postId,
		AuthorId:       userId,
		ParentId:       parentCommentId,
		Content:        payload.Content,
		CreateDatetime: now,
		UpdateDatetime: now,
		CreateUserId:   userId,
		UpdateUserId:   userId,
	}

	err = usecase.PostRepository.CreateComment(ctxContext, comment)
	if err != nil {
		return response, err
	}

	username, err := usecase.PostRepository.GetAuthorNameById(ctxContext, userId)
	if err != nil {
		return response, err
	}

	// Construct response from the created comment
	response = model.ServerCommentResponse{
		Id:             commentId,
		AuthorId:       userId,
		AuthorName:     username,
		ParentId:       parentCommentId,
		Content:        payload.Content,
		CreateDatetime: now,
		UpdateDatetime: now,
	}

	return response, nil
}

func (usecase *PostUsecase) GetComments(ctx fiber.Ctx, postIdParam string, userId uuid.UUID) (model.ServerCommentListResponse, error) {
	response := model.ServerCommentListResponse{}

	limit := fiber.Query[int](ctx, "limit", constant.DEFAULT_LIMIT)
	cursor := ctx.Query("cursor", "")

	postId, err := uuid.Parse(postIdParam)
	if err != nil {
		return response, &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid post id",
			Param:   "postId",
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
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.GetComments")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("post.id", postIdParam),
		attribute.Int("limit", limit),
		attribute.String("cursor", cursor),
	)

	// Check if user is a member of the server where the post belongs (single query)
	serverMemberExists, err := usecase.PostRepository.CheckPostServerMember(ctxContext, postId, userId)
	if err != nil {
		return response, err
	}

	if serverMemberExists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not a member of this server",
			Param:   "postId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "GetComments")
		return response, err
	}

	var serverCommentCursor model.ServerCommentCursor
	if cursor != "" {
		b, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return response, err
		}

		err = sonic.Unmarshal(b, &serverCommentCursor)
		if err != nil {
			return response, err
		}
	}

	MINIO_FULL_URL := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))

	// Fetch limit + 1 to check if there's more data
	comments, err := usecase.PostRepository.GetComments(ctxContext, limit+1, postId, &serverCommentCursor, MINIO_FULL_URL)
	if err != nil {
		return response, err
	}

	// Initialize with empty array
	response.Data = []model.ServerCommentResponse{}

	if len(comments) > limit {
		// There's more data, return limit items and create cursor
		response.Data = comments[:limit]

		last := comments[limit-1]

		// Create cursor properly using ServerCommentCursor
		commentCursor := model.ServerCommentCursor{
			Id:             last.Id,
			CreateDatetime: last.CreateDatetime,
		}

		b, err := sonic.Marshal(commentCursor)
		if err != nil {
			return response, err
		}

		response.Page.NextCursor = base64.RawURLEncoding.EncodeToString(b)
	} else {
		// No more data, return all data without cursor
		if len(comments) > 0 {
			response.Data = comments
		}
		// If empty, Data is already []empty array from initialization
	}

	return response, nil
}

func (usecase *PostUsecase) DeleteComment(ctx fiber.Ctx, postIdParam string, commentIdParam string, userId uuid.UUID) error {
	postId, err := uuid.Parse(postIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid post id",
			Param:   "postId",
		}
	}

	commentId, err := uuid.Parse(commentIdParam)
	if err != nil {
		return &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "Invalid comment id",
			Param:   "commentId",
		}
	}

	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	tr := otel.Tracer(serviceName + "-usecase")
	ctxContext, span := tr.Start(ctxContext, "usecase.DeleteComment")
	defer span.End()

	span.SetAttributes(
		attribute.String("user.id", userId.String()),
		attribute.String("post.id", postIdParam),
		attribute.String("comment.id", commentIdParam),
	)

	// Check if user is a member of the server where the post belongs (single query)
	serverMemberExists, err := usecase.PostRepository.CheckPostServerMember(ctxContext, postId, userId)
	if err != nil {
		return err
	}

	if serverMemberExists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not a member of this server",
			Param:   "postId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "DeleteComment")
		return err
	}

	// Check if user is the author of the comment
	commentOwnerExists, err := usecase.PostRepository.CheckCommentOwnership(ctxContext, commentId, userId)
	if err != nil {
		return err
	}

	if commentOwnerExists != 1 {
		err := &model.ValidationError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "You are not the author of this comment",
			Param:   "commentId",
		}
		util.RecordValidationError(ctxContext, usecase.Log, span, err, "DeleteComment")
		return err
	}

	err = usecase.PostRepository.DeleteComment(ctxContext, commentId)
	if err != nil {
		return err
	}

	return nil
}

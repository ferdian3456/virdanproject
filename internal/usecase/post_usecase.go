package usecase

import (
	"bytes"
	"fmt"
	"time"

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

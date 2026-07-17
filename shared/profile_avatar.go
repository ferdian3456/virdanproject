package shared

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
)

type ProfileAvatarRepo interface {
	CheckProfileAvatarImageOwnership(ctx context.Context, userId, imageId string) (bool, error)
	CreateProfileAvatarImage(ctx context.Context, tx pgx.Tx, image ProfileAvatarImage) error
	UploadObject(ctx context.Context, bucket, objectKey string, file *bytes.Reader, size int64) error
}

func ResolveProfileAvatar(
	ctxContext context.Context,
	tx pgx.Tx,
	fiberCtx fiber.Ctx,
	repo ProfileAvatarRepo,
	config *koanf.Koanf,
	log *zap.Logger,
	userId string,
	now time.Time,
) (*string, error) {
	formImageId := fiberCtx.FormValue("avatarImageId")
	fileHeader, fhErr := fiberCtx.FormFile("profileAvatar")
	hasFile := fhErr == nil && fileHeader != nil
	hasImageId := formImageId != ""

	if hasFile && hasImageId {
		return nil, &BadRequestError{
			Code:    ERR_VALIDATION_CODE,
			Message: "Provide either profileAvatar file or avatarImageId, not both",
			Param:   "avatarImageId",
		}
	}

	if hasImageId {
		v := NewValidator()
		v.UUID("avatarImageId", formImageId)
		if err := v.Validate(); err != nil {
			return nil, err
		}
		owned, err := repo.CheckProfileAvatarImageOwnership(ctxContext, userId, formImageId)
		if err != nil {
			return nil, err
		}
		if !owned {
			return nil, &ForbiddenError{
				Code:    ERR_FORBIDDEN_CODE,
				Message: "Avatar image is not owned by you",
				Param:   "avatarImageId",
			}
		}
		return ToPtr(formImageId), nil
	}

	if !hasFile {
		return nil, nil
	}

	imageReader, imageSize, _, _, err := ValidateImage(ctxContext, fileHeader, "profileAvatar", MAX_IMAGE_SIZE, 512, 512, true)
	if err != nil {
		return nil, err
	}

	newId := uuid.New().String()
	bucket := config.String("MINIO_BUCKET_NAME")
	objectKey := fmt.Sprintf("profile/avatar/%s.webp", newId)

	image := ProfileAvatarImage{
		Id:        newId,
		Bucket:    bucket,
		ObjectKey: objectKey,
		MimeType:  "image/webp",
		Size:      imageSize,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: userId,
		UpdatedBy: userId,
	}
	if err := repo.CreateProfileAvatarImage(ctxContext, tx, image); err != nil {
		return nil, err
	}

	if err := repo.UploadObject(ctxContext, bucket, objectKey, imageReader, imageSize); err != nil {
		GetLoggerWithTraceContext(ctxContext, log).Error("Failed to upload profile avatar object", zap.Error(err))
		return nil, err
	}

	return ToPtr(newId), nil
}

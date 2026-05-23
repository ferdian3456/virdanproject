package util

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
)

// ProfileAvatarRepo is the minimal repository surface ResolveProfileAvatar needs.
// Implemented by *repository.ProfileRepository.
type ProfileAvatarRepo interface {
	CheckProfileAvatarImageOwnership(ctx context.Context, userId, imageId string) (bool, error)
	CreateProfileAvatarImage(ctx context.Context, tx pgx.Tx, image model.ProfileAvatarImage) error
	UploadObject(ctx context.Context, bucket, objectKey string, file *bytes.Reader, size int64) error
}

// ResolveProfileAvatar resolves the per-server profile avatar from a multipart request,
// enforcing mutual exclusion between `profileAvatar` (file) and `avatarImageId` (existing).
//
// Behavior:
//   - profileAvatar file AND avatarImageId both present → 400 (ERR_VALIDATION_CODE)
//   - profileAvatar file present → validate image, INSERT profile_avatar_images, upload to MinIO,
//     return new id. MinIO object key: profile/avatar/{newId}.webp
//   - avatarImageId present → check ownership via CheckProfileAvatarImageOwnership, return as-is.
//   - Neither → return nil.
//
// Caller passes an open transaction. MinIO upload happens BEFORE tx commit so a rollback
// will leave an orphan object in MinIO — acceptable trade-off (cleanup is out of scope).
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
		return nil, &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
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
			return nil, &model.ForbiddenError{
				Code:    constant.ERR_FORBIDDEN_CODE,
				Message: "Avatar image is not owned by you",
				Param:   "avatarImageId",
			}
		}
		return ToPtr(formImageId), nil
	}

	if !hasFile {
		return nil, nil
	}

	imageReader, imageSize, err := ValidateImage(ctxContext, fileHeader, "profileAvatar")
	if err != nil {
		return nil, err
	}

	newId := uuid.New().String()
	bucket := config.String("MINIO_BUCKET_NAME")
	objectKey := fmt.Sprintf("profile/avatar/%s.webp", newId)

	image := model.ProfileAvatarImage{
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

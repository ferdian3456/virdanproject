package repository

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type PostRepository struct {
	Log      *zap.Logger
	DB       *pgxpool.Pool
	DBCache  *redis.Client
	DBObject *minio.Client
}

func NewPostRepository(zap *zap.Logger, db *pgxpool.Pool, dbCache *redis.Client, minio *minio.Client) *PostRepository {
	return &PostRepository{
		Log:      zap,
		DB:       db,
		DBCache:  dbCache,
		DBObject: minio,
	}
}

func (repository *PostRepository) CheckServerMember(ctx context.Context, serverId uuid.UUID, userId uuid.UUID) (int, error) {
	query := "SELECT 1 FROM server_members WHERE server_id = $1 AND user_id = $2 AND status = 'ACTIVE'"

	var exists int
	err := repository.DB.QueryRow(ctx, query, serverId, userId).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return exists, nil
		}

		return exists, err
	}

	return exists, nil
}

func (repository *PostRepository) UploadPostObject(ctx context.Context, bucketName string, imageName string, imageFile *bytes.Reader, imageSize int64) error {
	_, err := repository.DBObject.PutObject(ctx, bucketName, imageName, imageFile, imageSize,
		minio.PutObjectOptions{
			ContentType:  "image/webp",
			CacheControl: "public, max-age=31536000, immutable",
		})
	if err != nil {
		return err
	}

	return nil
}

func (repository *PostRepository) CreateServerPostImage(ctx context.Context, tx pgx.Tx, serverPostImage model.ServerPostImages) error {
	query := "INSERT INTO server_post_images (id, bucket, object_key, mime_type, size, create_datetime, update_datetime, create_user_id, update_user_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)"

	_, err := tx.Exec(ctx, query, serverPostImage.Id, serverPostImage.Bucket, serverPostImage.ObjectKey, serverPostImage.MimeType, serverPostImage.Size, serverPostImage.CreateDatetime, serverPostImage.UpdateDatetime, serverPostImage.CreateUserId, serverPostImage.UpdateUserId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *PostRepository) CreateServerPost(ctx context.Context, tx pgx.Tx, serverPost model.ServerPosts) error {
	query := "INSERT INTO server_posts (id, server_id, author_id, post_image_id, caption, create_datetime, update_datetime, create_user_id, update_user_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)"

	_, err := tx.Exec(ctx, query, serverPost.Id, serverPost.ServerId, serverPost.AuthorId, serverPost.PostImageId, serverPost.Caption, serverPost.CreateDatetime, serverPost.UpdateDatetime, serverPost.CreateUserId, serverPost.UpdateUserId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *PostRepository) CheckPostOwnership(ctx context.Context, postId uuid.UUID, userId uuid.UUID) (int, error) {
	query := "SELECT 1 FROM server_posts WHERE id = $1 AND author_id = $2"

	var exists int
	err := repository.DB.QueryRow(ctx, query, postId, userId).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return exists, nil
		}

		return exists, err
	}

	return exists, nil
}

func (repository *PostRepository) UpdatePostCaption(ctx context.Context, postId uuid.UUID, caption string, updateUserId uuid.UUID, updateDatetime time.Time) error {
	query := "UPDATE server_posts SET caption = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4"

	_, err := repository.DB.Exec(ctx, query, caption, updateDatetime, updateUserId, postId)
	if err != nil {
		return err
	}

	return nil
}

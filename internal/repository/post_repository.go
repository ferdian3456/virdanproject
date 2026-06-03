package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

type PostRepository struct {
	Log      *zap.Logger
	Config   *koanf.Koanf
	DB       *pgxpool.Pool
	DBCache  *redis.Client
	DBObject *minio.Client
}

func NewPostRepository(zap *zap.Logger, koanf *koanf.Koanf, db *pgxpool.Pool, dbCache *redis.Client, minio *minio.Client) *PostRepository {
	return &PostRepository{
		Log:      zap,
		Config:   koanf,
		DB:       db,
		DBCache:  dbCache,
		DBObject: minio,
	}
}

func (repository *PostRepository) UploadPostObject(ctx context.Context, bucketName string, objectKey string, file *bytes.Reader, size int64) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UploadPostObject")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("storage.system", "minio"),
		attribute.String("storage.operation", "Upload"),
		attribute.String("storage.bucket", bucketName),
		attribute.String("storage.object", objectKey),
	)

	_, err = repository.DBObject.PutObject(ctx, bucketName, objectKey, file, size,
		minio.PutObjectOptions{
			ContentType:  "image/webp",
			CacheControl: "public, max-age=31536000, immutable",
		})
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to upload post object to storage", zap.Error(err))
		return err
	}

	return nil
}

func (repository *PostRepository) CreateServerPostImage(ctx context.Context, tx pgx.Tx, image model.ServerPostImage) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CreateServerPostImage")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "INSERT"),
	)

	query := `INSERT INTO server_post_images (id, bucket, object_key, mime_type, size, created_at, updated_at, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err = tx.Exec(ctx, query, image.Id, image.Bucket, image.ObjectKey, image.MimeType, image.Size,
		image.CreatedAt, image.UpdatedAt, image.CreatedBy, image.UpdatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create server post image", zap.Error(err))
		return err
	}

	return nil
}

func (repository *PostRepository) CreateServerPost(ctx context.Context, tx pgx.Tx, post model.ServerPost) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CreateServerPost")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("server.id", post.ServerId),
		attribute.String("author.id", post.AuthorId),
	)

	query := `INSERT INTO server_posts (id, server_id, author_id, post_image_id, caption, created_at, updated_at, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err = tx.Exec(ctx, query, post.Id, post.ServerId, post.AuthorId, post.PostImageId, post.Caption,
		post.CreatedAt, post.UpdatedAt, post.CreatedBy, post.UpdatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create server post", zap.Error(err))
		return err
	}

	return nil
}

func (repository *PostRepository) CheckPostOwnership(ctx context.Context, postId string, userId string) (int, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CheckPostOwnership")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("post.id", postId),
		attribute.String("user.id", userId),
	)

	query := `SELECT COUNT(*) FROM server_posts WHERE id = $1 AND author_id = $2`

	var count int
	err = repository.DB.QueryRow(ctx, query, postId, userId).Scan(&count)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check post ownership", zap.Error(err))
		return 0, err
	}

	return count, nil
}

func (repository *PostRepository) UpdatePostCaption(ctx context.Context, postId string, caption string, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdatePostCaption")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("post.id", postId),
	)

	query := `UPDATE server_posts SET caption = $1, updated_at = $2, updated_by = $3 WHERE id = $4`

	_, err = repository.DB.Exec(ctx, query, caption, updatedAt, updatedBy, postId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update post caption", zap.Error(err))
		return err
	}

	return nil
}

func (repository *PostRepository) DeletePostHard(ctx context.Context, postId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeletePostHard")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "DELETE"),
		attribute.String("post.id", postId),
	)

	query := `DELETE FROM server_posts WHERE id = $1`

	_, err = repository.DB.Exec(ctx, query, postId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to hard delete post", zap.Error(err))
		return err
	}

	return nil
}

func (repository *PostRepository) GetPost(ctx context.Context, postId string, userId string, minioFullUrl string) (model.ServerPostResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetPost")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("post.id", postId),
		attribute.String("user.id", userId),
	)

	query := `
		SELECT sp.id, sp.server_id, sp.caption, sp.author_id,
			sp.created_at, sp.updated_at,
			spi.object_key,
			smp.nickname,
			smp.username,
			pai.object_key,
			CASE
				WHEN u.deleted_at IS NOT NULL THEN 'user_deleted'
				WHEN sm_author.user_id IS NULL THEN 'user_left'
				ELSE 'active'
			END AS author_status,
			(SELECT COUNT(*) FROM server_post_likes WHERE post_id = sp.id) AS like_count,
			(SELECT COUNT(*) FROM server_post_comments WHERE post_id = sp.id) AS comment_count,
			EXISTS (SELECT 1 FROM server_post_likes l WHERE l.post_id = sp.id AND l.user_id = $2) AS user_liked,
			EXISTS (SELECT 1 FROM server_post_saves s WHERE s.post_id = sp.id AND s.user_id = $2) AS user_saved
		FROM server_posts sp
		INNER JOIN users u ON sp.author_id = u.id
		INNER JOIN server_member_profiles smp ON smp.server_id = sp.server_id AND smp.user_id = sp.author_id
		LEFT JOIN server_members sm_author ON sm_author.server_id = sp.server_id AND sm_author.user_id = sp.author_id
		LEFT JOIN server_post_images spi ON sp.post_image_id = spi.id
		LEFT JOIN profile_avatar_images pai ON smp.avatar_image_id = pai.id
		WHERE sp.id = $1
		LIMIT 1`

	var resp model.ServerPostResponse
	var authorStatus string
	err = repository.DB.QueryRow(ctx, query, postId, userId).Scan(
		&resp.Id,
		&resp.ServerId,
		&resp.Caption,
		&resp.Author.UserId,
		&resp.CreatedAt,
		&resp.UpdatedAt,
		&resp.ImageUrl,
		&resp.Author.Nickname,
		&resp.Author.Username,
		&resp.Author.AvatarUrl,
		&authorStatus,
		&resp.LikeCount,
		&resp.CommentCount,
		&resp.UserLiked,
		&resp.UserSaved,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.NotFoundError{Code: constant.ERR_NOT_FOUND_CODE, Message: "Post not found", Param: "postId"}
			return model.ServerPostResponse{}, err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get post", zap.Error(err))
		return model.ServerPostResponse{}, err
	}

	resp.Author.Status = model.AuthorStatus(authorStatus)
	resp.IsOwner = resp.Author.UserId == userId

	if resp.ImageUrl != nil {
		*resp.ImageUrl = fmt.Sprintf("%s/%s", minioFullUrl, *resp.ImageUrl)
	}
	if resp.Author.AvatarUrl != nil {
		*resp.Author.AvatarUrl = fmt.Sprintf("%s/%s", minioFullUrl, *resp.Author.AvatarUrl)
	}

	return resp, nil
}

func (repository *PostRepository) GetServerPosts(ctx context.Context, limit int, serverId string, userId string, cursor *model.ServerPostCursor, minioFullUrl string) ([]model.ServerPostResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetServerPosts")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("server.id", serverId),
		attribute.String("user.id", userId),
	)

	baseSelect := `
		SELECT sp.id, sp.server_id, sp.caption, sp.author_id,
			sp.created_at, sp.updated_at,
			spi.object_key,
			smp.nickname,
			smp.username,
			pai.object_key,
			CASE
				WHEN u.deleted_at IS NOT NULL THEN 'user_deleted'
				WHEN sm_author.user_id IS NULL THEN 'user_left'
				ELSE 'active'
			END AS author_status,
			(SELECT COUNT(*) FROM server_post_likes WHERE post_id = sp.id) AS like_count,
			(SELECT COUNT(*) FROM server_post_comments WHERE post_id = sp.id) AS comment_count,
			EXISTS (SELECT 1 FROM server_post_likes l WHERE l.post_id = sp.id AND l.user_id = $2) AS user_liked,
			EXISTS (SELECT 1 FROM server_post_saves s WHERE s.post_id = sp.id AND s.user_id = $2) AS user_saved
		FROM server_posts sp
		INNER JOIN users u ON sp.author_id = u.id
		INNER JOIN server_member_profiles smp ON smp.server_id = sp.server_id AND smp.user_id = sp.author_id
		LEFT JOIN server_members sm_author ON sm_author.server_id = sp.server_id AND sm_author.user_id = sp.author_id
		LEFT JOIN server_post_images spi ON sp.post_image_id = spi.id
		LEFT JOIN profile_avatar_images pai ON smp.avatar_image_id = pai.id`

	var rows pgx.Rows
	if cursor.Id != "" && !cursor.CreatedAt.IsZero() {
		query := baseSelect + `
		WHERE sp.server_id = $1
		AND (sp.created_at < $3 OR (sp.created_at = $3 AND sp.id < $4))
		ORDER BY sp.created_at DESC, sp.id DESC
		LIMIT $5`
		rows, err = repository.DB.Query(ctx, query, serverId, userId, cursor.CreatedAt, cursor.Id, limit)
	} else {
		query := baseSelect + `
		WHERE sp.server_id = $1
		ORDER BY sp.created_at DESC, sp.id DESC
		LIMIT $3`
		rows, err = repository.DB.Query(ctx, query, serverId, userId, limit)
	}

	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query server posts", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	posts := []model.ServerPostResponse{}
	for rows.Next() {
		var resp model.ServerPostResponse
		var authorStatus string
		err = rows.Scan(
			&resp.Id,
			&resp.ServerId,
			&resp.Caption,
			&resp.Author.UserId,
			&resp.CreatedAt,
			&resp.UpdatedAt,
			&resp.ImageUrl,
			&resp.Author.Nickname,
			&resp.Author.Username,
			&resp.Author.AvatarUrl,
			&authorStatus,
			&resp.LikeCount,
			&resp.CommentCount,
			&resp.UserLiked,
			&resp.UserSaved,
		)
		if err != nil {
			util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan server post row", zap.Error(err))
			return nil, err
		}

		resp.Author.Status = model.AuthorStatus(authorStatus)
		resp.IsOwner = resp.Author.UserId == userId

		if resp.ImageUrl != nil {
			*resp.ImageUrl = fmt.Sprintf("%s/%s", minioFullUrl, *resp.ImageUrl)
		}
		if resp.Author.AvatarUrl != nil {
			*resp.Author.AvatarUrl = fmt.Sprintf("%s/%s", minioFullUrl, *resp.Author.AvatarUrl)
		}

		posts = append(posts, resp)
	}

	return posts, nil
}

func (repository *PostRepository) SearchServerPosts(ctx context.Context, limit int, serverId string, userId string, query string, cursor *model.ServerPostCursor, minioFullUrl string) ([]model.ServerPostResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.SearchServerPosts")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("server.id", serverId),
		attribute.String("user.id", userId),
	)

	baseSelect := `
		SELECT sp.id, sp.server_id, sp.caption, sp.author_id,
			sp.created_at, sp.updated_at,
			spi.object_key,
			smp.nickname,
			smp.username,
			pai.object_key,
			CASE
				WHEN u.deleted_at IS NOT NULL THEN 'user_deleted'
				WHEN sm_author.user_id IS NULL THEN 'user_left'
				ELSE 'active'
			END AS author_status,
			(SELECT COUNT(*) FROM server_post_likes WHERE post_id = sp.id) AS like_count,
			(SELECT COUNT(*) FROM server_post_comments WHERE post_id = sp.id) AS comment_count,
			EXISTS (SELECT 1 FROM server_post_likes l WHERE l.post_id = sp.id AND l.user_id = $2) AS user_liked,
			EXISTS (SELECT 1 FROM server_post_saves s WHERE s.post_id = sp.id AND s.user_id = $2) AS user_saved
		FROM server_posts sp
		INNER JOIN users u ON sp.author_id = u.id
		INNER JOIN server_member_profiles smp ON smp.server_id = sp.server_id AND smp.user_id = sp.author_id
		LEFT JOIN server_members sm_author ON sm_author.server_id = sp.server_id AND sm_author.user_id = sp.author_id
		LEFT JOIN server_post_images spi ON sp.post_image_id = spi.id
		LEFT JOIN profile_avatar_images pai ON smp.avatar_image_id = pai.id`

	var rows pgx.Rows
	if cursor.Id != "" && !cursor.CreatedAt.IsZero() {
		querySQL := baseSelect + `
		WHERE sp.server_id = $1
		AND sp.caption ILIKE '%' || $3 || '%' ESCAPE '\'
		AND (sp.created_at < $4 OR (sp.created_at = $4 AND sp.id < $5))
		ORDER BY sp.created_at DESC, sp.id DESC
		LIMIT $6`
		rows, err = repository.DB.Query(ctx, querySQL, serverId, userId, query, cursor.CreatedAt, cursor.Id, limit)
	} else {
		querySQL := baseSelect + `
		WHERE sp.server_id = $1
		AND sp.caption ILIKE '%' || $3 || '%' ESCAPE '\'
		ORDER BY sp.created_at DESC, sp.id DESC
		LIMIT $4`
		rows, err = repository.DB.Query(ctx, querySQL, serverId, userId, query, limit)
	}

	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query search server posts", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	posts := []model.ServerPostResponse{}
	for rows.Next() {
		var resp model.ServerPostResponse
		var authorStatus string
		err = rows.Scan(
			&resp.Id,
			&resp.ServerId,
			&resp.Caption,
			&resp.Author.UserId,
			&resp.CreatedAt,
			&resp.UpdatedAt,
			&resp.ImageUrl,
			&resp.Author.Nickname,
			&resp.Author.Username,
			&resp.Author.AvatarUrl,
			&authorStatus,
			&resp.LikeCount,
			&resp.CommentCount,
			&resp.UserLiked,
			&resp.UserSaved,
		)
		if err != nil {
			util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan search server post row", zap.Error(err))
			return nil, err
		}

		resp.Author.Status = model.AuthorStatus(authorStatus)
		resp.IsOwner = resp.Author.UserId == userId

		if resp.ImageUrl != nil {
			*resp.ImageUrl = fmt.Sprintf("%s/%s", minioFullUrl, *resp.ImageUrl)
		}
		if resp.Author.AvatarUrl != nil {
			*resp.Author.AvatarUrl = fmt.Sprintf("%s/%s", minioFullUrl, *resp.Author.AvatarUrl)
		}

		posts = append(posts, resp)
	}

	return posts, nil
}

func (repository *PostRepository) GetServerPostForMe(ctx context.Context, limit int, serverId string, userId string, cursor *model.ServerPostCursor, minioFullUrl string) ([]model.ServerPostResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetServerPostForMe")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("server.id", serverId),
		attribute.String("user.id", userId),
	)

	baseSelect := `
		SELECT sp.id, sp.server_id, sp.caption, sp.author_id,
			sp.created_at, sp.updated_at,
			spi.object_key,
			smp.nickname,
			smp.username,
			pai.object_key,
			(SELECT COUNT(*) FROM server_post_likes WHERE post_id = sp.id) AS like_count,
			(SELECT COUNT(*) FROM server_post_comments WHERE post_id = sp.id) AS comment_count,
			EXISTS (SELECT 1 FROM server_post_likes l WHERE l.post_id = sp.id AND l.user_id = $2) AS user_liked,
			EXISTS (SELECT 1 FROM server_post_saves s WHERE s.post_id = sp.id AND s.user_id = $2) AS user_saved
		FROM server_posts sp
		INNER JOIN server_member_profiles smp ON smp.server_id = sp.server_id AND smp.user_id = sp.author_id
		LEFT JOIN server_post_images spi ON sp.post_image_id = spi.id
		LEFT JOIN profile_avatar_images pai ON smp.avatar_image_id = pai.id`

	var rows pgx.Rows
	if cursor.Id != "" && !cursor.CreatedAt.IsZero() {
		query := baseSelect + `
		WHERE sp.server_id = $1 AND sp.author_id = $2
		AND (sp.created_at < $3 OR (sp.created_at = $3 AND sp.id < $4))
		ORDER BY sp.created_at DESC, sp.id DESC
		LIMIT $5`
		rows, err = repository.DB.Query(ctx, query, serverId, userId, cursor.CreatedAt, cursor.Id, limit)
	} else {
		query := baseSelect + `
		WHERE sp.server_id = $1 AND sp.author_id = $2
		ORDER BY sp.created_at DESC, sp.id DESC
		LIMIT $3`
		rows, err = repository.DB.Query(ctx, query, serverId, userId, limit)
	}

	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query server posts for me", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	posts := []model.ServerPostResponse{}
	for rows.Next() {
		var resp model.ServerPostResponse
		err = rows.Scan(
			&resp.Id,
			&resp.ServerId,
			&resp.Caption,
			&resp.Author.UserId,
			&resp.CreatedAt,
			&resp.UpdatedAt,
			&resp.ImageUrl,
			&resp.Author.Nickname,
			&resp.Author.Username,
			&resp.Author.AvatarUrl,
			&resp.LikeCount,
			&resp.CommentCount,
			&resp.UserLiked,
			&resp.UserSaved,
		)
		if err != nil {
			util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan server post for me row", zap.Error(err))
			return nil, err
		}

		resp.Author.Status = model.AuthorStatusActive
		resp.IsOwner = true

		if resp.ImageUrl != nil {
			*resp.ImageUrl = fmt.Sprintf("%s/%s", minioFullUrl, *resp.ImageUrl)
		}
		if resp.Author.AvatarUrl != nil {
			*resp.Author.AvatarUrl = fmt.Sprintf("%s/%s", minioFullUrl, *resp.Author.AvatarUrl)
		}

		posts = append(posts, resp)
	}

	return posts, nil
}

// GetServerPostsByAuthor returns a member's posts in a server for viewing by
// another user. Like GetServerPostForMe but the author filter (authorId) and
// the userLiked check (requesterId) are separate params, so userLiked reflects
// the viewer, not the author. IsOwner is true only when the viewer is the author.
func (repository *PostRepository) GetServerPostsByAuthor(ctx context.Context, limit int, serverId, authorId, requesterId string, cursor *model.ServerPostCursor, minioFullUrl string) ([]model.ServerPostResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetServerPostsByAuthor")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("server.id", serverId),
		attribute.String("author.id", authorId),
		attribute.String("user.id", requesterId),
	)

	baseSelect := `
		SELECT sp.id, sp.server_id, sp.caption, sp.author_id,
			sp.created_at, sp.updated_at,
			spi.object_key,
			smp.nickname,
			smp.username,
			pai.object_key,
			(SELECT COUNT(*) FROM server_post_likes WHERE post_id = sp.id) AS like_count,
			(SELECT COUNT(*) FROM server_post_comments WHERE post_id = sp.id) AS comment_count,
			EXISTS (SELECT 1 FROM server_post_likes l WHERE l.post_id = sp.id AND l.user_id = $3) AS user_liked,
			EXISTS (SELECT 1 FROM server_post_saves s WHERE s.post_id = sp.id AND s.user_id = $3) AS user_saved
		FROM server_posts sp
		INNER JOIN server_member_profiles smp ON smp.server_id = sp.server_id AND smp.user_id = sp.author_id
		LEFT JOIN server_post_images spi ON sp.post_image_id = spi.id
		LEFT JOIN profile_avatar_images pai ON smp.avatar_image_id = pai.id`

	var rows pgx.Rows
	if cursor.Id != "" && !cursor.CreatedAt.IsZero() {
		query := baseSelect + `
		WHERE sp.server_id = $1 AND sp.author_id = $2
		AND (sp.created_at < $4 OR (sp.created_at = $4 AND sp.id < $5))
		ORDER BY sp.created_at DESC, sp.id DESC
		LIMIT $6`
		rows, err = repository.DB.Query(ctx, query, serverId, authorId, requesterId, cursor.CreatedAt, cursor.Id, limit)
	} else {
		query := baseSelect + `
		WHERE sp.server_id = $1 AND sp.author_id = $2
		ORDER BY sp.created_at DESC, sp.id DESC
		LIMIT $4`
		rows, err = repository.DB.Query(ctx, query, serverId, authorId, requesterId, limit)
	}

	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query server posts by author", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	posts := []model.ServerPostResponse{}
	for rows.Next() {
		var resp model.ServerPostResponse
		err = rows.Scan(
			&resp.Id,
			&resp.ServerId,
			&resp.Caption,
			&resp.Author.UserId,
			&resp.CreatedAt,
			&resp.UpdatedAt,
			&resp.ImageUrl,
			&resp.Author.Nickname,
			&resp.Author.Username,
			&resp.Author.AvatarUrl,
			&resp.LikeCount,
			&resp.CommentCount,
			&resp.UserLiked,
			&resp.UserSaved,
		)
		if err != nil {
			util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan server post by author row", zap.Error(err))
			return nil, err
		}

		resp.Author.Status = model.AuthorStatusActive
		resp.IsOwner = resp.Author.UserId == requesterId

		if resp.ImageUrl != nil {
			*resp.ImageUrl = fmt.Sprintf("%s/%s", minioFullUrl, *resp.ImageUrl)
		}
		if resp.Author.AvatarUrl != nil {
			*resp.Author.AvatarUrl = fmt.Sprintf("%s/%s", minioFullUrl, *resp.Author.AvatarUrl)
		}

		posts = append(posts, resp)
	}

	return posts, nil
}

func (repository *PostRepository) GetPostServerId(ctx context.Context, postId string) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetPostServerId")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("post.id", postId),
	)

	query := `SELECT server_id FROM server_posts WHERE id = $1 LIMIT 1`

	var serverId string
	err = repository.DB.QueryRow(ctx, query, postId).Scan(&serverId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.NotFoundError{Code: constant.ERR_NOT_FOUND_CODE, Message: "Post not found", Param: "postId"}
			return "", err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get post server_id", zap.Error(err))
		return "", err
	}

	return serverId, nil
}

func (repository *PostRepository) CheckPostLike(ctx context.Context, postId string, userId string) (bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CheckPostLike")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("post.id", postId),
		attribute.String("user.id", userId),
	)

	query := `SELECT EXISTS (SELECT 1 FROM server_post_likes WHERE post_id = $1 AND user_id = $2)`

	var exists bool
	err = repository.DB.QueryRow(ctx, query, postId, userId).Scan(&exists)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check post like", zap.Error(err))
		return false, err
	}

	return exists, nil
}

func (repository *PostRepository) GetPostLikeCount(ctx context.Context, postId string) (int, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetPostLikeCount")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("post.id", postId),
	)

	query := `SELECT COUNT(*) FROM server_post_likes WHERE post_id = $1`

	var count int
	err = repository.DB.QueryRow(ctx, query, postId).Scan(&count)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get post like count", zap.Error(err))
		return 0, err
	}

	return count, nil
}

// CreatePostLikeIdempotent inserts a like idempotently. Returns true when a row was actually
// inserted, false on the ON CONFLICT no-op (a repeated like). The caller notifies only when true,
// so like/unlike/like toggling never spams the post author.
func (repository *PostRepository) CreatePostLikeIdempotent(ctx context.Context, like model.ServerPostLike) (bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CreatePostLikeIdempotent")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("post.id", like.PostId),
		attribute.String("user.id", like.UserId),
	)

	query := `INSERT INTO server_post_likes (id, post_id, user_id, created_at, updated_at, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (post_id, user_id) DO NOTHING`

	var tag pgconn.CommandTag
	tag, err = repository.DB.Exec(ctx, query, like.Id, like.PostId, like.UserId,
		like.CreatedAt, like.UpdatedAt, like.CreatedBy, like.UpdatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create post like", zap.Error(err))
		return false, err
	}

	return tag.RowsAffected() == 1, nil
}

// GetPostAuthorId returns a post's author — the notification recipient for a like or a top-level
// comment.
func (repository *PostRepository) GetPostAuthorId(ctx context.Context, postId string) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetPostAuthorId")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("post.id", postId),
	)

	query := `SELECT author_id FROM server_posts WHERE id = $1 LIMIT 1`
	var authorId string
	err = repository.DB.QueryRow(ctx, query, postId).Scan(&authorId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.NotFoundError{Code: constant.ERR_NOT_FOUND_CODE, Message: "Post not found", Param: "postId"}
			return "", err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get post author_id", zap.Error(err))
		return "", err
	}
	return authorId, nil
}

// GetCommentAuthorId returns a comment's author — the notification recipient for a reply (the
// parent comment's author, not the post owner).
func (repository *PostRepository) GetCommentAuthorId(ctx context.Context, commentId string) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetCommentAuthorId")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("comment.id", commentId),
	)

	query := `SELECT author_id FROM server_post_comments WHERE id = $1 LIMIT 1`
	var authorId string
	err = repository.DB.QueryRow(ctx, query, commentId).Scan(&authorId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.NotFoundError{Code: constant.ERR_NOT_FOUND_CODE, Message: "Comment not found", Param: "commentId"}
			return "", err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get comment author_id", zap.Error(err))
		return "", err
	}
	return authorId, nil
}

func (repository *PostRepository) DeletePostLike(ctx context.Context, postId string, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeletePostLike")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "DELETE"),
		attribute.String("post.id", postId),
		attribute.String("user.id", userId),
	)

	query := `DELETE FROM server_post_likes WHERE post_id = $1 AND user_id = $2`

	_, err = repository.DB.Exec(ctx, query, postId, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete post like", zap.Error(err))
		return err
	}

	return nil
}

func (repository *PostRepository) CheckCommentExists(ctx context.Context, commentId string, postId string) (int, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CheckCommentExists")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("comment.id", commentId),
		attribute.String("post.id", postId),
	)

	query := `SELECT COUNT(*) FROM server_post_comments WHERE id = $1 AND post_id = $2`

	var count int
	err = repository.DB.QueryRow(ctx, query, commentId, postId).Scan(&count)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check comment exists", zap.Error(err))
		return 0, err
	}

	return count, nil
}

func (repository *PostRepository) CreateComment(ctx context.Context, comment model.ServerPostComment) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CreateComment")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("post.id", comment.PostId),
		attribute.String("user.id", comment.AuthorId),
	)

	query := `INSERT INTO server_post_comments (id, post_id, author_id, parent_id, content, created_at, updated_at, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err = repository.DB.Exec(ctx, query, comment.Id, comment.PostId, comment.AuthorId, comment.ParentId,
		comment.Content, comment.CreatedAt, comment.UpdatedAt, comment.CreatedBy, comment.UpdatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create comment", zap.Error(err))
		return err
	}

	return nil
}

func (repository *PostRepository) GetCommentById(ctx context.Context, commentId string, userId string, minioFullUrl string) (model.ServerCommentResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetCommentById")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("comment.id", commentId),
	)

	query := `
		SELECT
			c.id, c.post_id, c.parent_id, c.content, c.author_id,
			c.created_at, c.updated_at,
			smp.nickname AS author_nickname,
			smp.username AS author_username,
			pai.object_key AS author_avatar_key,
			CASE
				WHEN u.deleted_at IS NOT NULL THEN 'user_deleted'
				WHEN sm_author.user_id IS NULL THEN 'user_left'
				ELSE 'active'
			END AS author_status
		FROM server_post_comments c
		INNER JOIN server_posts sp ON c.post_id = sp.id
		INNER JOIN users u ON c.author_id = u.id
		INNER JOIN server_member_profiles smp ON smp.server_id = sp.server_id AND smp.user_id = c.author_id
		LEFT JOIN server_members sm_author ON sm_author.server_id = sp.server_id AND sm_author.user_id = c.author_id
		LEFT JOIN profile_avatar_images pai ON smp.avatar_image_id = pai.id
		WHERE c.id = $1
		LIMIT 1`

	var resp model.ServerCommentResponse
	var authorAvatarKey *string
	var authorNickname, authorUsername, authorStatus string

	err = repository.DB.QueryRow(ctx, query, commentId).Scan(
		&resp.Id, &resp.PostId, &resp.ParentId, &resp.Content, &resp.Author.UserId,
		&resp.CreatedAt, &resp.UpdatedAt,
		&authorNickname, &authorUsername, &authorAvatarKey,
		&authorStatus,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.NotFoundError{Code: constant.ERR_NOT_FOUND_CODE, Message: "Comment not found", Param: "commentId"}
			return resp, err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get comment by id", zap.Error(err))
		return resp, err
	}

	resp.Author.Nickname = authorNickname
	resp.Author.Username = authorUsername
	resp.Author.Status = model.AuthorStatus(authorStatus)
	if authorAvatarKey != nil {
		avatarUrl := fmt.Sprintf("%s/%s", minioFullUrl, *authorAvatarKey)
		resp.Author.AvatarUrl = &avatarUrl
	}
	resp.IsOwner = resp.Author.UserId == userId

	return resp, nil
}

func (repository *PostRepository) GetComments(ctx context.Context, limit int, postId string, userId string, cursor *model.ServerCommentCursor, minioFullUrl string) ([]model.ServerCommentResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetComments")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("post.id", postId),
	)

	query := `
		SELECT
			c.id, c.post_id, c.parent_id, c.content, c.author_id,
			c.created_at, c.updated_at,
			smp.nickname AS author_nickname,
			smp.username AS author_username,
			pai.object_key AS author_avatar_key,
			CASE
				WHEN u.deleted_at IS NOT NULL THEN 'user_deleted'
				WHEN sm_author.user_id IS NULL THEN 'user_left'
				ELSE 'active'
			END AS author_status
		FROM server_post_comments c
		INNER JOIN server_posts sp ON c.post_id = sp.id
		INNER JOIN users u ON c.author_id = u.id
		INNER JOIN server_member_profiles smp ON smp.server_id = sp.server_id AND smp.user_id = c.author_id
		LEFT JOIN server_members sm_author ON sm_author.server_id = sp.server_id AND sm_author.user_id = c.author_id
		LEFT JOIN profile_avatar_images pai ON smp.avatar_image_id = pai.id
		WHERE c.post_id = $1`

	args := []interface{}{postId}
	argIdx := 2

	if cursor != nil {
		query += fmt.Sprintf(" AND (c.created_at, c.id) > ($%d, $%d)", argIdx, argIdx+1)
		args = append(args, cursor.CreatedAt, cursor.Id)
		argIdx += 2
	}

	query += fmt.Sprintf(" ORDER BY c.created_at ASC, c.id ASC LIMIT $%d", argIdx)
	args = append(args, limit)

	var rows pgx.Rows
	rows, err = repository.DB.Query(ctx, query, args...)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query comments", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var comments []model.ServerCommentResponse
	for rows.Next() {
		var resp model.ServerCommentResponse
		var authorAvatarKey *string
		var authorNickname, authorUsername, authorStatus string

		err = rows.Scan(
			&resp.Id, &resp.PostId, &resp.ParentId, &resp.Content, &resp.Author.UserId,
			&resp.CreatedAt, &resp.UpdatedAt,
			&authorNickname, &authorUsername, &authorAvatarKey,
			&authorStatus,
		)
		if err != nil {
			util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan comment row", zap.Error(err))
			return nil, err
		}

		resp.Author.Nickname = authorNickname
		resp.Author.Username = authorUsername
		resp.Author.Status = model.AuthorStatus(authorStatus)
		if authorAvatarKey != nil {
			avatarUrl := fmt.Sprintf("%s/%s", minioFullUrl, *authorAvatarKey)
			resp.Author.AvatarUrl = &avatarUrl
		}
		resp.IsOwner = resp.Author.UserId == userId

		comments = append(comments, resp)
	}

	return comments, nil
}

func (repository *PostRepository) CheckCommentOwnership(ctx context.Context, commentId string, userId string) (int, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CheckCommentOwnership")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("comment.id", commentId),
		attribute.String("user.id", userId),
	)

	query := `SELECT COUNT(*) FROM server_post_comments WHERE id = $1 AND author_id = $2`

	var count int
	err = repository.DB.QueryRow(ctx, query, commentId, userId).Scan(&count)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check comment ownership", zap.Error(err))
		return 0, err
	}

	return count, nil
}

func (repository *PostRepository) DeleteCommentHard(ctx context.Context, commentId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeleteCommentHard")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "DELETE"),
		attribute.String("comment.id", commentId),
	)

	// FK ON DELETE CASCADE handles replies (parent_id chain): all descendants deleted.
	query := `DELETE FROM server_post_comments WHERE id = $1`

	_, err = repository.DB.Exec(ctx, query, commentId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to hard delete comment", zap.Error(err))
		return err
	}

	return nil
}

func (repository *PostRepository) CreatePostSave(ctx context.Context, save model.ServerPostSave) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CreatePostSave")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("post.id", save.PostId),
		attribute.String("user.id", save.UserId),
	)

	query := `INSERT INTO server_post_saves (id, post_id, user_id, created_at, updated_at, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err = repository.DB.Exec(ctx, query, save.Id, save.PostId, save.UserId,
		save.CreatedAt, save.UpdatedAt, save.CreatedBy, save.UpdatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create post save", zap.Error(err))
		return err
	}

	return nil
}

func (repository *PostRepository) CheckPostSave(ctx context.Context, postId string, userId string) (bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CheckPostSave")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("post.id", postId),
		attribute.String("user.id", userId),
	)

	query := `SELECT EXISTS (SELECT 1 FROM server_post_saves WHERE post_id = $1 AND user_id = $2)`

	var exists bool
	err = repository.DB.QueryRow(ctx, query, postId, userId).Scan(&exists)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check post save", zap.Error(err))
		return false, err
	}

	return exists, nil
}

func (repository *PostRepository) DeletePostSave(ctx context.Context, postId string, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeletePostSave")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "DELETE"),
		attribute.String("post.id", postId),
		attribute.String("user.id", userId),
	)

	query := `DELETE FROM server_post_saves WHERE post_id = $1 AND user_id = $2`

	_, err = repository.DB.Exec(ctx, query, postId, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete post save", zap.Error(err))
		return err
	}

	return nil
}

// GetSavedPosts returns the viewer's saved posts within one server, newest-save-first.
// Member-guard is enforced in the usecase before this runs. UserSaved is always true here;
// SavedAt carries server_post_saves.created_at for cursor building.
func (repository *PostRepository) GetSavedPosts(ctx context.Context, limit int, serverId, userId string, cursor *model.SavedPostCursor, minioFullUrl string) ([]model.ServerPostResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetSavedPosts")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("server.id", serverId),
		attribute.String("user.id", userId),
	)

	baseSelect := `
		SELECT sp.id, sp.server_id, sp.caption, sp.author_id,
			sp.created_at, sp.updated_at,
			spi.object_key,
			smp.nickname,
			smp.username,
			pai.object_key,
			CASE
				WHEN u.deleted_at IS NOT NULL THEN 'user_deleted'
				WHEN sm_author.user_id IS NULL THEN 'user_left'
				ELSE 'active'
			END AS author_status,
			(SELECT COUNT(*) FROM server_post_likes WHERE post_id = sp.id) AS like_count,
			(SELECT COUNT(*) FROM server_post_comments WHERE post_id = sp.id) AS comment_count,
			EXISTS (SELECT 1 FROM server_post_likes l WHERE l.post_id = sp.id AND l.user_id = $1) AS user_liked,
			sps.created_at AS saved_at
		FROM server_post_saves sps
		INNER JOIN server_posts sp ON sp.id = sps.post_id
		INNER JOIN users u ON sp.author_id = u.id
		INNER JOIN server_member_profiles smp ON smp.server_id = sp.server_id AND smp.user_id = sp.author_id
		LEFT JOIN server_members sm_author ON sm_author.server_id = sp.server_id AND sm_author.user_id = sp.author_id
		LEFT JOIN server_post_images spi ON sp.post_image_id = spi.id
		LEFT JOIN profile_avatar_images pai ON smp.avatar_image_id = pai.id`

	var rows pgx.Rows
	if cursor.PostId != "" && !cursor.SavedAt.IsZero() {
		query := baseSelect + `
		WHERE sps.user_id = $1 AND sp.server_id = $2
		AND (sps.created_at < $3 OR (sps.created_at = $3 AND sps.post_id < $4))
		ORDER BY sps.created_at DESC, sps.post_id DESC
		LIMIT $5`
		rows, err = repository.DB.Query(ctx, query, userId, serverId, cursor.SavedAt, cursor.PostId, limit)
	} else {
		query := baseSelect + `
		WHERE sps.user_id = $1 AND sp.server_id = $2
		ORDER BY sps.created_at DESC, sps.post_id DESC
		LIMIT $3`
		rows, err = repository.DB.Query(ctx, query, userId, serverId, limit)
	}

	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query saved posts", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	posts := []model.ServerPostResponse{}
	for rows.Next() {
		var resp model.ServerPostResponse
		var authorStatus string
		var savedAt time.Time
		err = rows.Scan(
			&resp.Id,
			&resp.ServerId,
			&resp.Caption,
			&resp.Author.UserId,
			&resp.CreatedAt,
			&resp.UpdatedAt,
			&resp.ImageUrl,
			&resp.Author.Nickname,
			&resp.Author.Username,
			&resp.Author.AvatarUrl,
			&authorStatus,
			&resp.LikeCount,
			&resp.CommentCount,
			&resp.UserLiked,
			&savedAt,
		)
		if err != nil {
			util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan saved post row", zap.Error(err))
			return nil, err
		}

		resp.Author.Status = model.AuthorStatus(authorStatus)
		resp.IsOwner = resp.Author.UserId == userId
		resp.UserSaved = true
		savedAtCopy := savedAt
		resp.SavedAt = &savedAtCopy

		if resp.ImageUrl != nil {
			*resp.ImageUrl = fmt.Sprintf("%s/%s", minioFullUrl, *resp.ImageUrl)
		}
		if resp.Author.AvatarUrl != nil {
			*resp.Author.AvatarUrl = fmt.Sprintf("%s/%s", minioFullUrl, *resp.Author.AvatarUrl)
		}

		posts = append(posts, resp)
	}

	return posts, nil
}

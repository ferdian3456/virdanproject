package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

func (repository *PostRepository) DeletePost(ctx context.Context, postId uuid.UUID) error {
	query := "DELETE FROM server_posts WHERE id = $1"

	_, err := repository.DB.Exec(ctx, query, postId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *PostRepository) GetPostImage(ctx context.Context, tx pgx.Tx, postId uuid.UUID) (uuid.UUID, string, error) {
	query := `
		SELECT sp.post_image_id, spi.object_key
		FROM server_posts sp
		INNER JOIN server_post_images spi ON sp.post_image_id = spi.id
		WHERE sp.id = $1
	`

	var postImageId uuid.UUID
	var objectKey string
	err := tx.QueryRow(ctx, query, postId).Scan(&postImageId, &objectKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, "", nil
		}
		return uuid.Nil, "", err
	}

	return postImageId, objectKey, nil
}

func (repository *PostRepository) DeletePostImage(ctx context.Context, tx pgx.Tx, postImageId uuid.UUID) error {
	query := "DELETE FROM server_post_images WHERE id = $1"

	_, err := tx.Exec(ctx, query, postImageId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *PostRepository) DeletePostObject(ctx context.Context, bucketName string, objectKey string) error {
	err := repository.DBObject.RemoveObject(ctx, bucketName, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (repository *PostRepository) GetServerPosts(ctx context.Context, limit int, serverId uuid.UUID, cursor *model.ServerPostCursor, minioFullUrl string) ([]model.ServerPostResponse, error) {
	var rows pgx.Rows
	var err error

	// Check if cursor is provided (not first page)
	if cursor.Id != uuid.Nil && !cursor.CreateDatetime.IsZero() {
		// Query with cursor for pagination
		queryWithCursor := `
			SELECT sp.author_id, sp.id, spi.object_key, sp.caption, sp.create_datetime, sp.update_datetime,
			       COALESCE(comment_counts.comment_count, 0) as comment_count,
			       COALESCE(like_counts.like_count, 0) as like_count
			FROM server_posts sp
			INNER JOIN server_post_images spi ON sp.post_image_id = spi.id
			LEFT JOIN (
				SELECT post_id, COUNT(*) as comment_count
				FROM server_post_comments
				GROUP BY post_id
			) comment_counts ON sp.id = comment_counts.post_id
			LEFT JOIN (
				SELECT post_id, COUNT(*) as like_count
				FROM server_post_likes
				GROUP BY post_id
			) like_counts ON sp.id = like_counts.post_id
			WHERE sp.server_id = $1
			AND (sp.create_datetime < $2 OR (sp.create_datetime = $2 AND sp.id < $3))
			ORDER BY sp.create_datetime DESC, sp.id DESC
			LIMIT $4
		`
		rows, err = repository.DB.Query(ctx, queryWithCursor, serverId, cursor.CreateDatetime, cursor.Id, limit)
	} else {
		// Query without cursor for first page
		query := `
			SELECT sp.author_id, sp.id, spi.object_key, sp.caption, sp.create_datetime, sp.update_datetime,
			       COALESCE(comment_counts.comment_count, 0) as comment_count,
			       COALESCE(like_counts.like_count, 0) as like_count
			FROM server_posts sp
			INNER JOIN server_post_images spi ON sp.post_image_id = spi.id
			LEFT JOIN (
				SELECT post_id, COUNT(*) as comment_count
				FROM server_post_comments
				GROUP BY post_id
			) comment_counts ON sp.id = comment_counts.post_id
			LEFT JOIN (
				SELECT post_id, COUNT(*) as like_count
				FROM server_post_likes
				GROUP BY post_id
			) like_counts ON sp.id = like_counts.post_id
			WHERE sp.server_id = $1
			ORDER BY sp.create_datetime DESC, sp.id DESC
			LIMIT $2
		`
		rows, err = repository.DB.Query(ctx, query, serverId, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := []model.ServerPostResponse{}

	for rows.Next() {
		var post model.ServerPostResponse
		err := rows.Scan(&post.OwnerId, &post.PostId, &post.PostImageUrl, &post.Caption, &post.CreateDatetime, &post.UpdateDatetime, &post.CommentCount, &post.LikeCount)
		if err != nil {
			return nil, err
		}

		post.PostImageUrl = fmt.Sprintf("%s/%s.webp", minioFullUrl, post.PostImageUrl)

		posts = append(posts, post)
	}

	return posts, nil
}

func (repository *PostRepository) GetPost(ctx context.Context, postId uuid.UUID, minioFullUrl string) (model.ServerPostResponse, error) {
	query := `
		SELECT sp.author_id, sp.id, spi.object_key, sp.caption, sp.create_datetime, sp.update_datetime,
		       COALESCE(comment_counts.comment_count, 0) as comment_count,
		       COALESCE(like_counts.like_count, 0) as like_count
		FROM server_posts sp
		INNER JOIN server_post_images spi ON sp.post_image_id = spi.id
		LEFT JOIN (
			SELECT post_id, COUNT(*) as comment_count
			FROM server_post_comments
			GROUP BY post_id
		) comment_counts ON sp.id = comment_counts.post_id
		LEFT JOIN (
			SELECT post_id, COUNT(*) as like_count
			FROM server_post_likes
			GROUP BY post_id
		) like_counts ON sp.id = like_counts.post_id
		WHERE sp.id = $1
	`

	var post model.ServerPostResponse
	err := repository.DB.QueryRow(ctx, query, postId).Scan(
		&post.OwnerId, &post.PostId, &post.PostImageUrl, &post.Caption,
		&post.CreateDatetime, &post.UpdateDatetime, &post.CommentCount, &post.LikeCount,
	)
	if err != nil {
		return post, err
	}

	post.PostImageUrl = fmt.Sprintf("%s/%s.webp", minioFullUrl, post.PostImageUrl)

	return post, nil
}

func (repository *PostRepository) CheckPostLike(ctx context.Context, postId uuid.UUID, userId uuid.UUID) (int, error) {
	query := "SELECT 1 FROM server_post_likes WHERE post_id = $1 AND user_id = $2"

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

func (repository *PostRepository) CreatePostLike(ctx context.Context, postLike model.ServerPostLikes) error {
	query := "INSERT INTO server_post_likes (post_id, user_id, create_datetime, update_datetime, create_user_id, update_user_id) VALUES ($1, $2, $3, $4, $5, $6)"

	_, err := repository.DB.Exec(ctx, query, postLike.PostId, postLike.UserId, postLike.CreateDatetime, postLike.UpdateDatetime, postLike.CreateUserId, postLike.UpdateUserId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *PostRepository) GetPostServerId(ctx context.Context, postId uuid.UUID) (uuid.UUID, error) {
	query := "SELECT server_id FROM server_posts WHERE id = $1"

	var serverId uuid.UUID
	err := repository.DB.QueryRow(ctx, query, postId).Scan(&serverId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, nil
		}
		return uuid.Nil, err
	}

	return serverId, nil
}

func (repository *PostRepository) CheckPostServerMember(ctx context.Context, postId uuid.UUID, userId uuid.UUID) (int, error) {
	query := `
		SELECT 1
		FROM server_posts sp
		INNER JOIN server_members sm ON sp.server_id = sm.server_id
		WHERE sp.id = $1 AND sm.user_id = $2 AND sm.status = 'ACTIVE'
	`

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

func (repository *PostRepository) DeletePostLike(ctx context.Context, postId uuid.UUID, userId uuid.UUID) error {
	query := "DELETE FROM server_post_likes WHERE post_id = $1 AND user_id = $2"

	_, err := repository.DB.Exec(ctx, query, postId, userId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *PostRepository) CheckCommentExists(ctx context.Context, commentId uuid.UUID, postId uuid.UUID) (int, error) {
	query := "SELECT 1 FROM server_post_comments WHERE id = $1 AND post_id = $2"

	var exists int
	err := repository.DB.QueryRow(ctx, query, commentId, postId).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return exists, nil
		}

		return exists, err
	}

	return exists, nil
}

func (repository *PostRepository) CreateComment(ctx context.Context, comment model.ServerPostComments) error {
	query := "INSERT INTO server_post_comments (id, post_id, author_id, parent_id, content, create_datetime, update_datetime, create_user_id, update_user_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)"

	_, err := repository.DB.Exec(ctx, query, comment.Id, comment.PostId, comment.AuthorId, comment.ParentId, comment.Content, comment.CreateDatetime, comment.UpdateDatetime, comment.CreateUserId, comment.UpdateUserId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *PostRepository) GetComments(ctx context.Context, limit int, postId uuid.UUID, cursor *model.ServerCommentCursor) ([]model.ServerCommentResponse, error) {
	var rows pgx.Rows
	var err error

	// Check if cursor is provided (not first page)
	if cursor.Id != uuid.Nil && !cursor.CreateDatetime.IsZero() {
		// Query with cursor for pagination
		queryWithCursor := `
			SELECT id, author_id, parent_id, content, create_datetime, update_datetime
			FROM server_post_comments
			WHERE post_id = $1
			AND (create_datetime < $2 OR (create_datetime = $2 AND id < $3))
			ORDER BY create_datetime DESC, id DESC
			LIMIT $4
		`
		rows, err = repository.DB.Query(ctx, queryWithCursor, postId, cursor.CreateDatetime, cursor.Id, limit)
	} else {
		// Query without cursor for first page
		query := `
			SELECT id, author_id, parent_id, content, create_datetime, update_datetime
			FROM server_post_comments
			WHERE post_id = $1
			ORDER BY create_datetime DESC, id DESC
			LIMIT $2
		`
		rows, err = repository.DB.Query(ctx, query, postId, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	comments := []model.ServerCommentResponse{}

	for rows.Next() {
		var comment model.ServerCommentResponse
		err := rows.Scan(&comment.Id, &comment.AuthorId, &comment.ParentId, &comment.Content, &comment.CreateDatetime, &comment.UpdateDatetime)
		if err != nil {
			return nil, err
		}

		comments = append(comments, comment)
	}

	return comments, nil
}

func (repository *PostRepository) CheckCommentOwnership(ctx context.Context, commentId uuid.UUID, userId uuid.UUID) (int, error) {
	query := "SELECT 1 FROM server_post_comments WHERE id = $1 AND author_id = $2"

	var exists int
	err := repository.DB.QueryRow(ctx, query, commentId, userId).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return exists, nil
		}

		return exists, err
	}

	return exists, nil
}

func (repository *PostRepository) DeleteComment(ctx context.Context, commentId uuid.UUID) error {
	query := "DELETE FROM server_post_comments WHERE id = $1"

	_, err := repository.DB.Exec(ctx, query, commentId)
	if err != nil {
		return err
	}

	return nil
}

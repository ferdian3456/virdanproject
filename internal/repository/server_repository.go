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

type ServerRepository struct {
	Log      *zap.Logger
	DB       *pgxpool.Pool
	DBCache  *redis.Client
	DBObject *minio.Client
}

func NewServerRepository(zap *zap.Logger, db *pgxpool.Pool, dbCache *redis.Client, minio *minio.Client) *ServerRepository {
	return &ServerRepository{
		Log:      zap,
		DB:       db,
		DBCache:  dbCache,
		DBObject: minio,
	}
}

func (repository *ServerRepository) CreateServer(ctx context.Context, tx pgx.Tx, server model.Server) error {
	query := "INSERT INTO servers (id,owner_id,name,short_name,category_id,avatar_image_id, banner_image_id, description,settings, create_datetime, update_datetime, create_user_id, update_user_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)"

	_, err := tx.Exec(ctx, query, server.Id, server.OwnerId, server.Name, server.ShortName, server.CategoryId, server.AvatarImageId, server.BannerImageId, server.Description, server.Settings, server.CreateDatetime, server.UpdateDatetime, server.CreateUserId, server.UpdateUserId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) CreateServerRole(ctx context.Context, tx pgx.Tx, serverRole model.ServerRole) error {
	query := "INSERT INTO server_roles (id, server_id, name, permissions, create_datetime, update_datetime, create_user_id, update_user_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)"

	_, err := tx.Exec(ctx, query, serverRole.Id, serverRole.ServerId, serverRole.Name, serverRole.Permissions, serverRole.CreateDatetime, serverRole.UpdateDatetime, serverRole.CreateUserId, serverRole.UpdateUserId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) CreateServerMember(ctx context.Context, tx pgx.Tx, serverMember model.ServerMember) error {
	query := "INSERT INTO server_members (id, server_id, user_id, server_role_id, status, joined_datetime, left_datetime, create_datetime, update_datetime, create_user_id, update_user_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)"

	_, err := tx.Exec(ctx, query, serverMember.Id, serverMember.ServerId, serverMember.UserId, serverMember.ServerRoleId, serverMember.Status, serverMember.JoinedDatetime, serverMember.LeftDatetime, serverMember.CreateDatetime, serverMember.UpdateDatetime, serverMember.CreateUserId, serverMember.UpdateUserId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) CheckInviteCodes(ctx context.Context, code string) (int, error) {
	query := "SELECT 1 FROM server_invites WHERE code = $1"

	var exists int
	err := repository.DB.QueryRow(ctx, query, code).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return exists, nil
		}

		return exists, err
	}

	return exists, nil
}

func (repository *ServerRepository) CheckInviteCodesAndRetrieveServerId(ctx context.Context, code string) (uuid.UUID, error) {
	query := "SELECT server_id FROM server_invites WHERE code = $1 AND is_active = true AND used_count < max_uses"

	var serverId uuid.UUID
	err := repository.DB.QueryRow(ctx, query, code).Scan(&serverId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return serverId, nil
		}

		return serverId, err
	}

	return serverId, nil
}

func (repository *ServerRepository) CreateServerInvites(ctx context.Context, serverInvites model.ServerInvites) error {
	query := "INSERT INTO server_invites (id, server_id, code, max_uses, used_count, expires_datetime, is_active, create_user_id, update_user_id, create_datetime, update_datetime) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10,$11)"

	_, err := repository.DB.Exec(ctx, query, serverInvites.Id, serverInvites.ServerId, serverInvites.Code, serverInvites.MaxUses, serverInvites.UsedCount, serverInvites.ExpiresDatetime, serverInvites.IsActive, serverInvites.CreateUserId, serverInvites.UpdateUserId, serverInvites.CreateDatetime, serverInvites.UpdateDatetime)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) GetServerInfoForInvite(ctx context.Context, inviteCode string) (model.ServerInfoForInviteResponse, error) {
	query := `
		SELECT C.username, A.name, A.description, D.object_key,E.object_key  FROM servers A
		INNER JOIN server_invites B ON A.id = B.server_id
		INNER JOIN users C ON C.id = A.owner_id
		LEFT JOIN server_avatar_images D ON A.id = D.server_id
		LEFT JOIN server_banner_images E ON A.id = E.server_id
		WHERE B.code = $1
	`

	server := model.ServerInfoForInviteResponse{}

	err := repository.DB.QueryRow(ctx, query, inviteCode).Scan(&server.OwnerName, &server.ServerName, &server.Description, &server.AvatarImageId, &server.BannerImageId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return server, nil
		}

		return server, err
	}

	return server, nil
}

func (repository *ServerRepository) CheckServerCategories(ctx context.Context, categoryId int) (int, error) {
	query := "SELECT 1 FROM server_categories WHERE id = $1"

	var exists int
	err := repository.DB.QueryRow(ctx, query, categoryId).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return exists, nil
		}

		return exists, err
	}

	return exists, nil
}

func (repository *ServerRepository) CreateServerAvatarImage(ctx context.Context, tx pgx.Tx, serverAvatarImage model.ServerAvatarImage) error {
	query := "INSERT INTO server_avatar_images (id, server_id, bucket, object_key, mime_type, size, create_datetime, update_datetime, create_user_id, update_user_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)"

	_, err := tx.Exec(ctx, query, serverAvatarImage.Id, serverAvatarImage.ServerId, serverAvatarImage.Bucket, serverAvatarImage.ObjectKey, serverAvatarImage.MimeType, 0, serverAvatarImage.CreateDatetime, serverAvatarImage.UpdateDatetime, serverAvatarImage.CreateUserId, serverAvatarImage.UpdateUserId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) CreateServerBannerImage(ctx context.Context, tx pgx.Tx, serverBannerImage model.ServerBannerImage) error {
	query := "INSERT INTO server_banner_images (id, server_id, bucket, object_key, mime_type, size, create_datetime, update_datetime, create_user_id, update_user_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)"

	_, err := tx.Exec(ctx, query, serverBannerImage.Id, serverBannerImage.ServerId, serverBannerImage.Bucket, serverBannerImage.ObjectKey, serverBannerImage.MimeType, serverBannerImage.Size, serverBannerImage.CreateDatetime, serverBannerImage.UpdateDatetime, serverBannerImage.CreateUserId, serverBannerImage.UpdateUserId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) UploadObject(ctx context.Context, bucketName string, imageName string, imageFile *bytes.Reader, imageSize int64) error {
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

func (repository *ServerRepository) GetServerDiscovery(ctx context.Context, limit int, categoryId int, cursor *model.ServerDiscoveryCursor, minioFullUrl string) ([]model.ServerInfoResponse, error) {
	var rows pgx.Rows
	var err error

	// Check if cursor is provided (not first page)
	if cursor.Id != "" && !cursor.CreateDatetime.IsZero() {
		// Query with cursor for pagination
		queryWithCursor := `
		SELECT A.id,A.name,A.short_name,B.name,C.object_key,D.object_key,A.description,A.create_datetime FROM servers A
		LEFT JOIN server_categories B ON A.category_id = B.id
		LEFT JOIN server_avatar_images C ON A.avatar_image_id = C.id
		LEFT JOIN server_banner_images D ON A.banner_image_id = D.id
		WHERE (A.create_datetime < $1 OR (A.create_datetime = $1 AND A.id < $2))
		AND ($3::int IS NULL OR B.id = $3)
		AND (A.settings->>'isPrivate')::boolean = false
		ORDER BY A.create_datetime DESC, A.id DESC
		LIMIT $4
		`
		rows, err = repository.DB.Query(ctx, queryWithCursor, cursor.CreateDatetime, cursor.Id, categoryId, limit)
	} else {
		// Query without cursor for first page
		query := `
		SELECT A.id,A.name,A.short_name,B.name,C.object_key,D.object_key,A.description,A.create_datetime FROM servers A
		LEFT JOIN server_categories B ON A.category_id = B.id
		LEFT JOIN server_avatar_images C ON A.avatar_image_id = C.id
		LEFT JOIN server_banner_images D ON A.banner_image_id = D.id
		WHERE ($1::int IS NULL OR B.id = $1)
		AND (A.settings->>'isPrivate')::boolean = false
		ORDER BY A.create_datetime DESC, A.id DESC
		LIMIT $2
		`
		rows, err = repository.DB.Query(ctx, query, categoryId, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	servers := []model.ServerInfoResponse{}

	for rows.Next() {
		var server model.ServerInfoResponse
		err := rows.Scan(&server.Id, &server.Name, &server.ShortName, &server.CategoryName, &server.AvatarImageUrl, &server.BannerImageUrl, &server.Description, &server.CreateDatetime)
		if err != nil {
			return nil, err
		}

		if server.AvatarImageUrl != nil {
			*server.AvatarImageUrl = fmt.Sprintf("%s/%s.webp", minioFullUrl, *server.AvatarImageUrl)
		}
		if server.BannerImageUrl != nil {
			*server.BannerImageUrl = fmt.Sprintf("%s/%s.webp", minioFullUrl, *server.BannerImageUrl)
		}

		servers = append(servers, server)
	}

	return servers, nil
}

func (repository *ServerRepository) GetUserServer(ctx context.Context, limit int, cursor *model.ServerUserCursor, userId uuid.UUID, minioFullUrl string) ([]model.ServerUserResponse, error) {
	var rows pgx.Rows
	var err error

	// Check if cursor is provided (not first page)
	if cursor.ServerId != "" && !cursor.JoinedDatetime.IsZero() {
		// Query with cursor for pagination
		queryWithCursor := `
		SELECT B.id, B.name, B.short_name, C.object_key, A.joined_datetime FROM server_members A
		INNER JOIN servers B ON A.server_id = B.id
		LEFT JOIN server_avatar_images C ON C.id = B.avatar_image_id
		WHERE (A.joined_datetime < $1 OR (A.joined_datetime = $1 AND A.server_id < $2)) AND A.user_id = $3
		ORDER BY A.joined_datetime DESC, A.server_id DESC
		LIMIT $4
		`
		rows, err = repository.DB.Query(ctx, queryWithCursor, cursor.JoinedDatetime, cursor.ServerId, userId, limit)
	} else {
		// Query without cursor for first page
		query := `
		SELECT B.id, B.name, B.short_name, C.object_key, A.joined_datetime FROM server_members A
		INNER JOIN servers B ON A.server_id = B.id
		LEFT JOIN server_avatar_images C ON C.id = B.avatar_image_id
		WHERE A.user_id = $1
		ORDER BY A.joined_datetime DESC, A.server_id DESC
		LIMIT $2
		`
		rows, err = repository.DB.Query(ctx, query, userId, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	servers := []model.ServerUserResponse{}

	for rows.Next() {
		var server model.ServerUserResponse
		err := rows.Scan(&server.Id, &server.Name, &server.ShortName, &server.AvatarImageUrl, &server.JoinedDatetime)
		if err != nil {
			return nil, err
		}

		if server.AvatarImageUrl != nil {
			*server.AvatarImageUrl = fmt.Sprintf("%s%s.webp", minioFullUrl, *server.AvatarImageUrl)
		}

		servers = append(servers, server)
	}

	return servers, nil
}

func (repository *ServerRepository) CheckServerEligible(ctx context.Context, serverId uuid.UUID) (int, error) {
	query := `
	SELECT 1 FROM servers WHERE id = $1 AND (settings->>'isPrivate')::boolean = false
	`

	var exists int
	err := repository.DB.QueryRow(ctx, query, serverId).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return exists, nil
		}
		return exists, err
	}

	return exists, nil
}

func (repository *ServerRepository) CheckServerMember(ctx context.Context, serverId uuid.UUID, userId uuid.UUID) (int, error) {
	query := `
	SELECT 1 FROM server_members WHERE server_id = $1 AND user_id = $2 AND status = 'ACTIVE'
	`

	var exists int
	repository.Log.Debug("checking", zap.String("serverId", serverId.String()), zap.String("userId", userId.String()))
	err := repository.DB.QueryRow(ctx, query, serverId, userId).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return exists, nil
		}
		return exists, err
	}

	return exists, nil
}

func (repository *ServerRepository) CheckServerOwnership(ctx context.Context, serverId uuid.UUID, userId uuid.UUID) (int, error) {
	query := "SELECT 1 FROM servers WHERE id = $1 AND owner_id = $2"

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

func (repository *ServerRepository) UpdateServerName(ctx context.Context, serverId uuid.UUID, name string, updateUserId uuid.UUID, updateDatetime time.Time) error {
	query := "UPDATE servers SET name = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4"

	_, err := repository.DB.Exec(ctx, query, name, updateDatetime, updateUserId, serverId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) UpdateServerShortName(ctx context.Context, serverId uuid.UUID, shortName string, updateUserId uuid.UUID, updateDatetime time.Time) error {
	query := "UPDATE servers SET short_name = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4"

	_, err := repository.DB.Exec(ctx, query, shortName, updateDatetime, updateUserId, serverId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) UpdateServerCategory(ctx context.Context, serverId uuid.UUID, categoryId *int, updateUserId uuid.UUID, updateDatetime time.Time) error {
	query := "UPDATE servers SET category_id = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4"

	_, err := repository.DB.Exec(ctx, query, categoryId, updateDatetime, updateUserId, serverId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) UpdateServerDescription(ctx context.Context, serverId uuid.UUID, description *string, updateUserId uuid.UUID, updateDatetime time.Time) error {
	query := "UPDATE servers SET description = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4"

	_, err := repository.DB.Exec(ctx, query, description, updateDatetime, updateUserId, serverId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) DeleteServer(ctx context.Context, serverId uuid.UUID) error {
	query := "DELETE FROM servers WHERE id = $1"

	_, err := repository.DB.Exec(ctx, query, serverId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) DeleteServerAvatarImage(ctx context.Context, tx pgx.Tx, serverId uuid.UUID) error {
	query := "DELETE FROM server_avatar_images WHERE server_id = $1"

	_, err := tx.Exec(ctx, query, serverId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) UpdateServerAvatarImage(ctx context.Context, tx pgx.Tx, serverId uuid.UUID, avatarImageId *uuid.UUID, updateUserId uuid.UUID, updateDatetime time.Time) error {
	query := "UPDATE servers SET avatar_image_id = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4"

	_, err := tx.Exec(ctx, query, avatarImageId, updateDatetime, updateUserId, serverId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) GetServerAvatar(ctx context.Context, tx pgx.Tx, serverId uuid.UUID) (string, error) {
	query := "SELECT object_key FROM server_avatar_images WHERE server_id=$1 LIMIT 1"

	var objectKey string
	err := tx.QueryRow(ctx, query, serverId).Scan(&objectKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return objectKey, err
	}

	return objectKey, nil
}

func (repository *ServerRepository) RemoveServerAvatarObject(ctx context.Context, bucketName string, fileName string) error {
	err := repository.DBObject.RemoveObject(ctx, bucketName, fileName, minio.RemoveObjectOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) DeleteServerBannerImage(ctx context.Context, tx pgx.Tx, serverId uuid.UUID) error {
	query := "DELETE FROM server_banner_images WHERE server_id = $1"

	_, err := tx.Exec(ctx, query, serverId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) UpdateServerBannerImage(ctx context.Context, tx pgx.Tx, serverId uuid.UUID, bannerImageId *uuid.UUID, updateUserId uuid.UUID, updateDatetime time.Time) error {
	query := "UPDATE servers SET banner_image_id = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4"

	_, err := tx.Exec(ctx, query, bannerImageId, updateDatetime, updateUserId, serverId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) GetServerBanner(ctx context.Context, tx pgx.Tx, serverId uuid.UUID) (string, error) {
	query := "SELECT object_key FROM server_banner_images WHERE server_id=$1 LIMIT 1"

	var objectKey string
	err := tx.QueryRow(ctx, query, serverId).Scan(&objectKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return objectKey, err
	}

	return objectKey, nil
}

func (repository *ServerRepository) RemoveServerBannerObject(ctx context.Context, bucketName string, fileName string) error {
	err := repository.DBObject.RemoveObject(ctx, bucketName, fileName, minio.RemoveObjectOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) UpdateServerSettings(ctx context.Context, serverId uuid.UUID, settings []byte, updateUserId uuid.UUID, updateDatetime time.Time) error {
	query := "UPDATE servers SET settings = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4"

	_, err := repository.DB.Exec(ctx, query, settings, updateDatetime, updateUserId, serverId)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ServerRepository) GetServerDetail(ctx context.Context, serverId uuid.UUID) (model.ServerUpdateResponse, error) {
	query := `SELECT id,owner_id, name, short_name, category_id, description, settings, create_datetime, update_datetime, create_user_id, update_user_id
			  FROM servers WHERE id = $1`

	var response model.ServerUpdateResponse
	err := repository.DB.QueryRow(ctx, query, serverId).Scan(&response.Id, &response.OwnerId, &response.Name, &response.ShortName, &response.CategoryId, &response.Description, &response.Settings, &response.CreateDatetime, &response.UpdateDatetime, &response.CreateUserId, &response.UpdateUserId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return response, nil
		}
		return response, err
	}

	return response, nil
}

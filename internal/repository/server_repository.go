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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

type ServerRepository struct {
	Log      *zap.Logger
	Config   *koanf.Koanf
	DB       *pgxpool.Pool
	DBCache  *redis.Client
	DBObject *minio.Client
}

func NewServerRepository(zap *zap.Logger, koanf *koanf.Koanf, db *pgxpool.Pool, dbCache *redis.Client, minio *minio.Client) *ServerRepository {
	return &ServerRepository{
		Log:      zap,
		Config:   koanf,
		DB:       db,
		DBCache:  dbCache,
		DBObject: minio,
	}
}

func (repository *ServerRepository) CreateServer(ctx context.Context, tx pgx.Tx, server model.Server) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.CreateServer")
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
		attribute.String("server.id", server.Id),
	)

	query := `INSERT INTO servers
              (id, owner_id, name, short_name, avatar_image_id, banner_image_id, category_id, description, settings, created_at, updated_at, created_by, updated_by)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	_, err = tx.Exec(ctx, query,
		server.Id, server.OwnerId, server.Name, server.ShortName,
		server.AvatarImageId, server.BannerImageId, server.CategoryId,
		server.Description, server.Settings,
		server.CreatedAt, server.UpdatedAt, server.CreatedBy, server.UpdatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create server", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) CreateServerRole(ctx context.Context, tx pgx.Tx, serverRole model.ServerRole) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.CreateServerRole")
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
		attribute.String("server.id", serverRole.ServerId),
	)

	query := `INSERT INTO server_roles (id, server_id, name, permissions, created_at, updated_at, created_by, updated_by)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err = tx.Exec(ctx, query,
		serverRole.Id, serverRole.ServerId, serverRole.Name, serverRole.Permissions,
		serverRole.CreatedAt, serverRole.UpdatedAt, serverRole.CreatedBy, serverRole.UpdatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create server role", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) GetRoleByName(ctx context.Context, serverId, roleName string) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.GetRoleByName")
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
	)

	query := `SELECT id FROM server_roles WHERE server_id = $1 AND name = $2 LIMIT 1`

	var roleId string
	err = repository.DB.QueryRow(ctx, query, serverId, roleName).Scan(&roleId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
			return "", nil
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get role by name", zap.Error(err))
		return "", err
	}

	return roleId, nil
}

func (repository *ServerRepository) CreateServerMember(ctx context.Context, tx pgx.Tx, serverMember model.ServerMember) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.CreateServerMember")
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
		attribute.String("server.id", serverMember.ServerId),
		attribute.String("user.id", serverMember.UserId),
	)

	query := `INSERT INTO server_members (id, server_id, user_id, server_role_id, joined_at, created_at, updated_at, created_by, updated_by)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err = tx.Exec(ctx, query,
		serverMember.Id, serverMember.ServerId, serverMember.UserId, serverMember.ServerRoleId,
		serverMember.JoinedAt,
		serverMember.CreatedAt, serverMember.UpdatedAt, serverMember.CreatedBy, serverMember.UpdatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create server member", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) CheckServerEligibleForJoin(ctx context.Context, serverId string) (model.ServerCheckEligibleInfo, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.CheckServerEligibleForJoin")
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
	)

	query := `SELECT COALESCE((settings->>'isPrivate')::boolean, false) FROM servers WHERE id = $1 LIMIT 1`

	var info model.ServerCheckEligibleInfo
	var isPrivate bool
	err = repository.DB.QueryRow(ctx, query, serverId).Scan(&isPrivate)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
			return info, nil
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check server eligible for join", zap.Error(err))
		return info, err
	}

	info.Exists = true
	info.IsPrivate = isPrivate
	return info, nil
}

func (repository *ServerRepository) CheckServerMember(ctx context.Context, serverId, userId string) (int, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.CheckServerMember")
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

	query := `SELECT COUNT(*) FROM server_members WHERE server_id = $1 AND user_id = $2`

	var count int
	err = repository.DB.QueryRow(ctx, query, serverId, userId).Scan(&count)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check server member", zap.Error(err))
		return 0, err
	}

	return count, nil
}

func (repository *ServerRepository) CheckServerOwnership(ctx context.Context, serverId, userId string) (int, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.CheckServerOwnership")
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

	query := `SELECT COUNT(*) FROM servers WHERE id = $1 AND owner_id = $2`

	var count int
	err = repository.DB.QueryRow(ctx, query, serverId, userId).Scan(&count)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check server ownership", zap.Error(err))
		return 0, err
	}

	return count, nil
}

func (repository *ServerRepository) CheckServerCategories(ctx context.Context, categoryId int) (int, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.CheckServerCategories")
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
		attribute.Int("category.id", categoryId),
	)

	query := `SELECT COUNT(*) FROM server_categories WHERE id = $1`

	var count int
	err = repository.DB.QueryRow(ctx, query, categoryId).Scan(&count)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check server categories", zap.Error(err))
		return 0, err
	}

	return count, nil
}

func (repository *ServerRepository) CheckCategoryActive(ctx context.Context, categoryId int) (bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.CheckCategoryActive")
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
		attribute.Int("category.id", categoryId),
	)

	query := `SELECT COUNT(*) FROM server_categories WHERE id = $1 AND is_active = true`

	var count int
	err = repository.DB.QueryRow(ctx, query, categoryId).Scan(&count)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check category active", zap.Error(err))
		return false, err
	}

	return count > 0, nil
}

func (repository *ServerRepository) ValidateAndConsumeInvite(ctx context.Context, code string) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.ValidateAndConsumeInvite")
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
		attribute.String("invite.code", code),
	)

	query := `UPDATE server_invites
              SET used_count = used_count + 1, updated_at = NOW()
              WHERE code = $1
                AND is_active = true
                AND (expires_at IS NULL OR expires_at > NOW())
                AND used_count < max_uses
              RETURNING server_id`

	var serverId string
	err = repository.DB.QueryRow(ctx, query, code).Scan(&serverId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.BadRequestError{
				Code:    constant.ERR_VALIDATION_CODE,
				Message: "Invite code is invalid, expired, or has reached max uses",
				Param:   "inviteCode",
			}
			return "", err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to validate/consume invite", zap.Error(err))
		return "", err
	}

	return serverId, nil
}

func (repository *ServerRepository) CreateServerInvites(ctx context.Context, invite model.ServerInvite) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.CreateServerInvites")
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
		attribute.String("server.id", invite.ServerId),
	)

	query := `INSERT INTO server_invites (id, server_id, code, max_uses, used_count, expires_at, is_active, created_by, updated_by, created_at, updated_at)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	_, err = repository.DB.Exec(ctx, query,
		invite.Id, invite.ServerId, invite.Code, invite.MaxUses, invite.UsedCount,
		invite.ExpiresAt, invite.IsActive,
		invite.CreatedBy, invite.UpdatedBy, invite.CreatedAt, invite.UpdatedAt)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create server invite", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) GetServerInfoForInvite(ctx context.Context, code, minioFullUrl string) (model.ServerInfoForInviteResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.GetServerInfoForInvite")
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
		attribute.String("invite.code", code),
	)

	query := `
        SELECT
            si.code, si.server_id, s.name AS server_name,
            sai.bucket, sai.object_key,
            smp.nickname AS owner_nickname,
            (SELECT COUNT(*) FROM server_members sm WHERE sm.server_id = s.id) AS member_count,
            si.expires_at
        FROM server_invites si
        INNER JOIN servers s ON si.server_id = s.id
        INNER JOIN server_member_profiles smp ON smp.server_id = s.id AND smp.user_id = s.owner_id
        LEFT JOIN server_avatar_images sai ON s.avatar_image_id = sai.id
        WHERE si.code = $1
          AND si.is_active = true
          AND (si.expires_at IS NULL OR si.expires_at > NOW())
        LIMIT 1
    `

	var resp model.ServerInfoForInviteResponse
	var bucket, objKey *string
	err = repository.DB.QueryRow(ctx, query, code).Scan(
		&resp.Code, &resp.ServerId, &resp.ServerName,
		&bucket, &objKey,
		&resp.OwnerNickname,
		&resp.MemberCount,
		&resp.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.NotFoundError{Code: constant.ERR_NOT_FOUND_CODE, Message: "Invite code not found or expired"}
			return resp, err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get invite info", zap.Error(err))
		return resp, err
	}

	if bucket != nil && objKey != nil {
		url := fmt.Sprintf("%s/%s", minioFullUrl, *objKey)
		resp.ServerAvatarUrl = &url
	}

	return resp, nil
}

func (repository *ServerRepository) CreateServerAvatarImage(ctx context.Context, tx pgx.Tx, image model.ServerAvatarImage) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.CreateServerAvatarImage")
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
		attribute.String("image.id", image.Id),
	)

	query := `INSERT INTO server_avatar_images (id, bucket, object_key, mime_type, size, created_at, updated_at, created_by, updated_by)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err = tx.Exec(ctx, query,
		image.Id, image.Bucket, image.ObjectKey, image.MimeType, image.Size,
		image.CreatedAt, image.UpdatedAt, image.CreatedBy, image.UpdatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create server avatar image", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) GetServerAvatarImageId(ctx context.Context, tx pgx.Tx, serverId string) (*string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.GetServerAvatarImageId")
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
	)

	query := `SELECT avatar_image_id FROM servers WHERE id = $1 LIMIT 1`

	var imageId *string
	err = tx.QueryRow(ctx, query, serverId).Scan(&imageId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
			return nil, nil
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get server avatar image id", zap.Error(err))
		return nil, err
	}

	return imageId, nil
}

func (repository *ServerRepository) UpdateServerAvatarImage(ctx context.Context, tx pgx.Tx, serverId string, avatarImageId *string, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.UpdateServerAvatarImage")
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
		attribute.String("server.id", serverId),
	)

	query := `UPDATE servers SET avatar_image_id = $1, updated_at = $2, updated_by = $3 WHERE id = $4`

	_, err = tx.Exec(ctx, query, avatarImageId, updatedAt, updatedBy, serverId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server avatar image", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) DeleteServerAvatarImage(ctx context.Context, tx pgx.Tx, imageId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.DeleteServerAvatarImage")
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
		attribute.String("image.id", imageId),
	)

	query := `DELETE FROM server_avatar_images WHERE id = $1`

	_, err = tx.Exec(ctx, query, imageId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete server avatar image", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) CreateServerBannerImage(ctx context.Context, tx pgx.Tx, image model.ServerBannerImage) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.CreateServerBannerImage")
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
		attribute.String("image.id", image.Id),
	)

	query := `INSERT INTO server_banner_images (id, bucket, object_key, mime_type, size, created_at, updated_at, created_by, updated_by)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err = tx.Exec(ctx, query,
		image.Id, image.Bucket, image.ObjectKey, image.MimeType, image.Size,
		image.CreatedAt, image.UpdatedAt, image.CreatedBy, image.UpdatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create server banner image", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) GetServerBannerImageId(ctx context.Context, tx pgx.Tx, serverId string) (*string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.GetServerBannerImageId")
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
	)

	query := `SELECT banner_image_id FROM servers WHERE id = $1 LIMIT 1`

	var imageId *string
	err = tx.QueryRow(ctx, query, serverId).Scan(&imageId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
			return nil, nil
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get server banner image id", zap.Error(err))
		return nil, err
	}

	return imageId, nil
}

func (repository *ServerRepository) UpdateServerBannerImage(ctx context.Context, tx pgx.Tx, serverId string, bannerImageId *string, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.UpdateServerBannerImage")
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
		attribute.String("server.id", serverId),
	)

	query := `UPDATE servers SET banner_image_id = $1, updated_at = $2, updated_by = $3 WHERE id = $4`

	_, err = tx.Exec(ctx, query, bannerImageId, updatedAt, updatedBy, serverId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server banner image", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) DeleteServerBannerImage(ctx context.Context, tx pgx.Tx, imageId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.DeleteServerBannerImage")
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
		attribute.String("image.id", imageId),
	)

	query := `DELETE FROM server_banner_images WHERE id = $1`

	_, err = tx.Exec(ctx, query, imageId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete server banner image", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) UploadObject(ctx context.Context, bucketName, imageName string, imageFile *bytes.Reader, imageSize int64) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.UploadObject")
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("minio.bucket", bucketName),
		attribute.String("minio.object", imageName),
	)

	_, err = repository.DBObject.PutObject(ctx, bucketName, imageName, imageFile, imageSize,
		minio.PutObjectOptions{
			ContentType:  "image/webp",
			CacheControl: "public, max-age=31536000, immutable",
		})
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to upload object to storage", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) UpdateServerName(ctx context.Context, serverId, name, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.UpdateServerName")
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
		attribute.String("server.id", serverId),
	)

	query := `UPDATE servers SET name = $1, updated_at = $2, updated_by = $3 WHERE id = $4`

	_, err = repository.DB.Exec(ctx, query, name, updatedAt, updatedBy, serverId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server name", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) UpdateServerShortName(ctx context.Context, serverId, shortName, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.UpdateServerShortName")
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
		attribute.String("server.id", serverId),
	)

	query := `UPDATE servers SET short_name = $1, updated_at = $2, updated_by = $3 WHERE id = $4`

	_, err = repository.DB.Exec(ctx, query, shortName, updatedAt, updatedBy, serverId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server short name", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) UpdateServerCategory(ctx context.Context, serverId string, categoryId int, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.UpdateServerCategory")
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
		attribute.String("server.id", serverId),
	)

	query := `UPDATE servers SET category_id = $1, updated_at = $2, updated_by = $3 WHERE id = $4`

	_, err = repository.DB.Exec(ctx, query, categoryId, updatedAt, updatedBy, serverId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server category", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) UpdateServerDescription(ctx context.Context, serverId string, description *string, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.UpdateServerDescription")
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
		attribute.String("server.id", serverId),
	)

	query := `UPDATE servers SET description = $1, updated_at = $2, updated_by = $3 WHERE id = $4`

	_, err = repository.DB.Exec(ctx, query, description, updatedAt, updatedBy, serverId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server description", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) UpdateServerSettings(ctx context.Context, serverId string, settings []byte, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.UpdateServerSettings")
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
		attribute.String("server.id", serverId),
	)

	query := `UPDATE servers SET settings = $1, updated_at = $2, updated_by = $3 WHERE id = $4`

	_, err = repository.DB.Exec(ctx, query, settings, updatedAt, updatedBy, serverId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server settings", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) DeleteServerHard(ctx context.Context, serverId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.DeleteServerHard")
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
		attribute.String("server.id", serverId),
	)

	query := `DELETE FROM servers WHERE id = $1`

	_, err = repository.DB.Exec(ctx, query, serverId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to hard delete server", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) DeleteServersByOwnerId(ctx context.Context, tx pgx.Tx, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.DeleteServersByOwnerId")
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
		attribute.String("user.id", userId),
	)

	query := `DELETE FROM servers WHERE owner_id = $1`

	_, err = tx.Exec(ctx, query, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to batch delete owned servers", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) DeleteAllServerMembersByUserId(ctx context.Context, tx pgx.Tx, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.DeleteAllServerMembersByUserId")
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
		attribute.String("user.id", userId),
	)

	query := `DELETE FROM server_members WHERE user_id = $1`

	_, err = tx.Exec(ctx, query, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete user memberships", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) DeleteServerMember(ctx context.Context, serverId, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.DeleteServerMember")
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
		attribute.String("server.id", serverId),
		attribute.String("user.id", userId),
	)

	query := `DELETE FROM server_members WHERE server_id = $1 AND user_id = $2`

	_, err = repository.DB.Exec(ctx, query, serverId, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete server member", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ServerRepository) GetServerById(ctx context.Context, serverId, userId, minioFullUrl string) (model.ServerDetailResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.GetServerById")
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

	var resp model.ServerDetailResponse

	query := `
        SELECT
            s.id, s.name, s.short_name, s.category_id, sc.name AS category_name,
            s.description, s.settings,
            s.owner_id, smp.nickname AS owner_nickname,
            sai.object_key AS server_avatar_key,
            sbi.object_key AS server_banner_key,
            (SELECT COUNT(*) FROM server_members sm WHERE sm.server_id = s.id) AS member_count,
            EXISTS (SELECT 1 FROM server_members sm2 WHERE sm2.server_id = s.id AND sm2.user_id = $2) AS is_member,
            s.created_at, s.updated_at
        FROM servers s
        INNER JOIN server_member_profiles smp ON smp.server_id = s.id AND smp.user_id = s.owner_id
        LEFT JOIN server_categories sc ON s.category_id = sc.id
        LEFT JOIN server_avatar_images sai ON s.avatar_image_id = sai.id
        LEFT JOIN server_banner_images sbi ON s.banner_image_id = sbi.id
        WHERE s.id = $1
          AND (
              COALESCE((s.settings->>'isPrivate')::bool, false) = false
              OR EXISTS (SELECT 1 FROM server_members sm3 WHERE sm3.server_id = s.id AND sm3.user_id = $2)
          )
        LIMIT 1
    `

	var description *string
	var avatarKey, bannerKey *string
	err = repository.DB.QueryRow(ctx, query, serverId, userId).Scan(
		&resp.Id, &resp.Name, &resp.ShortName, &resp.CategoryId, &resp.CategoryName,
		&description, &resp.Settings,
		&resp.OwnerId, &resp.OwnerNickname,
		&avatarKey, &bannerKey,
		&resp.MemberCount, &resp.IsMember,
		&resp.CreatedAt, &resp.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.NotFoundError{
				Code:    constant.ERR_NOT_FOUND_CODE,
				Message: "Server not found",
				Param:   "serverId",
			}
			return resp, err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get server by ID", zap.Error(err))
		return resp, err
	}

	resp.Description = description
	if avatarKey != nil {
		url := fmt.Sprintf("%s/%s", minioFullUrl, *avatarKey)
		resp.AvatarUrl = &url
	}
	if bannerKey != nil {
		url := fmt.Sprintf("%s/%s", minioFullUrl, *bannerKey)
		resp.BannerUrl = &url
	}

	return resp, nil
}

func (repository *ServerRepository) GetServerDiscovery(ctx context.Context, userId string, limit int, categoryId *int, cursor *model.ServerDiscoveryCursor, minioFullUrl string) ([]model.ServerInfoResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.GetServerDiscovery")
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
		attribute.String("user.id", userId),
		attribute.Int("query.limit", limit),
	)

	var rows pgx.Rows

	if cursor != nil && cursor.Id != "" && !cursor.CreatedAt.IsZero() {
		query := `
            SELECT A.id, A.name, A.short_name, A.category_id, B.name AS category_name,
                   C.object_key AS avatar_key, D.object_key AS banner_key,
                   A.description,
                   (SELECT COUNT(*) FROM server_members sm WHERE sm.server_id = A.id) AS member_count,
                   EXISTS (SELECT 1 FROM server_members sm WHERE sm.server_id = A.id AND sm.user_id = $5) AS is_member,
                   A.created_at
            FROM servers A
            LEFT JOIN server_categories B ON A.category_id = B.id
            LEFT JOIN server_avatar_images C ON A.avatar_image_id = C.id
            LEFT JOIN server_banner_images D ON A.banner_image_id = D.id
            WHERE (A.created_at < $1 OR (A.created_at = $1 AND A.id < $2))
              AND ($3::int IS NULL OR B.id = $3)
              AND COALESCE((A.settings->>'isPrivate')::boolean, false) = false
            ORDER BY A.created_at DESC, A.id DESC
            LIMIT $4
        `
		rows, err = repository.DB.Query(ctx, query, cursor.CreatedAt, cursor.Id, categoryId, limit, userId)
	} else {
		query := `
            SELECT A.id, A.name, A.short_name, A.category_id, B.name AS category_name,
                   C.object_key AS avatar_key, D.object_key AS banner_key,
                   A.description,
                   (SELECT COUNT(*) FROM server_members sm WHERE sm.server_id = A.id) AS member_count,
                   EXISTS (SELECT 1 FROM server_members sm WHERE sm.server_id = A.id AND sm.user_id = $3) AS is_member,
                   A.created_at
            FROM servers A
            LEFT JOIN server_categories B ON A.category_id = B.id
            LEFT JOIN server_avatar_images C ON A.avatar_image_id = C.id
            LEFT JOIN server_banner_images D ON A.banner_image_id = D.id
            WHERE ($1::int IS NULL OR B.id = $1)
              AND COALESCE((A.settings->>'isPrivate')::boolean, false) = false
            ORDER BY A.created_at DESC, A.id DESC
            LIMIT $2
        `
		rows, err = repository.DB.Query(ctx, query, categoryId, limit, userId)
	}
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query server discovery", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	servers := []model.ServerInfoResponse{}

	for rows.Next() {
		var server model.ServerInfoResponse
		var avatarKey, bannerKey *string
		err = rows.Scan(
			&server.Id, &server.Name, &server.ShortName,
			&server.CategoryId, &server.CategoryName,
			&avatarKey, &bannerKey,
			&server.Description, &server.MemberCount, &server.IsMember,
			&server.CreatedAt,
		)
		if err != nil {
			util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan server discovery row", zap.Error(err))
			return nil, err
		}

		if avatarKey != nil {
			url := fmt.Sprintf("%s/%s", minioFullUrl, *avatarKey)
			server.AvatarUrl = &url
		}
		if bannerKey != nil {
			url := fmt.Sprintf("%s/%s", minioFullUrl, *bannerKey)
			server.BannerUrl = &url
		}

		servers = append(servers, server)
	}

	return servers, nil
}

func (repository *ServerRepository) GetUserServers(ctx context.Context, userId string, limit int, cursor *model.ServerUserCursor, minioFullUrl string) ([]model.ServerUserResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.GetUserServers")
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
		attribute.String("user.id", userId),
	)

	var rows pgx.Rows

	if cursor != nil && cursor.ServerId != "" && !cursor.JoinedAt.IsZero() {
		query := `
            SELECT B.id, B.name, B.short_name, B.category_id, C.name AS category_name,
                   D.object_key AS avatar_key,
                   (SELECT COUNT(*) FROM server_members sm WHERE sm.server_id = B.id) AS member_count,
                   A.joined_at,
                   smp.nickname AS my_nickname,
                   pimg.object_key AS my_avatar_key
            FROM server_members A
            INNER JOIN servers B ON A.server_id = B.id
            LEFT JOIN server_categories C ON B.category_id = C.id
            LEFT JOIN server_avatar_images D ON B.avatar_image_id = D.id
            LEFT JOIN server_member_profiles smp ON smp.server_id = A.server_id AND smp.user_id = A.user_id
            LEFT JOIN profile_avatar_images pimg ON smp.avatar_image_id = pimg.id
            WHERE (A.joined_at < $1 OR (A.joined_at = $1 AND A.server_id < $2))
              AND A.user_id = $3
            ORDER BY A.joined_at DESC, A.server_id DESC
            LIMIT $4
        `
		rows, err = repository.DB.Query(ctx, query, cursor.JoinedAt, cursor.ServerId, userId, limit)
	} else {
		query := `
            SELECT B.id, B.name, B.short_name, B.category_id, C.name AS category_name,
                   D.object_key AS avatar_key,
                   (SELECT COUNT(*) FROM server_members sm WHERE sm.server_id = B.id) AS member_count,
                   A.joined_at,
                   smp.nickname AS my_nickname,
                   pimg.object_key AS my_avatar_key
            FROM server_members A
            INNER JOIN servers B ON A.server_id = B.id
            LEFT JOIN server_categories C ON B.category_id = C.id
            LEFT JOIN server_avatar_images D ON B.avatar_image_id = D.id
            LEFT JOIN server_member_profiles smp ON smp.server_id = A.server_id AND smp.user_id = A.user_id
            LEFT JOIN profile_avatar_images pimg ON smp.avatar_image_id = pimg.id
            WHERE A.user_id = $1
            ORDER BY A.joined_at DESC, A.server_id DESC
            LIMIT $2
        `
		rows, err = repository.DB.Query(ctx, query, userId, limit)
	}
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query user servers", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	servers := []model.ServerUserResponse{}

	for rows.Next() {
		var server model.ServerUserResponse
		var avatarKey, myAvatarKey *string
		err = rows.Scan(
			&server.Id, &server.Name, &server.ShortName,
			&server.CategoryId, &server.CategoryName,
			&avatarKey,
			&server.MemberCount,
			&server.JoinedAt,
			&server.MyNickname,
			&myAvatarKey,
		)
		if err != nil {
			util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan user server row", zap.Error(err))
			return nil, err
		}

		if avatarKey != nil {
			url := fmt.Sprintf("%s/%s", minioFullUrl, *avatarKey)
			server.AvatarUrl = &url
		}
		if myAvatarKey != nil {
			url := fmt.Sprintf("%s/%s", minioFullUrl, *myAvatarKey)
			server.MyAvatarUrl = &url
		}

		servers = append(servers, server)
	}

	return servers, nil
}

func (repository *ServerRepository) GetServerCategories(ctx context.Context, limit, cursorId int) ([]model.ServerCategoryResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.GetServerCategories")
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
	)

	var rows pgx.Rows

	if cursorId > 0 {
		query := `SELECT id, name FROM server_categories WHERE id < $1 AND is_active = true ORDER BY id DESC LIMIT $2`
		rows, err = repository.DB.Query(ctx, query, cursorId, limit)
	} else {
		query := `SELECT id, name FROM server_categories WHERE is_active = true ORDER BY id DESC LIMIT $1`
		rows, err = repository.DB.Query(ctx, query, limit)
	}
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query server categories", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	categories := []model.ServerCategoryResponse{}

	for rows.Next() {
		var category model.ServerCategoryResponse
		err = rows.Scan(&category.Id, &category.CategoryName)
		if err != nil {
			util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan server category row", zap.Error(err))
			return nil, err
		}
		categories = append(categories, category)
	}

	return categories, nil
}

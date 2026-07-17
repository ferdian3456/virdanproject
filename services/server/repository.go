package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ferdian3456/virdanproject/shared"
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

type Repository struct {
	Log      *zap.Logger
	Config   *koanf.Koanf
	DB       *pgxpool.Pool
	DBCache  *redis.Client
	DBObject *minio.Client
}

func NewRepository(log *zap.Logger, config *koanf.Koanf, db *pgxpool.Pool, dbCache *redis.Client, dbObject *minio.Client) *Repository {
	return &Repository{
		Log:      log,
		Config:   config,
		DB:       db,
		DBCache:  dbCache,
		DBObject: dbObject,
	}
}

func (repository *Repository) CreateServer(ctx context.Context, tx pgx.Tx, server Server) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CreateServer")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create server", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) CreateServerRole(ctx context.Context, tx pgx.Tx, serverRole ServerRole) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CreateServerRole")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create server role", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) GetRoleByName(ctx context.Context, serverId, roleName string) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetRoleByName")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get role by name", zap.Error(err))
		return "", err
	}

	return roleId, nil
}

func (repository *Repository) CreateServerMember(ctx context.Context, tx pgx.Tx, serverMember ServerMember) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CreateServerMember")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create server member", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) CheckServerEligibleForJoin(ctx context.Context, serverId string) (ServerCheckEligibleInfo, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CheckServerEligibleForJoin")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("server.id", serverId),
	)

	query := `SELECT COALESCE((settings->>'isPrivate')::boolean, false) FROM servers WHERE id = $1 LIMIT 1`

	var info ServerCheckEligibleInfo
	var isPrivate bool
	err = repository.DB.QueryRow(ctx, query, serverId).Scan(&isPrivate)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
			return info, nil
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check server eligible for join", zap.Error(err))
		return info, err
	}

	info.Exists = true
	info.IsPrivate = isPrivate
	return info, nil
}

func (repository *Repository) CheckServerMember(ctx context.Context, serverId, userId string) (int, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CheckServerMember")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check server member", zap.Error(err))
		return 0, err
	}

	return count, nil
}

func (repository *Repository) CheckServerOwnership(ctx context.Context, serverId, userId string) (int, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CheckServerOwnership")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check server ownership", zap.Error(err))
		return 0, err
	}

	return count, nil
}

func (repository *Repository) CountServersOwnedByUser(ctx context.Context, userId string) (int, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CountServersOwnedByUser")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("user.id", userId),
	)

	query := `SELECT COUNT(*) FROM servers WHERE owner_id = $1`

	var count int
	err = repository.DB.QueryRow(ctx, query, userId).Scan(&count)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to count servers owned by user", zap.Error(err))
		return 0, err
	}

	return count, nil
}

func (repository *Repository) CheckServerCategories(ctx context.Context, categoryId int) (int, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CheckServerCategories")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check server categories", zap.Error(err))
		return 0, err
	}

	return count, nil
}

func (repository *Repository) CheckCategoryActive(ctx context.Context, categoryId int) (bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CheckCategoryActive")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check category active", zap.Error(err))
		return false, err
	}

	return count > 0, nil
}

func (repository *Repository) ValidateAndConsumeInvite(ctx context.Context, code string) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.ValidateAndConsumeInvite")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
			err = &shared.BadRequestError{
				Code:    shared.ERR_VALIDATION_CODE,
				Message: "Invite code is invalid, expired, or has reached max uses",
				Param:   "inviteCode",
			}
			return "", err
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to validate/consume invite", zap.Error(err))
		return "", err
	}

	return serverId, nil
}

func (repository *Repository) CreateServerInvites(ctx context.Context, invite ServerInvite) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CreateServerInvites")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create server invite", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) GetServerInfoForInvite(ctx context.Context, code, minioFullUrl string) (ServerInfoForInviteResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetServerInfoForInvite")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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

	var resp ServerInfoForInviteResponse
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
			err = &shared.NotFoundError{Code: shared.ERR_NOT_FOUND_CODE, Message: "Invite code not found or expired"}
			return resp, err
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get invite info", zap.Error(err))
		return resp, err
	}

	if bucket != nil && objKey != nil {
		url := fmt.Sprintf("%s/%s", minioFullUrl, *objKey)
		resp.ServerAvatarUrl = &url
	}

	return resp, nil
}

func (repository *Repository) CreateServerAvatarImage(ctx context.Context, tx pgx.Tx, image ServerAvatarImage) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CreateServerAvatarImage")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create server avatar image", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) GetServerAvatarImageId(ctx context.Context, tx pgx.Tx, serverId string) (*string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetServerAvatarImageId")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get server avatar image id", zap.Error(err))
		return nil, err
	}

	return imageId, nil
}

func (repository *Repository) UpdateServerAvatarImage(ctx context.Context, tx pgx.Tx, serverId string, avatarImageId *string, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdateServerAvatarImage")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server avatar image", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) DeleteServerAvatarImage(ctx context.Context, tx pgx.Tx, imageId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeleteServerAvatarImage")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete server avatar image", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) CreateServerBannerImage(ctx context.Context, tx pgx.Tx, image ServerBannerImage) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CreateServerBannerImage")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create server banner image", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) GetServerBannerImageId(ctx context.Context, tx pgx.Tx, serverId string) (*string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetServerBannerImageId")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get server banner image id", zap.Error(err))
		return nil, err
	}

	return imageId, nil
}

func (repository *Repository) UpdateServerBannerImage(ctx context.Context, tx pgx.Tx, serverId string, bannerImageId *string, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdateServerBannerImage")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server banner image", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) DeleteServerBannerImage(ctx context.Context, tx pgx.Tx, imageId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeleteServerBannerImage")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete server banner image", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) UploadObject(ctx context.Context, bucketName, imageName string, imageFile *bytes.Reader, imageSize int64) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UploadObject")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to upload object to storage", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) UpdateServerName(ctx context.Context, serverId, name, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdateServerName")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server name", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) UpdateServerShortName(ctx context.Context, serverId, shortName, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdateServerShortName")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server short name", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) UpdateServerCategory(ctx context.Context, serverId string, categoryId int, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdateServerCategory")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server category", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) UpdateServerDescription(ctx context.Context, serverId string, description *string, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdateServerDescription")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server description", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) UpdateServerSettings(ctx context.Context, serverId string, settings []byte, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdateServerSettings")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server settings", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) DeleteServerHard(ctx context.Context, serverId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeleteServerHard")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to hard delete server", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) DeleteServersByOwnerId(ctx context.Context, tx pgx.Tx, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeleteServersByOwnerId")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to batch delete owned servers", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) DeleteAllServerMembersByUserId(ctx context.Context, tx pgx.Tx, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeleteAllServerMembersByUserId")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete user memberships", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) DeleteServerMember(ctx context.Context, serverId, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeleteServerMember")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete server member", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) GetServerById(ctx context.Context, serverId, userId, minioFullUrl string) (ServerDetailResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetServerById")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("server.id", serverId),
		attribute.String("user.id", userId),
	)

	var resp ServerDetailResponse

	query := `
        SELECT
            s.id, s.name, s.short_name, s.category_id, sc.name AS category_name,
            s.description, s.settings,
            s.owner_id, smp.nickname AS owner_nickname,
            sai.object_key AS server_avatar_key,
            sbi.object_key AS server_banner_key,
            (SELECT COUNT(*) FROM server_members sm WHERE sm.server_id = s.id) AS member_count,
            EXISTS (SELECT 1 FROM server_members sm2 WHERE sm2.server_id = s.id AND sm2.user_id = $2) AS is_member,
            (SELECT MAX(spo.plus_expires_at) FROM server_plus_orders spo
                WHERE spo.server_id = s.id AND spo.status = 'PAID' AND spo.plus_expires_at > $3) AS plus_expires_at,
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
	var plusExpiresAt *time.Time
	now := time.Now().UTC()
	err = repository.DB.QueryRow(ctx, query, serverId, userId, now).Scan(
		&resp.Id, &resp.Name, &resp.ShortName, &resp.CategoryId, &resp.CategoryName,
		&description, &resp.Settings,
		&resp.OwnerId, &resp.OwnerNickname,
		&avatarKey, &bannerKey,
		&resp.MemberCount, &resp.IsMember,
		&plusExpiresAt,
		&resp.CreatedAt, &resp.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &shared.NotFoundError{
				Code:    shared.ERR_NOT_FOUND_CODE,
				Message: "Server not found",
				Param:   "serverId",
			}
			return resp, err
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get server by ID", zap.Error(err))
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
	if plusExpiresAt != nil {
		resp.PlusActive = true
		resp.PlusExpiresAt = plusExpiresAt
	}

	return resp, nil
}

func (repository *Repository) GetServerDiscovery(ctx context.Context, userId string, limit int, categoryId *int, cursor *ServerDiscoveryCursor, minioFullUrl string) ([]ServerInfoResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetServerDiscovery")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query server discovery", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	servers := []ServerInfoResponse{}

	for rows.Next() {
		var srv ServerInfoResponse
		var avatarKey, bannerKey *string
		err = rows.Scan(
			&srv.Id, &srv.Name, &srv.ShortName,
			&srv.CategoryId, &srv.CategoryName,
			&avatarKey, &bannerKey,
			&srv.Description, &srv.MemberCount, &srv.IsMember,
			&srv.CreatedAt,
		)
		if err != nil {
			shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan server discovery row", zap.Error(err))
			return nil, err
		}

		if avatarKey != nil {
			url := fmt.Sprintf("%s/%s", minioFullUrl, *avatarKey)
			srv.AvatarUrl = &url
		}
		if bannerKey != nil {
			url := fmt.Sprintf("%s/%s", minioFullUrl, *bannerKey)
			srv.BannerUrl = &url
		}

		servers = append(servers, srv)
	}

	return servers, nil
}

func (repository *Repository) GetUserServers(ctx context.Context, userId string, limit int, cursor *ServerUserCursor, minioFullUrl string) ([]ServerUserResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetUserServers")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query user servers", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	servers := []ServerUserResponse{}

	for rows.Next() {
		var srv ServerUserResponse
		var avatarKey, myAvatarKey *string
		err = rows.Scan(
			&srv.Id, &srv.Name, &srv.ShortName,
			&srv.CategoryId, &srv.CategoryName,
			&avatarKey,
			&srv.MemberCount,
			&srv.JoinedAt,
			&srv.MyNickname,
			&myAvatarKey,
		)
		if err != nil {
			shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan user server row", zap.Error(err))
			return nil, err
		}

		if avatarKey != nil {
			url := fmt.Sprintf("%s/%s", minioFullUrl, *avatarKey)
			srv.AvatarUrl = &url
		}
		if myAvatarKey != nil {
			url := fmt.Sprintf("%s/%s", minioFullUrl, *myAvatarKey)
			srv.MyAvatarUrl = &url
		}

		servers = append(servers, srv)
	}

	return servers, nil
}

func (repository *Repository) GetServerCategories(ctx context.Context, limit, cursorId int) ([]ServerCategoryResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetServerCategories")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
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
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query server categories", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	categories := []ServerCategoryResponse{}

	for rows.Next() {
		var category ServerCategoryResponse
		err = rows.Scan(&category.Id, &category.CategoryName)
		if err != nil {
			shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan server category row", zap.Error(err))
			return nil, err
		}
		categories = append(categories, category)
	}

	return categories, nil
}

func (repository *Repository) GetMemberRoleName(ctx context.Context, serverId, userId string) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetMemberRoleName")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("server.id", serverId),
		attribute.String("user.id", userId),
	)

	query := `SELECT sr.name
	          FROM server_members sm
	          JOIN server_roles sr ON sr.id = sm.server_role_id
	          WHERE sm.server_id = $1 AND sm.user_id = $2
	          LIMIT 1`

	var roleName string
	err = repository.DB.QueryRow(ctx, query, serverId, userId).Scan(&roleName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
			return "", nil
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get member role name", zap.Error(err))
		return "", err
	}

	return roleName, nil
}

func (repository *Repository) CountServerMembers(ctx context.Context, serverId string) (int, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CountServerMembers")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("server.id", serverId),
	)

	query := `SELECT COUNT(*) FROM server_members WHERE server_id = $1`

	var count int
	err = repository.DB.QueryRow(ctx, query, serverId).Scan(&count)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to count server members", zap.Error(err))
		return 0, err
	}

	return count, nil
}

func (repository *Repository) UpdateMemberRole(ctx context.Context, serverId, userId, roleId, actorId string, now time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdateMemberRole")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("server.id", serverId),
		attribute.String("user.id", userId),
	)

	query := `UPDATE server_members
	          SET server_role_id = $1, updated_at = $2, updated_by = $3
	          WHERE server_id = $4 AND user_id = $5`

	_, err = repository.DB.Exec(ctx, query, roleId, now, actorId, serverId, userId)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update member role", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) TransferServerOwnership(ctx context.Context, serverId, oldOwnerId, newOwnerId, ownerRoleId, adminRoleId, actorId string, now time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.TransferServerOwnership")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("server.id", serverId),
	)

	tx, err := repository.DB.Begin(ctx)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to begin tx for transfer ownership", zap.Error(err))
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx,
		`UPDATE servers SET owner_id = $1, updated_at = $2, updated_by = $3 WHERE id = $4`,
		newOwnerId, now, actorId, serverId)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server owner_id", zap.Error(err))
		return err
	}

	_, err = tx.Exec(ctx,
		`UPDATE server_members SET server_role_id = $1, updated_at = $2, updated_by = $3 WHERE server_id = $4 AND user_id = $5`,
		ownerRoleId, now, actorId, serverId, newOwnerId)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to promote new owner", zap.Error(err))
		return err
	}

	_, err = tx.Exec(ctx,
		`UPDATE server_members SET server_role_id = $1, updated_at = $2, updated_by = $3 WHERE server_id = $4 AND user_id = $5`,
		adminRoleId, now, actorId, serverId, oldOwnerId)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to demote old owner", zap.Error(err))
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to commit transfer ownership", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) GetServerMembers(ctx context.Context, serverId string, limit int, cursor *ServerMemberCursor, minioFullUrl string) ([]ServerMemberItem, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetServerMembers")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("server.id", serverId),
	)

	query := `SELECT sm.user_id, sr.name, smp.nickname, smp.username, pai.object_key, sm.joined_at
	          FROM server_members sm
	          JOIN server_roles sr ON sr.id = sm.server_role_id
	          JOIN server_member_profiles smp ON smp.server_id = sm.server_id AND smp.user_id = sm.user_id
	          LEFT JOIN profile_avatar_images pai ON pai.id = smp.avatar_image_id
	          WHERE sm.server_id = $1
	            AND ($2::timestamptz IS NULL OR (sm.joined_at, sm.user_id) > ($2, $3))
	          ORDER BY sm.joined_at ASC, sm.user_id ASC
	          LIMIT $4`

	var cursorJoinedAt *time.Time
	var cursorUserId *string
	if cursor != nil {
		cursorJoinedAt = &cursor.JoinedAt
		cursorUserId = &cursor.UserId
	}

	rows, err := repository.DB.Query(ctx, query, serverId, cursorJoinedAt, cursorUserId, limit)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query server members", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	members := []ServerMemberItem{}
	for rows.Next() {
		var item ServerMemberItem
		var objectKey *string
		err = rows.Scan(&item.UserId, &item.Role, &item.Nickname, &item.Username, &objectKey, &item.JoinedAt)
		if err != nil {
			shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan server member", zap.Error(err))
			return nil, err
		}
		if objectKey != nil {
			url := fmt.Sprintf("%s/%s", minioFullUrl, *objectKey)
			item.AvatarUrl = &url
		}
		members = append(members, item)
	}

	if err = rows.Err(); err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Rows error on server members", zap.Error(err))
		return nil, err
	}

	return members, nil
}

func (repository *Repository) CreateServerMemberProfile(ctx context.Context, tx pgx.Tx, profile ServerMemberProfile) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CreateServerMemberProfile")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("server.id", profile.ServerId),
		attribute.String("user.id", profile.UserId),
		attribute.String("profile.nickname", profile.Nickname),
	)

	query := `INSERT INTO server_member_profiles
              (id, server_id, user_id, nickname, username, bio, avatar_image_id, created_at, updated_at, created_by, updated_by)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	_, err = tx.Exec(ctx, query,
		profile.Id, profile.ServerId, profile.UserId, profile.Nickname, profile.Username,
		profile.Bio, profile.AvatarImageId,
		profile.CreatedAt, profile.UpdatedAt, profile.CreatedBy, profile.UpdatedBy)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			switch pgErr.ConstraintName {
			case "idx_server_member_profiles_uk_02":
				err = &shared.ConflictError{
					Code:    shared.ERR_CONFLICT_CODE,
					Message: "Nickname is already taken in this server",
					Param:   "nickname",
				}
			case "idx_server_member_profiles_uk_03":
				err = &shared.ConflictError{
					Code:    shared.ERR_CONFLICT_CODE,
					Message: "Username is already taken in this server",
					Param:   "username",
				}
			case "idx_server_member_profiles_uk_01":
				err = &shared.ConflictError{
					Code:    shared.ERR_CONFLICT_CODE,
					Message: "You already have a profile in this server",
					Param:   "serverId",
				}
			default:
				err = &shared.ConflictError{
					Code:    shared.ERR_CONFLICT_CODE,
					Message: "Conflict creating profile",
					Param:   "",
				}
			}
			return err
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create server_member_profile", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) UpdateServerProfileFull(ctx context.Context, tx pgx.Tx, profileId, nickname, username string, bio *string, avatarImageId *string, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdateServerProfileFull")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("profile.id", profileId),
	)

	query := `UPDATE server_member_profiles
              SET nickname = $1, username = $2, bio = $3, avatar_image_id = $4, updated_at = $5, updated_by = $6
              WHERE id = $7`

	_, err = tx.Exec(ctx, query, nickname, username, bio, avatarImageId, updatedAt, updatedBy, profileId)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			switch pgErr.ConstraintName {
			case "idx_server_member_profiles_uk_02":
				err = &shared.ConflictError{
					Code:    shared.ERR_CONFLICT_CODE,
					Message: "Nickname is already taken in this server",
					Param:   "nickname",
				}
				return err
			case "idx_server_member_profiles_uk_03":
				err = &shared.ConflictError{
					Code:    shared.ERR_CONFLICT_CODE,
					Message: "Username is already taken in this server",
					Param:   "username",
				}
				return err
			}
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server_member_profile", zap.Error(err))
		return err
	}
	return nil
}

func (repository *Repository) UpdateServerProfileNickBioTx(ctx context.Context, tx pgx.Tx, profileId, nickname, username string, bio *string, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdateServerProfileNickBioTx")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("profile.id", profileId),
	)

	query := `UPDATE server_member_profiles
              SET nickname = $1, username = $2, bio = $3, updated_at = $4, updated_by = $5
              WHERE id = $6`

	_, err = tx.Exec(ctx, query, nickname, username, bio, updatedAt, updatedBy, profileId)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			switch pgErr.ConstraintName {
			case "idx_server_member_profiles_uk_02":
				err = &shared.ConflictError{
					Code:    shared.ERR_CONFLICT_CODE,
					Message: "Nickname is already taken in this server",
					Param:   "nickname",
				}
				return err
			case "idx_server_member_profiles_uk_03":
				err = &shared.ConflictError{
					Code:    shared.ERR_CONFLICT_CODE,
					Message: "Username is already taken in this server",
					Param:   "username",
				}
				return err
			}
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server_member_profile (tx)", zap.Error(err))
		return err
	}
	return nil
}

func (repository *Repository) UpdateServerProfileNickBio(ctx context.Context, profileId, nickname, username string, bio *string, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdateServerProfileNickBio")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("profile.id", profileId),
	)

	query := `UPDATE server_member_profiles
              SET nickname = $1, username = $2, bio = $3, updated_at = $4, updated_by = $5
              WHERE id = $6`

	_, err = repository.DB.Exec(ctx, query, nickname, username, bio, updatedAt, updatedBy, profileId)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			switch pgErr.ConstraintName {
			case "idx_server_member_profiles_uk_02":
				err = &shared.ConflictError{
					Code:    shared.ERR_CONFLICT_CODE,
					Message: "Nickname is already taken in this server",
					Param:   "nickname",
				}
				return err
			case "idx_server_member_profiles_uk_03":
				err = &shared.ConflictError{
					Code:    shared.ERR_CONFLICT_CODE,
					Message: "Username is already taken in this server",
					Param:   "username",
				}
				return err
			}
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server_member_profile", zap.Error(err))
		return err
	}
	return nil
}

func (repository *Repository) TryGetServerMemberProfileId(ctx context.Context, tx pgx.Tx, serverId, userId string) (string, bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.TryGetServerMemberProfileId")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("server.id", serverId),
		attribute.String("user.id", userId),
	)

	query := `SELECT id FROM server_member_profiles WHERE server_id = $1 AND user_id = $2 LIMIT 1`
	var id string
	err = tx.QueryRow(ctx, query, serverId, userId).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
			return "", false, nil
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query server_member_profile id", zap.Error(err))
		return "", false, err
	}
	return id, true, nil
}

func (repository *Repository) TryGetServerMemberProfileIdNoTx(ctx context.Context, serverId, userId string) (string, bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.TryGetServerMemberProfileIdNoTx")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("server.id", serverId),
		attribute.String("user.id", userId),
	)

	query := `SELECT id FROM server_member_profiles WHERE server_id = $1 AND user_id = $2 LIMIT 1`
	var id string
	err = repository.DB.QueryRow(ctx, query, serverId, userId).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
			return "", false, nil
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query profile id", zap.Error(err))
		return "", false, err
	}
	return id, true, nil
}

func (repository *Repository) GetProfileId(ctx context.Context, tx pgx.Tx, serverId, userId string) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetProfileId")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("server.id", serverId),
		attribute.String("user.id", userId),
	)

	query := `SELECT id FROM server_member_profiles WHERE server_id = $1 AND user_id = $2 LIMIT 1`
	var profileId string
	err = tx.QueryRow(ctx, query, serverId, userId).Scan(&profileId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &shared.NotFoundError{
				Code:    shared.ERR_NOT_FOUND_CODE,
				Message: "Profile not found in this server",
				Param:   "serverId",
			}
			return "", err
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get profile id", zap.Error(err))
		return "", err
	}
	return profileId, nil
}

func (repository *Repository) GetServerMemberProfile(ctx context.Context, serverId, userId, minioFullUrl string) (ServerMemberProfileResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetServerMemberProfile")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("server.id", serverId),
		attribute.String("user.id", userId),
	)

	var resp ServerMemberProfileResponse
	query := `
        SELECT smp.id, smp.server_id, smp.nickname, smp.username, smp.bio,
               smp.avatar_image_id, pai.object_key,
               smp.created_at, smp.updated_at
        FROM server_member_profiles smp
        LEFT JOIN profile_avatar_images pai ON smp.avatar_image_id = pai.id
        WHERE smp.server_id = $1 AND smp.user_id = $2
        LIMIT 1
    `

	err = repository.DB.QueryRow(ctx, query, serverId, userId).Scan(
		&resp.ProfileId, &resp.ServerId, &resp.Nickname, &resp.Username, &resp.Bio,
		&resp.AvatarImageId, &resp.AvatarUrl,
		&resp.CreatedAt, &resp.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &shared.NotFoundError{
				Code:    shared.ERR_NOT_FOUND_CODE,
				Message: "Profile not found in this server",
				Param:   "serverId",
			}
			return resp, err
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get server member profile", zap.Error(err))
		return resp, err
	}

	if resp.AvatarUrl != nil {
		*resp.AvatarUrl = fmt.Sprintf("%s/%s", minioFullUrl, *resp.AvatarUrl)
	}

	return resp, nil
}

func (repository *Repository) CreateProfileAvatarImage(ctx context.Context, tx pgx.Tx, image shared.ProfileAvatarImage) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CreateProfileAvatarImage")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("profile.avatar_image_id", image.Id),
	)

	query := `INSERT INTO profile_avatar_images
              (id, bucket, object_key, mime_type, size, created_at, updated_at, created_by, updated_by)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err = tx.Exec(ctx, query,
		image.Id, image.Bucket, image.ObjectKey, image.MimeType, image.Size,
		image.CreatedAt, image.UpdatedAt, image.CreatedBy, image.UpdatedBy)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create profile_avatar_image", zap.Error(err))
		return err
	}
	return nil
}

func (repository *Repository) UpdateProfileAvatarImageId(ctx context.Context, tx pgx.Tx, profileId string, avatarImageId *string, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdateProfileAvatarImageId")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("profile.id", profileId),
	)

	query := `UPDATE server_member_profiles
              SET avatar_image_id = $1, updated_at = $2, updated_by = $3
              WHERE id = $4`

	_, err = tx.Exec(ctx, query, avatarImageId, updatedAt, updatedBy, profileId)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update profile avatar_image_id", zap.Error(err))
		return err
	}
	return nil
}

func (repository *Repository) GetProfileHistory(ctx context.Context, userId, minioFullUrl string) ([]GetProfileHistoryResponseItem, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetProfileHistory")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("user.id", userId),
	)

	query := `
        SELECT
            smp.id, smp.server_id, s.name,
            smp.nickname, smp.username, smp.bio,
            smp.avatar_image_id, pai.object_key,
            EXISTS (
                SELECT 1 FROM server_members sm
                WHERE sm.server_id = smp.server_id AND sm.user_id = smp.user_id
            ) AS is_still_member,
            smp.created_at, smp.updated_at
        FROM server_member_profiles smp
        INNER JOIN servers s ON smp.server_id = s.id
        LEFT JOIN profile_avatar_images pai ON smp.avatar_image_id = pai.id
        WHERE smp.user_id = $1
        ORDER BY smp.created_at DESC
    `

	var rows pgx.Rows
	rows, err = repository.DB.Query(ctx, query, userId)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query profile history", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var items []GetProfileHistoryResponseItem
	for rows.Next() {
		var item GetProfileHistoryResponseItem
		err = rows.Scan(
			&item.ProfileId, &item.ServerId, &item.ServerName,
			&item.Nickname, &item.Username, &item.Bio,
			&item.AvatarImageId, &item.AvatarUrl,
			&item.IsStillMember,
			&item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan profile history row", zap.Error(err))
			return nil, err
		}
		if item.AvatarUrl != nil {
			*item.AvatarUrl = fmt.Sprintf("%s/%s", minioFullUrl, *item.AvatarUrl)
		}
		items = append(items, item)
	}

	return items, nil
}

func (repository *Repository) CheckProfileAvatarImageOwnership(ctx context.Context, userId, avatarImageId string) (bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CheckProfileAvatarImageOwnership")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("user.id", userId),
		attribute.String("profile.avatar_image_id", avatarImageId),
	)

	query := `SELECT EXISTS (
        SELECT 1 FROM server_member_profiles
        WHERE user_id = $1 AND avatar_image_id = $2
    )`
	var exists bool
	err = repository.DB.QueryRow(ctx, query, userId, avatarImageId).Scan(&exists)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check avatar ownership", zap.Error(err))
		return false, err
	}
	return exists, nil
}

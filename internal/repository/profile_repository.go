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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

type ProfileRepository struct {
	DB       *pgxpool.Pool
	DBObject *minio.Client
	Log      *zap.Logger
	Config   *koanf.Koanf
}

func NewProfileRepository(
	log *zap.Logger,
	config *koanf.Koanf,
	db *pgxpool.Pool,
	minioClient *minio.Client,
) *ProfileRepository {
	return &ProfileRepository{
		DB:       db,
		DBObject: minioClient,
		Log:      log,
		Config:   config,
	}
}

func (repository *ProfileRepository) CreateServerMemberProfile(ctx context.Context, tx pgx.Tx, profile model.ServerMemberProfile) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.CreateServerMemberProfile")
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
		attribute.String("server.id", profile.ServerId),
		attribute.String("user.id", profile.UserId),
		attribute.String("profile.nickname", profile.Nickname),
	)

	query := `INSERT INTO server_member_profiles
              (id, server_id, user_id, nickname, bio, avatar_image_id, created_at, updated_at, created_by, updated_by)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err = tx.Exec(ctx, query,
		profile.Id, profile.ServerId, profile.UserId, profile.Nickname,
		profile.Bio, profile.AvatarImageId,
		profile.CreatedAt, profile.UpdatedAt, profile.CreatedBy, profile.UpdatedBy)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			switch pgErr.ConstraintName {
			case "idx_server_member_profiles_uk_02":
				err = &model.ConflictError{
					Code:    constant.ERR_CONFLICT_CODE,
					Message: "Nickname is already taken in this server",
					Param:   "nickname",
				}
			case "idx_server_member_profiles_uk_01":
				err = &model.ConflictError{
					Code:    constant.ERR_CONFLICT_CODE,
					Message: "You already have a profile in this server",
					Param:   "serverId",
				}
			default:
				err = &model.ConflictError{
					Code:    constant.ERR_CONFLICT_CODE,
					Message: "Conflict creating profile",
					Param:   "",
				}
			}
			return err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create server_member_profile", zap.Error(err))
		return err
	}

	return nil
}

func (repository *ProfileRepository) UpdateServerProfileFull(ctx context.Context, tx pgx.Tx, profileId, nickname string, bio *string, avatarImageId *string, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.UpdateServerProfileFull")
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
		attribute.String("profile.id", profileId),
	)

	query := `UPDATE server_member_profiles
              SET nickname = $1, bio = $2, avatar_image_id = $3, updated_at = $4, updated_by = $5
              WHERE id = $6`

	_, err = tx.Exec(ctx, query, nickname, bio, avatarImageId, updatedAt, updatedBy, profileId)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if pgErr.ConstraintName == "idx_server_member_profiles_uk_02" {
				err = &model.ConflictError{
					Code:    constant.ERR_CONFLICT_CODE,
					Message: "Nickname is already taken in this server",
					Param:   "nickname",
				}
				return err
			}
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server_member_profile", zap.Error(err))
		return err
	}
	return nil
}

func (repository *ProfileRepository) UpdateServerProfileNickBio(ctx context.Context, profileId, nickname string, bio *string, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.UpdateServerProfileNickBio")
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
		attribute.String("profile.id", profileId),
	)

	query := `UPDATE server_member_profiles
              SET nickname = $1, bio = $2, updated_at = $3, updated_by = $4
              WHERE id = $5`

	_, err = repository.DB.Exec(ctx, query, nickname, bio, updatedAt, updatedBy, profileId)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if pgErr.ConstraintName == "idx_server_member_profiles_uk_02" {
				err = &model.ConflictError{
					Code:    constant.ERR_CONFLICT_CODE,
					Message: "Nickname is already taken in this server",
					Param:   "nickname",
				}
				return err
			}
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update server_member_profile", zap.Error(err))
		return err
	}
	return nil
}

func (repository *ProfileRepository) TryGetServerMemberProfileId(ctx context.Context, tx pgx.Tx, serverId, userId string) (string, bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.TryGetServerMemberProfileId")
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

	query := `SELECT id FROM server_member_profiles WHERE server_id = $1 AND user_id = $2 LIMIT 1`
	var id string
	err = tx.QueryRow(ctx, query, serverId, userId).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
			return "", false, nil
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query server_member_profile id", zap.Error(err))
		return "", false, err
	}
	return id, true, nil
}

func (repository *ProfileRepository) TryGetServerMemberProfileIdNoTx(ctx context.Context, serverId, userId string) (string, bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.TryGetServerMemberProfileIdNoTx")
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

	query := `SELECT id FROM server_member_profiles WHERE server_id = $1 AND user_id = $2 LIMIT 1`
	var id string
	err = repository.DB.QueryRow(ctx, query, serverId, userId).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
			return "", false, nil
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query profile id", zap.Error(err))
		return "", false, err
	}
	return id, true, nil
}

func (repository *ProfileRepository) GetProfileId(ctx context.Context, tx pgx.Tx, serverId, userId string) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.GetProfileId")
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

	query := `SELECT id FROM server_member_profiles WHERE server_id = $1 AND user_id = $2 LIMIT 1`
	var profileId string
	err = tx.QueryRow(ctx, query, serverId, userId).Scan(&profileId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.NotFoundError{
				Code:    constant.ERR_NOT_FOUND_CODE,
				Message: "Profile not found in this server",
				Param:   "serverId",
			}
			return "", err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get profile id", zap.Error(err))
		return "", err
	}
	return profileId, nil
}

func (repository *ProfileRepository) GetServerMemberProfile(ctx context.Context, serverId, userId, minioFullUrl string) (model.ServerMemberProfileResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.GetServerMemberProfile")
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

	var resp model.ServerMemberProfileResponse
	query := `
        SELECT smp.id, smp.server_id, smp.nickname, smp.bio,
               smp.avatar_image_id, pai.object_key,
               smp.created_at, smp.updated_at
        FROM server_member_profiles smp
        LEFT JOIN profile_avatar_images pai ON smp.avatar_image_id = pai.id
        WHERE smp.server_id = $1 AND smp.user_id = $2
        LIMIT 1
    `

	err = repository.DB.QueryRow(ctx, query, serverId, userId).Scan(
		&resp.ProfileId, &resp.ServerId, &resp.Nickname, &resp.Bio,
		&resp.AvatarImageId, &resp.AvatarImageUrl,
		&resp.CreatedAt, &resp.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.NotFoundError{
				Code:    constant.ERR_NOT_FOUND_CODE,
				Message: "You don't have a profile in this server",
				Param:   "serverId",
			}
			return resp, err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get server member profile", zap.Error(err))
		return resp, err
	}

	if resp.AvatarImageUrl != nil {
		*resp.AvatarImageUrl = fmt.Sprintf("%s/%s", minioFullUrl, *resp.AvatarImageUrl)
	}

	return resp, nil
}

func (repository *ProfileRepository) CreateProfileAvatarImage(ctx context.Context, tx pgx.Tx, image model.ProfileAvatarImage) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.CreateProfileAvatarImage")
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
		attribute.String("profile.avatar_image_id", image.Id),
	)

	query := `INSERT INTO profile_avatar_images
              (id, bucket, object_key, mime_type, size, created_at, updated_at, created_by, updated_by)
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err = tx.Exec(ctx, query,
		image.Id, image.Bucket, image.ObjectKey, image.MimeType, image.Size,
		image.CreatedAt, image.UpdatedAt, image.CreatedBy, image.UpdatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to create profile_avatar_image", zap.Error(err))
		return err
	}
	return nil
}

func (repository *ProfileRepository) UpdateProfileAvatarImageId(ctx context.Context, tx pgx.Tx, profileId string, avatarImageId *string, updatedBy string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.UpdateProfileAvatarImageId")
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
		attribute.String("profile.id", profileId),
	)

	query := `UPDATE server_member_profiles
              SET avatar_image_id = $1, updated_at = $2, updated_by = $3
              WHERE id = $4`

	_, err = tx.Exec(ctx, query, avatarImageId, updatedAt, updatedBy, profileId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update profile avatar_image_id", zap.Error(err))
		return err
	}
	return nil
}

func (repository *ProfileRepository) GetProfileHistory(ctx context.Context, userId, minioFullUrl string) ([]model.GetProfileHistoryResponseItem, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.GetProfileHistory")
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

	query := `
        SELECT
            smp.id, smp.server_id, s.name,
            smp.nickname, smp.bio,
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
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to query profile history", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var items []model.GetProfileHistoryResponseItem
	for rows.Next() {
		var item model.GetProfileHistoryResponseItem
		err = rows.Scan(
			&item.ProfileId, &item.ServerId, &item.ServerName,
			&item.Nickname, &item.Bio,
			&item.AvatarImageId, &item.AvatarImageUrl,
			&item.IsStillMember,
			&item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan profile history row", zap.Error(err))
			return nil, err
		}
		if item.AvatarImageUrl != nil {
			*item.AvatarImageUrl = fmt.Sprintf("%s/%s", minioFullUrl, *item.AvatarImageUrl)
		}
		items = append(items, item)
	}

	return items, nil
}

func (repository *ProfileRepository) UploadObject(ctx context.Context, bucket, objectKey string, file *bytes.Reader, size int64) error {
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
		attribute.String("db.system", "minio"),
		attribute.String("db.operation", "PUT"),
		attribute.String("db.minio.bucket", bucket),
		attribute.String("db.minio.object", objectKey),
	)

	_, err = repository.DBObject.PutObject(ctx, bucket, objectKey, file, size,
		minio.PutObjectOptions{
			ContentType:  "image/webp",
			CacheControl: "public, max-age=31536000, immutable",
		})
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to upload profile avatar object", zap.Error(err))
		return err
	}
	return nil
}

func (repository *ProfileRepository) CheckProfileAvatarImageOwnership(ctx context.Context, userId, avatarImageId string) (bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.CheckProfileAvatarImageOwnership")
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
		attribute.String("profile.avatar_image_id", avatarImageId),
	)

	query := `SELECT EXISTS (
        SELECT 1 FROM server_member_profiles
        WHERE user_id = $1 AND avatar_image_id = $2
    )`
	var exists bool
	err = repository.DB.QueryRow(ctx, query, userId, avatarImageId).Scan(&exists)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check avatar ownership", zap.Error(err))
		return false, err
	}
	return exists, nil
}

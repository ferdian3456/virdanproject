package user

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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

func (repository *Repository) CheckEmailUnique(ctx context.Context, email string) (bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CheckEmailUnique")
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
		attribute.String("user.email", email),
	)

	query := `SELECT 1 FROM users WHERE email = $1 AND deleted_at IS NULL LIMIT 1`

	var exists int
	err = repository.DB.QueryRow(ctx, query, email).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check email uniqueness", zap.Error(err))
		return false, err
	}

	return true, nil
}

func (repository *Repository) GetUserInfo(ctx context.Context, id string) (UserResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetUserInfo")
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
		attribute.String("user.id", id),
	)

	query := `SELECT id, email, settings, created_at, updated_at FROM users
			  WHERE id = $1 AND deleted_at IS NULL LIMIT 1`

	var resp UserResponse
	err = repository.DB.QueryRow(ctx, query, id).Scan(
		&resp.Id, &resp.Email, &resp.Settings, &resp.CreatedAt, &resp.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &shared.NotFoundError{
				Code:    shared.ERR_NOT_FOUND_CODE,
				Message: "User not found",
				Param:   "userId",
			}
			return resp, err
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get user info by ID", zap.Error(err))
		return resp, err
	}

	return resp, nil
}

func (repository *Repository) CheckUserActive(ctx context.Context, userId string) (bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.CheckUserActive")
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

	query := `SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL LIMIT 1`

	var exists int
	err = repository.DB.QueryRow(ctx, query, userId).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to check user active status", zap.Error(err))
		return false, err
	}

	return true, nil
}

func (repository *Repository) HardDeleteUser(ctx context.Context, tx pgx.Tx, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.HardDeleteUser")
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

	query := `DELETE FROM users WHERE id = $1`

	_, err = tx.Exec(ctx, query, userId)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to hard delete user", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) GetPasswordHashById(ctx context.Context, userId string) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetPasswordHashById")
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

	query := `SELECT password FROM users WHERE id = $1 AND deleted_at IS NULL LIMIT 1`

	var hash string
	err = repository.DB.QueryRow(ctx, query, userId).Scan(&hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &shared.NotFoundError{
				Code:    shared.ERR_NOT_FOUND_CODE,
				Message: "User not found",
				Param:   "userId",
			}
			return "", err
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get password hash", zap.Error(err))
		return "", err
	}
	return hash, nil
}

func (repository *Repository) UpdatePasswordHash(ctx context.Context, userId, newHash string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdatePasswordHash")
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
		attribute.String("user.id", userId),
	)

	query := `UPDATE users SET password = $1, updated_at = $2, updated_by = $3 WHERE id = $4 AND deleted_at IS NULL`
	_, err = repository.DB.Exec(ctx, query, newHash, updatedAt, userId, userId)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update password hash", zap.Error(err))
		return err
	}
	return nil
}

func (repository *Repository) UpdateEmail(ctx context.Context, userId, newEmail string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdateEmail")
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
		attribute.String("user.id", userId),
	)
	query := `UPDATE users SET email = $1, updated_at = $2, updated_by = $3 WHERE id = $4 AND deleted_at IS NULL`
	_, err = repository.DB.Exec(ctx, query, newEmail, updatedAt, userId, userId)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			err = &shared.ConflictError{
				Code:    shared.ERR_CONFLICT_CODE,
				Message: "Email already in use",
				Param:   "newEmail",
			}
			return err
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update user email", zap.Error(err))
		return err
	}
	return nil
}

func (repository *Repository) GetUserEmail(ctx context.Context, userId string) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetUserEmail")
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
	query := `SELECT email FROM users WHERE id = $1 AND deleted_at IS NULL LIMIT 1`
	var email string
	err = repository.DB.QueryRow(ctx, query, userId).Scan(&email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &shared.NotFoundError{
				Code:    shared.ERR_NOT_FOUND_CODE,
				Message: "User not found",
				Param:   "userId",
			}
			return "", err
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get user email", zap.Error(err))
		return "", err
	}
	return email, nil
}

func (repository *Repository) UpdateNotificationPrefs(ctx context.Context, userId string, prefs NotificationPrefs, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdateNotificationPrefs")
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
		attribute.String("user.id", userId),
	)

	query := `UPDATE users
	          SET settings = settings || jsonb_build_object(
	              'notif_like',    $1::boolean,
	              'notif_comment', $2::boolean,
	              'notif_reply',   $3::boolean
	          ),
	          updated_at = $4,
	          updated_by = $5
	          WHERE id = $6 AND deleted_at IS NULL`

	_, err = repository.DB.Exec(ctx, query,
		prefs.NotifLike, prefs.NotifComment, prefs.NotifReply,
		updatedAt, userId, userId)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update notification prefs", zap.Error(err))
		return err
	}
	return nil
}

func (repository *Repository) RemoveAllAccessTokensFromCache(ctx context.Context, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.RemoveAllAccessTokensFromCache")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "DEL"),
		attribute.String("user.id", userId),
	)

	accessTokenKey := fmt.Sprintf("auth:accessToken:%s", userId)

	err = repository.DBCache.Del(ctx, accessTokenKey).Err()
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to clear access token cache", zap.Error(err))
		return err
	}

	return nil
}

func (repository *Repository) SetEmailChangeSession(ctx context.Context, userId, newEmail, otpHash string, ttl time.Duration) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.SetEmailChangeSession")
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HSET"),
		attribute.String("user.id", userId),
	)

	key := fmt.Sprintf("email_change:%s", userId)
	pipe := repository.DBCache.TxPipeline()
	pipe.Del(ctx, key)
	pipe.HSet(ctx, key, map[string]interface{}{
		"newEmail": newEmail,
		"otpHash":  otpHash,
		"attempts": "0",
	})
	pipe.Expire(ctx, key, ttl)
	_, err = pipe.Exec(ctx)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to set email change session", zap.Error(err))
		return err
	}
	return nil
}

func (repository *Repository) GetEmailChangeSessionTTL(ctx context.Context, userId string) (time.Duration, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetEmailChangeSessionTTL")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "TTL"),
		attribute.String("user.id", userId),
	)
	key := fmt.Sprintf("email_change:%s", userId)
	var ttl time.Duration
	ttl, err = repository.DBCache.TTL(ctx, key).Result()
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to read email change session TTL", zap.Error(err))
		return 0, err
	}
	return ttl, nil
}

func (repository *Repository) GetEmailChangeSession(ctx context.Context, userId string) (newEmail, otpHash string, attempts int, err error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetEmailChangeSession")
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HMGET"),
		attribute.String("user.id", userId),
	)
	key := fmt.Sprintf("email_change:%s", userId)
	var data []interface{}
	data, err = repository.DBCache.HMGet(ctx, key, "newEmail", "otpHash", "attempts").Result()
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get email change session", zap.Error(err))
		return "", "", 0, err
	}
	if data[0] == nil || data[1] == nil {
		return "", "", 0, nil
	}
	newEmail, _ = data[0].(string)
	otpHash, _ = data[1].(string)
	if data[2] != nil {
		if s, ok := data[2].(string); ok {
			if n, convErr := strconv.Atoi(s); convErr == nil {
				attempts = n
			}
		}
	}
	return newEmail, otpHash, attempts, nil
}

func (repository *Repository) IncrementEmailChangeAttempts(ctx context.Context, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.IncrementEmailChangeAttempts")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "HINCRBY"),
		attribute.String("user.id", userId),
	)
	key := fmt.Sprintf("email_change:%s", userId)
	_, err = repository.DBCache.HIncrBy(ctx, key, "attempts", 1).Result()
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to bump email change attempts", zap.Error(err))
		return err
	}
	return nil
}

func (repository *Repository) DeleteEmailChangeSession(ctx context.Context, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.DeleteEmailChangeSession")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(
		attribute.String("db.system", "redis"),
		attribute.String("db.operation", "DEL"),
		attribute.String("user.id", userId),
	)
	key := fmt.Sprintf("email_change:%s", userId)
	_, err = repository.DBCache.Del(ctx, key).Result()
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete email change session", zap.Error(err))
		return err
	}
	return nil
}

package repository

import (
	"context"

	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

type NotificationRepository struct {
	Log    *zap.Logger
	Config *koanf.Koanf
	DB     *pgxpool.Pool
}

func NewNotificationRepository(log *zap.Logger, config *koanf.Koanf, db *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{
		DB:     db,
		Log:    log,
		Config: config,
	}
}

func (repository *NotificationRepository) UpsertDeviceToken(ctx context.Context, tx pgx.Tx, token model.DeviceToken) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.UpsertDeviceToken")
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
		attribute.String("user.id", token.UserId),
	)

	query := `INSERT INTO device_tokens
	          (id, user_id, token, platform, created_at, updated_at, created_by, updated_by)
	          VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	          ON CONFLICT (token) DO UPDATE SET
	              user_id    = EXCLUDED.user_id,
	              platform   = EXCLUDED.platform,
	              updated_at = EXCLUDED.updated_at,
	              updated_by = EXCLUDED.updated_by`

	_, err = tx.Exec(ctx, query,
		token.Id, token.UserId, token.Token, token.Platform,
		token.CreatedAt, token.UpdatedAt, token.CreatedBy, token.UpdatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to upsert device_token", zap.Error(err))
		return err
	}
	return nil
}

func (repository *NotificationRepository) DeleteAllUserDeviceToken(ctx context.Context, tx pgx.Tx, userId string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.DeleteAllUserDeviceToken")
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

	query := "DELETE FROM device_tokens WHERE user_id = $1"
	_, err = tx.Exec(ctx, query, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete all user device_token", zap.Error(err))
		return err
	}
	return nil
}

func (repository *NotificationRepository) ListTokensByUserId(ctx context.Context, userId string) ([]string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.ListTokensByUserId")
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

	query := "SELECT token FROM device_tokens WHERE user_id = $1"
	rows, err := repository.DB.Query(ctx, query, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to list device tokens", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var token string
		if err = rows.Scan(&token); err != nil {
			util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan device token", zap.Error(err))
			return nil, err
		}
		tokens = append(tokens, token)
	}

	return tokens, nil
}

func (repository *NotificationRepository) DeleteDeviceToken(ctx context.Context, userId string, token string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.DeleteDeviceToken")
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

	query := "DELETE FROM device_tokens WHERE user_id = $1 AND token = $2"
	_, err = repository.DB.Exec(ctx, query, userId, token)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete device token", zap.Error(err))
		return err
	}
	return nil
}

func (repository *NotificationRepository) DeleteInvalidTokens(ctx context.Context, tokens []string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.DeleteInvalidTokens")
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
	)

	query := "DELETE FROM device_tokens WHERE token = ANY($1)"
	_, err = repository.DB.Exec(ctx, query, tokens)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete invalid tokens", zap.Error(err))
		return err
	}
	return nil
}

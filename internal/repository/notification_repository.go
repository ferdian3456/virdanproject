package repository

import (
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

// InsertNotification persists one notification row. At insert, updated_at = created_at and
// updated_by = actor (audit baseline before any mark-read mutation).
func (repository *NotificationRepository) InsertNotification(ctx context.Context, notification model.Notification) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.InsertNotification")
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
		attribute.String("user.id", notification.RecipientUserId),
	)

	query := `INSERT INTO notifications
	          (id, recipient_user_id, actor_user_id, actor_profile_id, type, server_id, post_id, comment_id, created_at, updated_at, created_by, updated_by)
	          VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`

	_, err = repository.DB.Exec(ctx, query,
		notification.Id, notification.RecipientUserId, notification.ActorUserId,
		notification.ActorProfileId, notification.Type, notification.ServerId,
		notification.PostId, notification.CommentId,
		notification.CreatedAt, notification.UpdatedAt,
		notification.CreatedBy, notification.UpdatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to insert notification", zap.Error(err))
		return err
	}
	return nil
}

// ListByRecipient returns a user's feed rows, newest first. The caller passes limit+1 (fetch one
// extra) so the usecase can tell whether a next page exists without a separate count. Cursor is a
// (created_at, id) tuple: id is the tie-breaker when two notifs share created_at (same goroutine
// batch), so paging never skips or duplicates a row. Joins server_member_profiles via
// actor_profile_id (PK lookup) for the actor's per-server username + avatar. Returns raw rows
// only — cursor encoding lives in the usecase.
func (repository *NotificationRepository) ListByRecipient(ctx context.Context, userId string, cursor *model.NotificationCursor, limit int, minioFullUrl string) ([]model.NotificationResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.ListNotificationsByRecipient")
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
	if cursor == nil {
		query := `SELECT n.id, n.type, smp.username, pai.object_key,
		                 n.server_id, n.post_id, n.comment_id, n.read_at, n.created_at
		          FROM notifications n
		          JOIN server_member_profiles smp ON smp.id = n.actor_profile_id
		          LEFT JOIN profile_avatar_images pai ON pai.id = smp.avatar_image_id
		          WHERE n.recipient_user_id = $1
		          ORDER BY n.created_at DESC, n.id DESC
		          LIMIT $2`
		rows, err = repository.DB.Query(ctx, query, userId, limit)
	} else {
		query := `SELECT n.id, n.type, smp.username, pai.object_key,
		                 n.server_id, n.post_id, n.comment_id, n.read_at, n.created_at
		          FROM notifications n
		          JOIN server_member_profiles smp ON smp.id = n.actor_profile_id
		          LEFT JOIN profile_avatar_images pai ON pai.id = smp.avatar_image_id
		          WHERE n.recipient_user_id = $1
		            AND (n.created_at, n.id) < ($2, $3)
		          ORDER BY n.created_at DESC, n.id DESC
		          LIMIT $4`
		rows, err = repository.DB.Query(ctx, query, userId, cursor.CreatedAt, cursor.Id, limit)
	}
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to list notifications", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var items []model.NotificationResponse
	for rows.Next() {
		var item model.NotificationResponse
		var objectKey *string
		if err = rows.Scan(
			&item.Id, &item.Type, &item.ActorUsername, &objectKey,
			&item.ServerId, &item.PostId, &item.CommentId, &item.ReadAt, &item.CreatedAt,
		); err != nil {
			util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan notification row", zap.Error(err))
			return nil, err
		}
		if objectKey != nil {
			formatted := fmt.Sprintf("%s/%s", minioFullUrl, *objectKey)
			item.ActorAvatarUrl = &formatted
		}
		items = append(items, item)
	}

	return items, nil
}

// MarkRead stamps read_at on one notification. Scoped recipient_user_id (a user can only mark
// their own) and guarded read_at IS NULL (a second tap won't overwrite the original read time).
// readAt comes from the usecase (time generated in Go, not SQL NOW()).
func (repository *NotificationRepository) MarkRead(ctx context.Context, userId string, notifId string, readAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.MarkNotificationRead")
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
		attribute.String("user.id", userId),
		attribute.String("notification.id", notifId),
	)

	query := `UPDATE notifications SET read_at = $3, updated_at = $4, updated_by = $5
	          WHERE id = $1 AND recipient_user_id = $2 AND read_at IS NULL`
	_, err = repository.DB.Exec(ctx, query, notifId, userId, readAt, readAt, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to mark notification as read", zap.Error(err))
		return err
	}
	return nil
}

// CountUnread returns the number of unread notifications (read_at IS NULL) for the tab badge.
func (repository *NotificationRepository) CountUnread(ctx context.Context, userId string) (int, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.CountUnreadNotifications")
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

	query := `SELECT COUNT(*) FROM notifications WHERE recipient_user_id = $1 AND read_at IS NULL`
	var count int
	err = repository.DB.QueryRow(ctx, query, userId).Scan(&count)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to count unread notifications", zap.Error(err))
		return 0, err
	}
	return count, nil
}

// GetActorUsernameAndAvatar fetches the actor's per-server username + avatar for the push title.
// actor_profile_id is FK-enforced, so a missing row is a real anomaly -> NotFoundError (Notify
// logs it + skips the push; the feed row is already saved). avatar may be nil (no avatar) -> not
// an error.
func (repository *NotificationRepository) GetActorUsernameAndAvatar(ctx context.Context, profileId string, minioFullUrl string) (string, *string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.GetActorUsernameAndAvatar")
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

	query := `SELECT smp.username, pai.object_key
	          FROM server_member_profiles smp
	          LEFT JOIN profile_avatar_images pai ON pai.id = smp.avatar_image_id
	          WHERE smp.id = $1 LIMIT 1`

	var username string
	var objectKey *string
	err = repository.DB.QueryRow(ctx, query, profileId).Scan(&username, &objectKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.NotFoundError{Code: constant.ERR_NOT_FOUND_CODE, Message: "Actor profile not found", Param: "actorProfileId"}
			return "", nil, err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get actor username", zap.Error(err))
		return "", nil, err
	}

	var avatarUrl *string
	if objectKey != nil {
		formatted := fmt.Sprintf("%s/%s", minioFullUrl, *objectKey)
		avatarUrl = &formatted
	}
	return username, avatarUrl, nil
}

// GetUserNotificationPrefs reads per-type PUSH toggles from users.settings JSONB. Defaults are
// written at signup (settings seeded with notif_* = true), so the COALESCE(..., true) here is just
// a defensive net for any legacy row missing a key. ErrNoRows = recipient gone -> return error so
// Notify logs + skips push.
func (repository *NotificationRepository) GetUserNotificationPrefs(ctx context.Context, userId string) (model.NotificationPrefs, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.GetUserNotificationPrefs")
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

	query := `SELECT
	              COALESCE((settings->>'notif_like')::boolean, true),
	              COALESCE((settings->>'notif_comment')::boolean, true),
	              COALESCE((settings->>'notif_reply')::boolean, true)
	          FROM users WHERE id = $1 AND deleted_at IS NULL`

	var prefs model.NotificationPrefs
	err = repository.DB.QueryRow(ctx, query, userId).Scan(
		&prefs.NotifLike, &prefs.NotifComment, &prefs.NotifReply,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.NotFoundError{Code: constant.ERR_NOT_FOUND_CODE, Message: "User not found", Param: "userId"}
			return model.NotificationPrefs{}, err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get notification prefs", zap.Error(err))
		return model.NotificationPrefs{}, err
	}
	return prefs, nil
}

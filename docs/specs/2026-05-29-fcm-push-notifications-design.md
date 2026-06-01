# Design Spec: FCM Push Notifications + Notification Feed + Mentions

> Status: In Progress (Fase 1 DONE, Fase 2 in implementation)
> Tanggal: 2026-05-29 | Last updated: 2026-05-30
> Scope: Backend (`virdanproject/`) + Frontend (`virdanmobileapp-flutter/`)
> Jira: VIR-69 (In Progress)
> Platform: Android dulu (iOS ditunda)

---

## 1. Tujuan

Tambah sistem notifikasi production-grade ke Virdan:

1. **Push notification** via Firebase Cloud Messaging (FCM) — muncul di status bar / lock screen HP.
2. **Notification feed** — halaman "Notifikasi" di app (arsip persisten, read/unread), ganti mock yang ada sekarang.
3. **Mentions** — `@username` di caption post & body comment, dengan autocomplete + highlight + notif.

Event yang memicu notif: **like post, comment, reply comment, mention**.

---

## 2. Keputusan terkunci

| Topik | Keputusan |
|---|---|
| Platform | Android dulu. iOS ditunda (butuh Apple Developer Program + APNs). |
| Firebase project | Belum ada → dibuat di Fase 0 (manual). |
| Event trigger | like, comment, reply, mention (caption post + body comment). |
| Cara kirim push | Goroutine fire-and-forget di `NotificationUsecase.Notify` (no interface terpisah — keputusan: hindari over-engineering, FCM test manual). Swap ke Redis Streams di Fase 3. |
| Mention key | **`username`** (lowercase, no-space, sudah jadi `@handle` di UI). BUKAN nickname. |
| Mention storage | Tabel `mentions` (2 FK nullable + CHECK XOR). |
| Mention target | Hanya member server yang sama. Username tidak ketemu di server → diabaikan. |
| Autocomplete | Masuk scope (endpoint search prefix + dropdown FE). |
| Post edit | Re-sync `mentions` (tambah baru, hapus hilang), notif **hanya yang baru**. Comment tidak bisa edit. |
| Dedup | `mention > reply > comment > like`. 1 notif per recipient per aksi. |
| Reply | Notif **hanya** ke author comment induk. Post owner TIDAK dapat notif per reply. Comment top-level → post owner. |
| Message FCM | Hybrid `notification` + `data`. `data` = `{type, serverId, postId, commentId}` untuk deep-link. |
| Multi-device | 1 row `device_tokens` per device. Test-send fan-out ke semua device user. |
| Notification feed | Simpan semua tipe. Push = channel, feed = arsip. |
| Unread badge | Masuk Fase 2. |
| Preferences (toggle) | **Fase 2** (dipindah dari Fase 3). Toggle per-type (like/comment/reply) via `users.settings` JSONB. |
| Grouping ("A & 3 lainnya") | Out of scope. 1 event = 1 notif. |
| Self-action | Skip notif untuk like/comment/reply/mention diri sendiri. |
| Hapus post/comment | `mentions` + `notifications` terkait ikut terhapus (FK cascade). |

---

## 3. Arsitektur

```
Flutter ──register/delete token──▶ POST/DELETE /api/devices ──▶ device_tokens

Domain event (like/comment/reply/mention) di post_usecase
  └─▶ NotificationUsecase.Notify([]event)   [goroutine, context.Background()+timeout, recover, OTel span]
        1. dedup per recipient (mention>reply>comment>like)
        2. cek preferensi recipient (users.settings JSONB) → skip kalau toggle off
        3. insert row(s) ke notifications
        4. ambil device_tokens recipient (ListTokensByUserId)
        5. SendEachForMulticast ──▶ FCM ──▶ device
        6. response per-token: Unregistered/InvalidArgument ──▶ DeleteInvalidTokens

Flutter terima:
  - background/terminated: FCM auto-display (notification payload)
  - foreground: onMessage ──▶ flutter_local_notifications display
  - tap: onMessageOpenedApp / getInitialMessage ──▶ deep-link via go_router
```

---

## 4. Out of scope

- iOS (APNs) — fase lanjutan.
- Notif preferences server-side (Fase 3).
- Grouping/agregasi notif.
- Real-time in-app feed update (WebSocket) — feed di-fetch via REST + refresh.
- Notif untuk follow/mention-via-DM (fitur follow/DM belum ada).

---

## 5. Backend design (`virdanproject/`)

Pola wajib ikut existing: `controller → usecase → repository`, pgx raw SQL (no ORM), Atlas migration (`make migrate-diff`), OTel span per layer, `ApiError` untuk error, auth middleware set `ctx.Locals("userId")`.

> **Prinsip data generation**: semua nilai (UUID, timestamp, dll) di-generate di Go, bukan SQL. Query hanya terima nilai via `$N`. Dilarang `NOW()`, `gen_random_uuid()`, atau SQL function apapun dalam INSERT/UPDATE.

### 5.1 Config Firebase

File: `internal/config/firebase.go`

```go
package config

import (
    "context"
    "encoding/base64"

    firebase "firebase.google.com/go/v4"
    "firebase.google.com/go/v4/messaging"
    "github.com/knadh/koanf/v2"
    "go.uber.org/zap"
    "google.golang.org/api/option"
)

// NewFCMClient load service account dari env FIREBASE_SERVICE_ACCOUNT_BASE64_JSON
// (nilai wajib base64-encoded JSON). Fatal kalau env kosong atau bukan base64 valid.
func NewFCMClient(ctx context.Context, config *koanf.Koanf, log *zap.Logger) *messaging.Client {
    raw := config.String("FIREBASE_SERVICE_ACCOUNT_BASE64_JSON")
    if raw == "" {
        log.Fatal("Firebase service account env is empty (FIREBASE_SERVICE_ACCOUNT_BASE64_JSON)")
    }

    creds, err := base64.StdEncoding.DecodeString(raw)
    if err != nil {
        log.Fatal("Firebase service account is not valid base64", zap.Error(err))
    }

    app, err := firebase.NewApp(ctx, nil, option.WithCredentialsJSON(creds))
    if err != nil {
        log.Fatal("Failed to initialize firebase app", zap.Error(err))
    }

    client, err := app.Messaging(ctx)
    if err != nil {
        log.Fatal("Failed to create firebase messaging client", zap.Error(err))
    }

    return client
}
```

Wire di `cmd/main.go` setelah `NewMinIO`:
```go
fcmClient := config.NewFCMClient(initCtx, koanf, zap)
```

Wire di `internal/config/app.go` — field `FCM *messaging.Client` di `ServerConfig`.

Env di `.env.example`:
```bash
# base64-encode service account JSON: base64 -w0 firebase-service-account.json
FIREBASE_SERVICE_ACCOUNT_BASE64_JSON=<base64-encoded-service-account-json>
```

### 5.2 Migrations (Atlas — edit `db/schema.sql` lalu `make migrate-diff`)

**device_tokens** (Fase 1 — DONE):

```sql
CREATE TABLE IF NOT EXISTS device_tokens (
    id         uuid PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token      text NOT NULL,
    platform   varchar(10) NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    created_by uuid NOT NULL,
    updated_by uuid NOT NULL
);
CREATE UNIQUE INDEX idx_device_tokens_uk_01 ON device_tokens(token);
CREATE INDEX        idx_device_tokens_pk_01 ON device_tokens(user_id);
```

> `platform` = `'android'` | `'ios'` (validated di Go, bukan CHECK constraint).
> Register = upsert `ON CONFLICT (token) DO UPDATE` → handle token pindah user.
> 1 token aktif per user: `RegisterDevice` delete semua token lama via tx sebelum insert baru.

**notifications** (Fase 2):

```sql
CREATE TABLE IF NOT EXISTS notifications (
    id                uuid PRIMARY KEY,
    recipient_user_id uuid NOT NULL REFERENCES users(id)                  ON DELETE CASCADE,
    actor_user_id     uuid NOT NULL REFERENCES users(id)                  ON DELETE CASCADE,
    actor_profile_id  uuid NOT NULL REFERENCES server_member_profiles(id) ON DELETE CASCADE,
    type              varchar(20) NOT NULL,
    server_id         uuid NOT NULL REFERENCES servers(id)                ON DELETE CASCADE,
    post_id           uuid NULL REFERENCES server_posts(id)               ON DELETE CASCADE,
    comment_id        uuid NULL REFERENCES server_post_comments(id)       ON DELETE CASCADE,
    read_at           timestamptz NULL,
    created_at        timestamptz NOT NULL,
    updated_at        timestamptz NOT NULL,
    created_by        uuid NOT NULL,
    updated_by        uuid NOT NULL
);
CREATE INDEX idx_notifications_pk_01 ON notifications(recipient_user_id, created_at DESC);
CREATE INDEX idx_notifications_pk_02 ON notifications(recipient_user_id) WHERE read_at IS NULL;
```

> `actor_profile_id` = denormalisasi untuk performa (direct PK lookup vs composite key JOIN).
> Aman: profile hanya terhapus jika server/user dihapus, yang juga cascade-delete notifications via `server_id`/`recipient_user_id` FK.
> Insert: `updated_at = created_at`, `updated_by = actor_user_id`.
> MarkRead: `updated_at = readAt`, `updated_by = recipient_user_id`.
> `type` values: `'like'` | `'comment'` | `'reply'` | `'mention'`.

**mentions** (Fase 2.5):

```sql
CREATE TABLE IF NOT EXISTS mentions (
    id                   uuid PRIMARY KEY,
    server_id            uuid NOT NULL REFERENCES servers(id)                ON DELETE CASCADE,
    post_id              uuid NULL     REFERENCES server_posts(id)           ON DELETE CASCADE,
    comment_id           uuid NULL     REFERENCES server_post_comments(id)   ON DELETE CASCADE,
    mentioned_user_id    uuid NOT NULL REFERENCES users(id)                  ON DELETE CASCADE,
    mentioned_profile_id uuid NOT NULL REFERENCES server_member_profiles(id) ON DELETE CASCADE,
    created_at           timestamptz NOT NULL,
    created_by           uuid NOT NULL,
    CONSTRAINT mentions_source_chk CHECK ((post_id IS NOT NULL) <> (comment_id IS NOT NULL))
);
CREATE INDEX        idx_mentions_pk_01 ON mentions(mentioned_user_id);
CREATE INDEX        idx_mentions_pk_02 ON mentions(post_id)    WHERE post_id IS NOT NULL;
CREATE INDEX        idx_mentions_pk_03 ON mentions(comment_id) WHERE comment_id IS NOT NULL;
CREATE UNIQUE INDEX idx_mentions_uk_01 ON mentions(post_id, mentioned_user_id)    WHERE post_id IS NOT NULL;
CREATE UNIQUE INDEX idx_mentions_uk_02 ON mentions(comment_id, mentioned_user_id) WHERE comment_id IS NOT NULL;
```

**Index autocomplete** (Fase 2.5):
```sql
CREATE INDEX idx_server_member_profiles_pk_username ON server_member_profiles (server_id, username varchar_pattern_ops);
```

### 5.3 Models (baru)

**`internal/model/device_token.go`**:
```go
package model

import "time"

type DeviceToken struct {
    Id        string
    UserId    string
    Token     string
    Platform  string
    CreatedAt time.Time
    UpdatedAt time.Time
    CreatedBy string
    UpdatedBy string
}

type DeviceTokenRegisterRequest struct {
    Token    string `json:"token"`
    Platform string `json:"platform"`
}

type DeviceTokenDeleteRequest struct {
    Token string `json:"token"`
}

type PushPayload struct {
    Title string
    Body  string
    Data  map[string]string
}
```

**`internal/model/notification.go`**:
```go
package model

import "time"

type Notification struct {
    Id               string
    RecipientUserId  string
    ActorUserId      string
    ActorProfileId   string
    Type             string
    ServerId         string
    PostId           *string
    CommentId        *string
    ReadAt           *time.Time
    CreatedAt        time.Time
    UpdatedAt        time.Time
    CreatedBy        string
    UpdatedBy        string
}

// NotificationEvent adalah unit kerja perantara dari post_usecase ke Notify().
// Tidak disimpan langsung ke DB — dipakai untuk dedup lalu buat Notification row.
type NotificationEvent struct {
    Type            string
    RecipientUserId string
    ActorUserId     string
    ActorProfileId  string
    ServerId        string
    PostId          string
    CommentId       *string
}

type NotificationResponse struct {
    Id             string     `json:"id"`
    Type           string     `json:"type"`
    ActorUsername  string     `json:"actorUsername"`
    ActorAvatarUrl *string    `json:"actorAvatarUrl"`
    ServerId       string     `json:"serverId"`
    PostId         *string    `json:"postId"`
    CommentId      *string    `json:"commentId"`
    ReadAt         *time.Time `json:"readAt"`
    CreatedAt      time.Time  `json:"createdAt"`
}

type NotificationListResponse struct {
    Data []NotificationResponse `json:"data"`
    Page Page                   `json:"page"`
}

type UnreadCountResponse struct {
    Count int `json:"count"`
}

type NotificationCursor struct {
    CreatedAt time.Time `json:"createdAt"`
    Id        string    `json:"id"`
}

type NotificationPrefs struct {
    NotifLike    bool
    NotifComment bool
    NotifReply   bool
}

type UpdateNotificationPreferencesRequest struct {
    NotifLike    bool `json:"notifLike"`
    NotifComment bool `json:"notifComment"`
    NotifReply   bool `json:"notifReply"`
}
```

### 5.4 Repositories

**`internal/repository/notification_repository.go`** — satu file untuk device tokens + notifications.

Imports yang dibutuhkan: `context`, `errors`, `fmt`, `model`, `util`, `pgx`, `pgxpool`, `koanf`, `otel`, `attribute`, `zap`.

#### 5.4.1 Device token methods (Fase 1 — DONE)

```go
// UpsertDeviceToken inserts a token, or on ON CONFLICT (token) reassigns user_id — handles
// the same physical device logging in as a different user. Runs inside the RegisterDevice tx
// (paired with DeleteAllUserDeviceToken) so register = atomic replace.
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

// DeleteAllUserDeviceToken wipes a user's tokens before inserting the new one, enforcing
// 1 active token per user (matches the 1-session login model; multi-device = TD-007).
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

// ListTokensByUserId returns all FCM tokens of a user — the recipient list that
// SendEachForMulticast needs, so it runs before every push. Returns a slice because the
// multicast API takes many tokens (currently ~1 per user under the 1-session model).
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

// DeleteDeviceToken removes one token on logout. Scoped by user_id (from JWT) because the
// token arrives in the user's request body (untrusted): scoping stops a user from deleting
// another user's token.
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

// DeleteInvalidTokens removes dead tokens that FCM flagged (Unregistered/InvalidArgument)
// after a send. NOT scoped by user_id — the tokens come from FCM's authoritative response,
// not user input, so there is no attack vector and no need to scope.
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
```

#### 5.4.2 Notification methods (Fase 2)

```go
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

// ListByRecipient returns a user's feed rows, newest first. The caller passes limit+1 (fetch
// one extra) so the usecase can tell whether a next page exists WITHOUT a separate count — see
// GetFeed. Cursor is a (created_at, id) tuple: id is the tie-breaker when two notifs share
// created_at (same goroutine batch), so paging never skips or duplicates a row. Joins
// server_member_profiles via actor_profile_id (PK lookup) for the actor's per-server username
// + avatar. Returns RAW rows only — cursor encoding lives in the usecase (codebase convention,
// matches GetServerPosts).
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
// their own) and guarded read_at IS NULL (a second tap won't overwrite the original read
// time). readAt comes from the usecase (time generated in Go, not SQL NOW()).
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

// GetActorUsernameAndAvatar fetches the actor's per-server username + avatar for the push
// title. actor_profile_id is FK-enforced, so a missing row is a real anomaly → returns
// NotFoundError (Notify logs it + skips the push; the feed row is already saved). avatar may be
// nil (user has no avatar) — that is NOT an error.
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
// written at signup (settings seeded with notif_* = true), so the COALESCE(..., true) here is
// just a defensive net for any legacy row missing a key — not the primary mechanism.
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
		// ErrNoRows = recipient user gone (deleted). Don't mask with defaults — return the
		// error so the caller (Notify) logs it + skips the push for this recipient.
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.NotFoundError{Code: constant.ERR_NOT_FOUND_CODE, Message: "User not found", Param: "userId"}
			return model.NotificationPrefs{}, err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to get notification prefs", zap.Error(err))
		return model.NotificationPrefs{}, err
	}
	return prefs, nil
}
```

#### 5.4.3 `internal/repository/user_repository.go` (Fase 2 — tambah 1 method)

```go
// UpdateNotificationPrefs merges only the notif_* keys into users.settings via
// jsonb_build_object, leaving other settings keys untouched. updatedAt is passed in from the
// usecase (timestamp generated in Go).
func (repository *UserRepository) UpdateNotificationPrefs(ctx context.Context, userId string, prefs model.NotificationPrefs, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.UpdateNotificationPrefs")
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
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to update notification prefs", zap.Error(err))
		return err
	}
	return nil
}
```

> `updatedAt` di-generate di usecase (`time.Now()`), bukan SQL — konsisten prinsip data dari Go.

**Signup harus seed default prefs (Fase 2 — ubah existing).** Defaults eksplisit di DB saat user dibuat, bukan andalkan COALESCE. `model.User` dibuat di `user_usecase.go:521` (complete-signup) dengan `Settings: []byte("{}")` — ganti:

```go
// internal/constant/constant.go (tambah)
const DEFAULT_USER_SETTINGS = `{"notif_like":true,"notif_comment":true,"notif_reply":true}`

// internal/usecase/user_usecase.go (complete-signup, baris Settings) — ganti
//   Settings: []byte("{}"),
// jadi:
Settings: []byte(constant.DEFAULT_USER_SETTINGS),
```

> COALESCE di `GetUserNotificationPrefs` tetap dipertahankan sebagai safety net buat row lama yang belum punya key — tapi primary mechanism = seed di signup ini.

#### 5.4.4 `internal/repository/post_repository.go` (Fase 2 — 3 perubahan)

**(a) `CreatePostLikeIdempotent` — ubah return `error` → `(bool, error)`** (butuh import `pgconn`):
```go
// CreatePostLikeIdempotent inserts a like idempotently. Returns true when a row was actually
// inserted, false on the ON CONFLICT no-op (a repeated like). The caller notifies only when
// true — so like/unlike/like toggling never spams the post author.
func (repository *PostRepository) CreatePostLikeIdempotent(ctx context.Context, like model.ServerPostLike) (bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.CreatePostLikeIdempotent")
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
```

**(b) `GetPostAuthorId`**:
```go
// GetPostAuthorId returns a post's author — the notification recipient for a like or a
// top-level comment.
func (repository *PostRepository) GetPostAuthorId(ctx context.Context, postId string) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.GetPostAuthorId")
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
```

**(c) `GetCommentAuthorId`**:
```go
// GetCommentAuthorId returns a comment's author — the notification recipient for a reply
// (the parent comment's author, not the post owner).
func (repository *PostRepository) GetCommentAuthorId(ctx context.Context, commentId string) (string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.GetCommentAuthorId")
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
```

### 5.5 NotificationUsecase (no separate Notifier interface)

> **Keputusan implementasi**: tidak ada `Notifier` interface terpisah. Semua logika FCM masuk `NotificationUsecase`. FCM di-test manual via test-send; DB operations (upsert/delete token, insert notif) di-test via integration test dengan testcontainers.

**Struct**:
```go
type NotificationUsecase struct {
    NotificationRepository *repository.NotificationRepository
    ProfileRepository      *repository.ProfileRepository
    FCMClient              *messaging.Client
    DB                     *pgxpool.Pool
    Log                    *zap.Logger
    Config                 *koanf.Koanf
}
```

Imports: `context`, `fmt`, `time`, `messaging`, `constant`, `model`, `repository`, `util`, `uuid`, `pgxpool`, `koanf`, `otel`, `go.opentelemetry.io/otel/trace` (untuk `SpanContextFromContext` + `ContextWithSpanContext` di `Notify`), `attribute`, `zap`.

#### 5.5.1 Method Fase 1 (DONE)

```go
// RegisterDevice validates token + platform, then in one tx wipes the user's old tokens and
// upserts the new one — enforcing 1 active token per user. Called after login + signup.
func (usecase *NotificationUsecase) RegisterDevice(ctx context.Context, userId string, request model.DeviceTokenRegisterRequest) error {
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-usecase").Start(ctx, "usecase.RegisterDevice")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	v := util.NewValidator()
	v.String("token", request.Token).Required().MaxLen(4096)
	if err = v.Validate(); err != nil {
		return err
	}

	if request.Platform != constant.PLATFORM_ANDROID && request.Platform != constant.PLATFORM_IOS {
		err = &model.BadRequestError{
			Code:    constant.ERR_VALIDATION_CODE,
			Message: "platform must be android or ios",
			Param:   "platform",
		}
		return err
	}

	tx, err := usecase.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	err = usecase.NotificationRepository.DeleteAllUserDeviceToken(ctx, tx, userId)
	if err != nil {
		return err
	}

	now := time.Now()
	deviceToken := model.DeviceToken{
		Id:        uuid.New().String(),
		UserId:    userId,
		Token:     request.Token,
		Platform:  request.Platform,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: userId,
		UpdatedBy: userId,
	}

	err = usecase.NotificationRepository.UpsertDeviceToken(ctx, tx, deviceToken)
	if err != nil {
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, usecase.Log).Error("Failed to commit register device transaction", zap.Error(err))
		return err
	}
	return nil
}

// UnregisterDevice deletes the caller's token on logout. Must run before clearing the FE's
// stored access token (the delete call needs auth). Best-effort on the FE side.
func (usecase *NotificationUsecase) UnregisterDevice(ctx context.Context, userId string, request model.DeviceTokenDeleteRequest) error {
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-usecase").Start(ctx, "usecase.UnregisterDevice")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	v := util.NewValidator()
	v.String("token", request.Token).Required()
	if err = v.Validate(); err != nil {
		return err
	}

	err = usecase.NotificationRepository.DeleteDeviceToken(ctx, userId, request.Token)
	if err != nil {
		return err
	}
	return nil
}

// TestSend pushes a fixed test notification to all of the caller's own devices — manual proof
// the FCM pipe works end-to-end. FCM failure = log only (not a user error). Cleans up any
// tokens FCM reports invalid.
func (usecase *NotificationUsecase) TestSend(ctx context.Context, userId string) error {
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-usecase").Start(ctx, "usecase.TestSend")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	tokens, err := usecase.NotificationRepository.ListTokensByUserId(ctx, userId)
	if err != nil {
		return err
	}

	if len(tokens) == 0 {
		err = &model.NotFoundError{
			Code:    constant.ERR_NOT_FOUND_CODE,
			Message: "No device registered for this user",
			Param:   "token",
		}
		return err
	}

	message := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: "Virdan",
			Body:  "Test notification berhasil.",
		},
		Data:    map[string]string{"type": "test"},
		Android: &messaging.AndroidConfig{Priority: "high"},
	}

	response, fcmErr := usecase.FCMClient.SendEachForMulticast(ctx, message)
	if fcmErr != nil {
		util.GetLoggerWithTraceContext(ctx, usecase.Log).Error("Failed to send FCM multicast", zap.Error(fcmErr))
		return nil
	}

	var invalidTokens []string
	for i, result := range response.Responses {
		if !result.Success && (messaging.IsUnregistered(result.Error) || messaging.IsInvalidArgument(result.Error)) {
			invalidTokens = append(invalidTokens, tokens[i])
		}
	}

	if len(invalidTokens) > 0 {
		if deleteErr := usecase.NotificationRepository.DeleteInvalidTokens(ctx, invalidTokens); deleteErr != nil {
			util.GetLoggerWithTraceContext(ctx, usecase.Log).Warn("Failed to delete invalid tokens", zap.Error(deleteErr))
		}
	}

	return nil
}
```

#### 5.5.2 `Notify` — goroutine fire-and-forget (Fase 2)

```go
// Notify delivers notifications for a batch of domain events. Fire-and-forget goroutine:
// called after the action commits, it must never block or fail the user's request (a like
// still succeeds if the push fails). The request ctx is cancelled when the HTTP response
// returns (before the push finishes), so the goroutine uses a fresh Background ctx — but it
// CARRIES the request's span context, so its span is a CHILD in the SAME trace (one trace =
// like request → Notify), not a disconnected new root. recover() keeps a goroutine panic from
// crashing the process. IG model: the row is ALWAYS persisted (feed archive); the per-type
// preference gates only the PUSH. Per recipient it dedups to the highest-priority event
// (mention>reply>comment>like). Every error is logged with trace context; one recipient's
// failure continues to the next. Self-notif (actor==recipient) is filtered by the caller.
// (Fase 3: when this moves to Redis Streams, switch to span LINKS + propagator inject/extract
// across the producer/consumer boundary instead of a detached child span.)
func (usecase *NotificationUsecase) Notify(ctx context.Context, events []model.NotificationEvent) {
	// Capture the request's span context BEFORE spawning — the request ctx is about to be
	// cancelled, but its SpanContext (trace_id + span_id) is immutable and safe to carry.
	parentSpanCtx := trace.SpanContextFromContext(ctx)

	go func() {
		// Fresh Background ctx (own lifecycle, not cancelled with the request) carrying the parent
		// trace, so the span below is a child of the request span — same trace_id.
		backgroundCtx := trace.ContextWithSpanContext(context.Background(), parentSpanCtx)
		backgroundCtx, cancel := context.WithTimeout(backgroundCtx, 30*time.Second)
		defer cancel()

		serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
		backgroundCtx, span := otel.Tracer(serviceName + "-usecase").Start(backgroundCtx, "usecase.Notify")
		defer span.End()

		logger := util.GetLoggerWithTraceContext(backgroundCtx, usecase.Log)

		// recover() is mandatory: a panic in a goroutine is NOT caught by the HTTP recover
		// middleware (it only wraps the request handler) — an unrecovered goroutine panic crashes
		// the whole process. Registered after logger so it can log; runs first on panic (LIFO).
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic in Notify goroutine", zap.Any("recover", recovered))
			}
		}()

		// 1. Dedup per recipient — priority: mention=4 > reply=3 > comment=2 > like=1
		priority := map[string]int{"like": 1, "comment": 2, "reply": 3, "mention": 4}
		deduped := make(map[string]model.NotificationEvent, len(events))
		for _, event := range events {
			existing, exists := deduped[event.RecipientUserId]
			if !exists || priority[event.Type] > priority[existing.Type] {
				deduped[event.RecipientUserId] = event
			}
		}

		minioFullUrl := fmt.Sprintf("%s%s/%s",
			usecase.Config.String("MINIO_HTTP"),
			usecase.Config.String("MINIO_URL"),
			usecase.Config.String("MINIO_BUCKET_NAME"),
		)

		// Single err reused across the loop (codebase convention). Each error is handled inline
		// (log + continue) — a goroutine can't propagate it to a caller.
		var err error
		for _, event := range deduped {
			// 2. Persist the row — ALWAYS (IG model: feed archive complete regardless of push pref).
			now := time.Now()
			postId := &event.PostId
			notification := model.Notification{
				Id:              uuid.New().String(),
				RecipientUserId: event.RecipientUserId,
				ActorUserId:     event.ActorUserId,
				ActorProfileId:  event.ActorProfileId,
				Type:            event.Type,
				ServerId:        event.ServerId,
				PostId:          postId,
				CommentId:       event.CommentId,
				CreatedAt:       now,
				UpdatedAt:       now,
				CreatedBy:       event.ActorUserId,
				UpdatedBy:       event.ActorUserId,
			}
			if err = usecase.NotificationRepository.InsertNotification(backgroundCtx, notification); err != nil {
				logger.Error("notif: failed to insert row, skipping recipient",
					zap.String("recipient_user_id", event.RecipientUserId), zap.Error(err))
				continue
			}

			// 3. Push path. No registered device → nothing to push (row already in feed).
			var tokens []string
			if tokens, err = usecase.NotificationRepository.ListTokensByUserId(backgroundCtx, event.RecipientUserId); err != nil {
				logger.Error("notif: failed to list device tokens, skipping push",
					zap.String("recipient_user_id", event.RecipientUserId), zap.Error(err))
				continue
			}
			if len(tokens) == 0 {
				continue
			}

			// 4. Per-type PUSH preference (IG model: gates push only, not the row). Fail-closed:
			//    read error → skip push (row already saved; don't risk pushing to an opt-out user).
			var prefs model.NotificationPrefs
			if prefs, err = usecase.NotificationRepository.GetUserNotificationPrefs(backgroundCtx, event.RecipientUserId); err != nil {
				logger.Error("notif: failed to read prefs, skipping push (fail-closed)",
					zap.String("recipient_user_id", event.RecipientUserId), zap.Error(err))
				continue
			}
			if !pushEnabledForType(prefs, event.Type) {
				continue
			}

			// 5. Actor's per-server username for the push title. Profile is FK-guaranteed, so a
			//    miss is a real anomaly → log + skip push.
			var actorUsername string
			if actorUsername, _, err = usecase.NotificationRepository.GetActorUsernameAndAvatar(backgroundCtx, event.ActorProfileId, minioFullUrl); err != nil {
				logger.Error("notif: failed to resolve actor username, skipping push",
					zap.String("actor_profile_id", event.ActorProfileId), zap.Error(err))
				continue
			}

			// 6. Send push.
			usecase.sendPush(backgroundCtx, tokens, actorUsername, event)
		}
	}()
}

// pushEnabledForType maps a notification type to its per-type push toggle. mention has no toggle
// in Fase 2 prefs → defaults to push-on (add a mention toggle to NotificationPrefs when needed).
func pushEnabledForType(prefs model.NotificationPrefs, notifType string) bool {
	switch notifType {
	case "like":
		return prefs.NotifLike
	case "comment":
		return prefs.NotifComment
	case "reply":
		return prefs.NotifReply
	default:
		return true
	}
}

// sendPush builds the hybrid notification+data FCM message (title = actor username, body per
// type, data = {type, serverId, postId, commentId?} for deep-link) and multicasts it, then
// removes any tokens FCM reports dead.
func (usecase *NotificationUsecase) sendPush(ctx context.Context, tokens []string, actorUsername string, event model.NotificationEvent) {
	body := notifBodyForType(event.Type)

	data := map[string]string{
		"type":     event.Type,
		"serverId": event.ServerId,
		"postId":   event.PostId,
	}
	if event.CommentId != nil {
		data["commentId"] = *event.CommentId
	}

	message := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: actorUsername,
			Body:  body,
		},
		Data:    data,
		Android: &messaging.AndroidConfig{Priority: "high"},
	}

	response, fcmErr := usecase.FCMClient.SendEachForMulticast(ctx, message)
	if fcmErr != nil {
		util.GetLoggerWithTraceContext(ctx, usecase.Log).Error("Failed to send FCM push", zap.Error(fcmErr))
		return
	}

	var invalidTokens []string
	for i, result := range response.Responses {
		if !result.Success && (messaging.IsUnregistered(result.Error) || messaging.IsInvalidArgument(result.Error)) {
			invalidTokens = append(invalidTokens, tokens[i])
		}
	}

	if len(invalidTokens) > 0 {
		if deleteErr := usecase.NotificationRepository.DeleteInvalidTokens(ctx, invalidTokens); deleteErr != nil {
			util.GetLoggerWithTraceContext(ctx, usecase.Log).Warn("Failed to delete invalid tokens after push", zap.Error(deleteErr))
		}
	}
}

// notifBodyForType maps a notification type to its Indonesian push body sentence (paired with
// the actor username as title — IG-style "username menyukai postinganmu.").
func notifBodyForType(notifType string) string {
	switch notifType {
	case "like":
		return "menyukai postinganmu."
	case "comment":
		return "mengomentari postinganmu."
	case "reply":
		return "membalas komentarmu."
	default:
		return "berinteraksi denganmu."
	}
}
```

#### 5.5.3 Feed methods (Fase 2)

```go
// GetFeed returns the paginated notification feed. Clamps limit to [DEFAULT_LIMIT, MAX_LIMIT]
// and decodes the opaque cursor (bad cursor → 400, not a silent reset).
func (usecase *NotificationUsecase) GetFeed(ctx context.Context, userId string, cursorStr string, limit int) (model.NotificationListResponse, error) {
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-usecase").Start(ctx, "usecase.GetNotificationFeed")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	if limit <= 0 {
		limit = constant.DEFAULT_LIMIT
	}
	if limit > constant.MAX_LIMIT {
		limit = constant.MAX_LIMIT
	}

	var cursor *model.NotificationCursor
	if cursorStr != "" {
		cursor, err = util.DecodeCursor[model.NotificationCursor](cursorStr)
		if err != nil {
			err = &model.BadRequestError{Code: constant.ERR_VALIDATION_CODE, Message: "Invalid cursor", Param: "cursor"}
			return model.NotificationListResponse{}, err
		}
	}

	minioFullUrl := fmt.Sprintf("%s%s/%s",
		usecase.Config.String("MINIO_HTTP"),
		usecase.Config.String("MINIO_URL"),
		usecase.Config.String("MINIO_BUCKET_NAME"),
	)

	// Fetch limit+1: if we get more than limit, there IS a next page. Encode the cursor here
	// (usecase), not in the repo — matches GetServerPosts. Returning exactly `limit` rows with
	// no extra = last page, nextCursor stays empty.
	items, err := usecase.NotificationRepository.ListByRecipient(ctx, userId, cursor, limit+1, minioFullUrl)
	if err != nil {
		return model.NotificationListResponse{}, err
	}

	response := model.NotificationListResponse{Data: []model.NotificationResponse{}}
	if len(items) > limit {
		response.Data = items[:limit]
		last := items[limit-1]
		response.Page.NextCursor = util.EncodeCursor(model.NotificationCursor{
			CreatedAt: last.CreatedAt,
			Id:        last.Id,
		})
	} else {
		response.Data = items
	}

	return response, nil
}

// MarkRead validates the id, generates the read timestamp in Go (not SQL NOW()), and delegates
// to the scoped repo update. Called when the user taps a notification.
func (usecase *NotificationUsecase) MarkRead(ctx context.Context, userId string, notifId string) error {
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-usecase").Start(ctx, "usecase.MarkNotificationRead")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("user.id", userId),
		attribute.String("notification.id", notifId),
	)

	v := util.NewValidator()
	v.UUID("id", notifId)
	if err = v.Validate(); err != nil {
		return err
	}

	now := time.Now()
	err = usecase.NotificationRepository.MarkRead(ctx, userId, notifId, now)
	return err
}

// GetUnreadCount returns the unread badge count for the Activity tab. FE polls this (no
// WebSocket) and refreshes it when the tab is opened.
func (usecase *NotificationUsecase) GetUnreadCount(ctx context.Context, userId string) (model.UnreadCountResponse, error) {
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-usecase").Start(ctx, "usecase.GetUnreadNotificationCount")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	count, err := usecase.NotificationRepository.CountUnread(ctx, userId)
	if err != nil {
		return model.UnreadCountResponse{}, err
	}
	return model.UnreadCountResponse{Count: count}, nil
}
```

#### 5.5.4 `UserUsecase.UpdateNotificationPreferences` (Fase 2)

```go
// UpdateNotificationPreferences persists the per-type toggles to users.settings, generating
// updated_at here. Lives in UserUsecase (preferences are a user-domain concern, stored on the
// user row), not NotificationUsecase.
func (usecase *UserUsecase) UpdateNotificationPreferences(ctx fiber.Ctx, userId string, request model.UpdateNotificationPreferencesRequest) error {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.UpdateNotificationPreferences")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	prefs := model.NotificationPrefs{
		NotifLike:    request.NotifLike,
		NotifComment: request.NotifComment,
		NotifReply:   request.NotifReply,
	}

	now := time.Now()
	err = usecase.UserRepository.UpdateNotificationPrefs(ctxContext, userId, prefs, now)
	return err
}
```

### 5.6 Domain triggers di `post_usecase.go`

**Struct tambahan**:
```go
type PostUsecase struct {
    PostRepository      *repository.PostRepository
    ServerRepository    *repository.ServerRepository
    ProfileRepository   *repository.ProfileRepository   // tambah
    NotificationUsecase *NotificationUsecase             // tambah
    DB                  *pgxpool.Pool
    Log                 *zap.Logger
    Config              *koanf.Koanf
}
```

**`LikePost` — setelah `CreatePostLikeIdempotent`**:
```go
inserted, err := usecase.PostRepository.CreatePostLikeIdempotent(ctxContext, postLike)
if err != nil { return model.PostLikeResponse{}, err }

// Notify only on a NEW like (inserted=true) — repeated like/unlike/like must not re-notify.
// Trigger runs AFTER the like is persisted: a failed insert returns above, so no notif for a
// failed action.
if inserted {
    // Notif is best-effort: any resolution error logs and is skipped — the like itself already
    // succeeded and must not be failed by a notification problem.
    postAuthorId, authorErr := usecase.PostRepository.GetPostAuthorId(ctxContext, postIdParam)
    if authorErr != nil {
        util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Warn("notif skipped: failed to resolve post author", zap.Error(authorErr))
    } else if postAuthorId != userId { // self-notif guard: don't notify yourself
        // NoTx variant: trigger runs outside the (already-committed) like. Resolves the actor's
        // per-server identity for notifications.actor_profile_id. found=false would yield an
        // empty id → FK violation on insert, so skip if unresolved.
        actorProfileId, found, profErr := usecase.ProfileRepository.TryGetServerMemberProfileIdNoTx(ctxContext, serverId, userId)
        if profErr != nil || !found {
            util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Warn("notif skipped: actor profile unresolved", zap.Error(profErr))
        } else {
            usecase.NotificationUsecase.Notify(ctxContext, []model.NotificationEvent{{
                Type: "like", RecipientUserId: postAuthorId,
                ActorUserId: userId, ActorProfileId: actorProfileId,
                ServerId: serverId, PostId: postIdParam,
            }})
        }
    }
}
```

**`CreateComment` — setelah repo `CreateComment` berhasil**:
```go
// Notif is best-effort: resolution errors log + skip, never fail the comment (already saved).
// Actor's per-server identity for actor_profile_id. NoTx: runs after the comment commits.
// found=false → empty id → FK violation, so skip the whole notif if unresolved.
actorProfileId, found, profErr := usecase.ProfileRepository.TryGetServerMemberProfileIdNoTx(ctxContext, serverId, userId)
if profErr != nil || !found {
    util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Warn("notif skipped: actor profile unresolved", zap.Error(profErr))
} else {
    // Slice (not single event) so Fase 2.5 can append mention events from the same comment and
    // let Notify's dedup engine collapse per recipient (comment + mention to same user = 1 notif).
    var notifEvents []model.NotificationEvent

    if payload.ParentId == nil {
        // Top-level comment → notify the POST author. Self-notif guard below.
        postAuthorId, authorErr := usecase.PostRepository.GetPostAuthorId(ctxContext, postIdParam)
        if authorErr != nil {
            util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Warn("notif skipped: failed to resolve post author", zap.Error(authorErr))
        } else if postAuthorId != userId {
            notifEvents = append(notifEvents, model.NotificationEvent{
                Type: "comment", RecipientUserId: postAuthorId,
                ActorUserId: userId, ActorProfileId: actorProfileId,
                ServerId: serverId, PostId: postIdParam,
            })
        }
    } else {
        // Reply → notify the PARENT COMMENT author, NOT the post owner (post owner isn't spammed
        // for every reply on their post — locked decision). Self-notif guard below.
        parentAuthorId, parentErr := usecase.PostRepository.GetCommentAuthorId(ctxContext, *payload.ParentId)
        if parentErr != nil {
            util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Warn("notif skipped: failed to resolve parent comment author", zap.Error(parentErr))
        } else if parentAuthorId != userId {
            notifEvents = append(notifEvents, model.NotificationEvent{
                Type: "reply", RecipientUserId: parentAuthorId,
                ActorUserId: userId, ActorProfileId: actorProfileId,
                ServerId: serverId, PostId: postIdParam, CommentId: &commentId,
            })
        }
    }

    if len(notifEvents) > 0 {
        usecase.NotificationUsecase.Notify(ctxContext, notifEvents)
    }
}
```

### 5.7 Mention (Fase 2.5) — full

Mention `@username` di **caption post** + **body comment**. Server-side authoritative: client kirim teks mentah, BE yang parse + resolve (jangan percaya daftar mention dari client). Hanya member server yang sama yang bisa di-mention; username tidak ketemu → diabaikan diam.

> **Kenapa butuh tabel `mentions` (bukan cukup `notifications`):**
> 1. **Edit re-sync** — edit caption butuh tau set mention LAMA buat di-diff vs baru (notif yang baru, hapus yang hilang, jangan re-notif yang tetap). Tanpa tabel ga ada record lama.
> 2. **Filter tab "Mentions"** di feed (UI ada tab All / Mentions).
> 3. **Mention bisa ke-dedup hilang** — comment + mention ke orang sama → 1 notif "mention", tapi mention-nya tetap terjadi (perlu dicatat buat render + edit-resync).
> 4. **Render highlight + tap** — FE perlu tau mana `@` yang valid member vs teks biasa.
> `notifications` = delivery (siapa dapat notif). `mentions` = fakta domain (siapa di-mention di mana). Beda concern.
>
> **2 endpoint, 2 momen berbeda:**
> - `SearchByPrefix` (autocomplete) — pas user **NGETIK** `@an` di composer (post belum jadi), butuh daftar saran prefix.
> - `ResolveOne` (resolve-on-tap) — pas user **TAP** `@andi` di post yang udah jadi. FE cuma pegang string `"andi"`, ga punya userId/profileId → tanya BE "username ini siapa?" → ketemu: buka profil; 404: ilustrasi "user tidak ada".

#### 5.7.1 Model — `internal/model/mention.go` (NEW)

```go
package model

import "time"

type Mention struct {
	Id                 string
	ServerId           string
	PostId             *string // XOR dengan CommentId (CHECK constraint)
	CommentId          *string
	MentionedUserId    string
	MentionedProfileId string
	CreatedAt          time.Time
	CreatedBy          string
}

// ProfileRef = hasil resolve username → identitas per-server. Dipakai mention resolve +
// autocomplete + resolve-on-click.
type ProfileRef struct {
	ProfileId string  `json:"profileId"`
	UserId    string  `json:"userId"`
	Username  string  `json:"username"`
	Nickname  string  `json:"nickname"`
	AvatarUrl *string `json:"avatarUrl"`
}

type MemberSearchResponse struct {
	Data []ProfileRef `json:"data"`
}
```

#### 5.7.2 Parser — `internal/util/mention.go` (NEW) + regex di `regex_patterns.go`

```go
// regex_patterns.go — pre-compiled (package var), bukan compile per-call.
var mentionRegex = regexp.MustCompile(`@([a-z0-9_.]{1,22})`)
```

```go
// ParseMentions extracts unique @username handles from raw text. Server-authoritative — the
// caller passes the raw post caption / comment body, never a client-supplied list. Lowercases
// first (usernames are stored lowercase), strips a trailing dot (so "@andi." → "andi"), dedups,
// and excludes the author's own username (no self-mention notif). Returns at most `max` handles
// (cap to bound the resolve query + notif fan-out).
func ParseMentions(text string, selfUsername string, max int) []string {
	lowered := strings.ToLower(text)
	matches := mentionRegex.FindAllStringSubmatch(lowered, -1)

	seen := make(map[string]struct{}, len(matches))
	var usernames []string
	for _, m := range matches {
		handle := strings.TrimRight(m[1], ".")
		if handle == "" || handle == selfUsername {
			continue
		}
		if _, dup := seen[handle]; dup {
			continue
		}
		seen[handle] = struct{}{}
		usernames = append(usernames, handle)
		if len(usernames) >= max {
			break
		}
	}
	return usernames
}
```

#### 5.7.3 Repository — `internal/repository/mention_repository.go` (NEW)

Struct + constructor pola sama `NotificationRepository` (`Log`, `Config`, `DB`). Method:

```go
// ResolveUsernames returns the ProfileRef for each username that is a member of this server.
// Usernames not in the server (non-member / typo) are simply absent from the result — the
// caller treats "not returned" as "ignored", no error. ANY($2) does the whole batch in one
// query. Runs in the same tx as the post/comment insert so mentions are atomic with the source.
func (repository *MentionRepository) ResolveUsernames(ctx context.Context, tx pgx.Tx, serverId string, usernames []string) ([]model.ProfileRef, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.ResolveUsernames")
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

	query := `SELECT id, user_id, username, nickname
	          FROM server_member_profiles
	          WHERE server_id = $1 AND username = ANY($2)`
	rows, err := tx.Query(ctx, query, serverId, usernames)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to resolve usernames", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var refs []model.ProfileRef
	for rows.Next() {
		var ref model.ProfileRef
		if err = rows.Scan(&ref.ProfileId, &ref.UserId, &ref.Username, &ref.Nickname); err != nil {
			util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan profile ref", zap.Error(err))
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// InsertBatch inserts all resolved mentions in one round-trip (pgx.Batch), inside the
// post/comment tx. Idempotent against the UNIQUE (post_id|comment_id, mentioned_user_id)
// indexes via ON CONFLICT DO NOTHING — re-running an edit that keeps a mention won't error.
func (repository *MentionRepository) InsertBatch(ctx context.Context, tx pgx.Tx, mentions []model.Mention) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.InsertMentionBatch")
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

	query := `INSERT INTO mentions
	          (id, server_id, post_id, comment_id, mentioned_user_id, mentioned_profile_id, created_at, created_by)
	          VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	          ON CONFLICT DO NOTHING`

	batch := &pgx.Batch{}
	for _, mention := range mentions {
		batch.Queue(query, mention.Id, mention.ServerId, mention.PostId, mention.CommentId,
			mention.MentionedUserId, mention.MentionedProfileId, mention.CreatedAt, mention.CreatedBy)
	}

	results := tx.SendBatch(ctx, batch)
	defer results.Close()
	for range mentions {
		if _, err = results.Exec(); err != nil {
			util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to insert mention batch", zap.Error(err))
			return err
		}
	}
	return nil
}

// ListMentionedUserIdsBySource returns the user ids already mentioned on a post (used on edit to
// diff old vs new). Pass postId (comment can't be edited, so comment path never re-syncs).
func (repository *MentionRepository) ListMentionedUserIdsByPost(ctx context.Context, tx pgx.Tx, postId string) ([]string, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.ListMentionedUserIdsByPost")
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

	query := `SELECT mentioned_user_id FROM mentions WHERE post_id = $1`
	rows, err := tx.Query(ctx, query, postId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to list mentioned user ids", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan mentioned user id", zap.Error(err))
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// DeleteByPostForUsers removes mentions for users no longer mentioned after a post edit. Scoped
// to post_id + the specific users to remove (don't touch mentions that survived the edit).
func (repository *MentionRepository) DeleteByPostForUsers(ctx context.Context, tx pgx.Tx, postId string, removedUserIds []string) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.DeleteMentionsByPostForUsers")
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

	query := `DELETE FROM mentions WHERE post_id = $1 AND mentioned_user_id = ANY($2)`
	_, err = tx.Exec(ctx, query, postId, removedUserIds)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to delete mentions for users", zap.Error(err))
		return err
	}
	return nil
}

// SearchByPrefix powers @-autocomplete: members of this server whose username starts with the
// typed prefix. Uses idx_server_member_profiles_pk_username (varchar_pattern_ops) — username is
// always lowercase, so LIKE 'prefix%' hits the index. Caller (controller) must verify the
// requester is a member first (prevents enumerating a private server's roster).
func (repository *MentionRepository) SearchByPrefix(ctx context.Context, serverId string, prefix string, limit int, minioFullUrl string) ([]model.ProfileRef, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.SearchMembersByPrefix")
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

	query := `SELECT smp.id, smp.user_id, smp.username, smp.nickname, pai.object_key
	          FROM server_member_profiles smp
	          LEFT JOIN profile_avatar_images pai ON pai.id = smp.avatar_image_id
	          WHERE smp.server_id = $1 AND smp.username LIKE $2
	          ORDER BY smp.username
	          LIMIT $3`
	rows, err := repository.DB.Query(ctx, query, serverId, prefix+"%", limit)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to search members by prefix", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var refs []model.ProfileRef
	for rows.Next() {
		var ref model.ProfileRef
		var objectKey *string
		if err = rows.Scan(&ref.ProfileId, &ref.UserId, &ref.Username, &ref.Nickname, &objectKey); err != nil {
			util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to scan member ref", zap.Error(err))
			return nil, err
		}
		if objectKey != nil {
			formatted := fmt.Sprintf("%s/%s", minioFullUrl, *objectKey)
			ref.AvatarUrl = &formatted
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// ResolveOne resolves a single @username on tap (open profile). NotFoundError → FE shows the
// "user tidak ada" illustration. Member-guard same as SearchByPrefix.
func (repository *MentionRepository) ResolveOne(ctx context.Context, serverId string, username string, minioFullUrl string) (model.ProfileRef, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName + "-repository").Start(ctx, "repository.ResolveOneMember")
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

	query := `SELECT smp.id, smp.user_id, smp.username, smp.nickname, pai.object_key
	          FROM server_member_profiles smp
	          LEFT JOIN profile_avatar_images pai ON pai.id = smp.avatar_image_id
	          WHERE smp.server_id = $1 AND smp.username = $2 LIMIT 1`

	var ref model.ProfileRef
	var objectKey *string
	err = repository.DB.QueryRow(ctx, query, serverId, username).Scan(
		&ref.ProfileId, &ref.UserId, &ref.Username, &ref.Nickname, &objectKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &model.NotFoundError{Code: constant.ERR_NOT_FOUND_CODE, Message: "User not found in this server", Param: "username"}
			return model.ProfileRef{}, err
		}
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("Failed to resolve username", zap.Error(err))
		return model.ProfileRef{}, err
	}
	if objectKey != nil {
		formatted := fmt.Sprintf("%s/%s", minioFullUrl, *objectKey)
		ref.AvatarUrl = &formatted
	}
	return ref, nil
}
```

#### 5.7.4 Integrasi di `post_usecase.go`

**CreatePost / CreateComment** — di dalam tx yang sama dengan insert post/comment (mention atomic dengan sumbernya):

```go
// di dalam tx, setelah insert post (atau comment):
// authorUsername = username actor di server ini (sudah ada dari resolve actorProfile, atau
// query server_member_profiles). Dipakai exclude-self di parser.
usernames := util.ParseMentions(payload.Caption, authorUsername, constant.MAX_MENTIONS_PER_POST)
if len(usernames) > 0 {
    refs, resolveErr := usecase.MentionRepository.ResolveUsernames(ctxContext, tx, serverId, usernames)
    if resolveErr != nil {
        return ..., resolveErr // mention gagal resolve = rollback (atomic dengan post)
    }

    now := time.Now()
    var mentions []model.Mention
    for _, ref := range refs {
        mentions = append(mentions, model.Mention{
            Id: uuid.New().String(), ServerId: serverId,
            PostId: &postId, CommentId: nil, // comment path: PostId nil, CommentId: &commentId
            MentionedUserId: ref.UserId, MentionedProfileId: ref.ProfileId,
            CreatedAt: now, CreatedBy: userId,
        })
    }
    if len(mentions) > 0 {
        if insErr := usecase.MentionRepository.InsertBatch(ctxContext, tx, mentions); insErr != nil {
            return ..., insErr
        }
    }
    // simpan `refs` untuk dipakai bikin NotificationEvent SETELAH tx commit
}
```

Setelah `tx.Commit` sukses — gabung mention events ke `Notify` call yang sama (biar dedup engine collapse comment+mention ke recipient sama):
```go
for _, ref := range mentionRefs {
    if ref.UserId == userId { continue } // self-mention guard (parser sudah exclude, belt-and-suspenders)
    notifEvents = append(notifEvents, model.NotificationEvent{
        Type: "mention", RecipientUserId: ref.UserId,
        ActorUserId: userId, ActorProfileId: actorProfileId,
        ServerId: serverId, PostId: postIdParam, CommentId: commentIdPtr, // comment path isi commentId
    })
}
// notifEvents (comment + mention) → satu usecase.NotificationUsecase.Notify(notifEvents)
// dedup priority mention=4 menang → 1 recipient yang di-comment + di-mention dapat 1 notif "mention"
```

**UpdatePost (re-sync)** — di tx update post:
```go
// 1. parse caption baru → resolve → set newUserIds
// 2. oldUserIds := MentionRepository.ListMentionedUserIdsByPost(tx, postId)
// 3. removed := oldUserIds - newUserIds  → DeleteByPostForUsers(tx, postId, removed)
// 4. added := newRefs yang UserId-nya bukan di oldUserIds → InsertBatch(tx, addedMentions)
// 5. notif HANYA untuk `added` (yang sudah pernah di-mention tidak di-notif ulang)
```

#### 5.7.5 Constant baru

```go
const MAX_MENTIONS_PER_POST = 10 // cap fan-out + resolve query size
const MENTION_SEARCH_LIMIT   = 8 // max hasil autocomplete dropdown
```

#### 5.7.6 Endpoint autocomplete + resolve (usecase + controller)

Usecase (`MentionUsecase` baru, atau gabung ke `ServerUsecase` — pilih sesuai preferensi domain). Wajib **member-guard**: requester harus member server (cegah enumerasi roster server private).

```go
// SearchMembers powers @-autocomplete. Guards membership first, then prefix-searches. Empty
// prefix → empty result (no full-roster dump). Requester membership checked via ServerRepository.
func (usecase *MentionUsecase) SearchMembers(ctx fiber.Ctx, serverId string, userId string, prefix string) (model.MemberSearchResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-usecase").Start(ctxContext, "usecase.SearchMembers")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	v := util.NewValidator()
	v.UUID("serverId", serverId)
	if err = v.Validate(); err != nil {
		return model.MemberSearchResponse{}, err
	}

	// Member-guard: requester wajib member server ini.
	memberCount, err := usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if err != nil {
		return model.MemberSearchResponse{}, err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return model.MemberSearchResponse{}, err
	}

	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" {
		return model.MemberSearchResponse{Data: []model.ProfileRef{}}, nil
	}

	minioFullUrl := fmt.Sprintf("%s%s/%s", usecase.Config.String("MINIO_HTTP"), usecase.Config.String("MINIO_URL"), usecase.Config.String("MINIO_BUCKET_NAME"))
	refs, err := usecase.MentionRepository.SearchByPrefix(ctxContext, serverId, prefix, constant.MENTION_SEARCH_LIMIT, minioFullUrl)
	if err != nil {
		return model.MemberSearchResponse{}, err
	}
	if refs == nil {
		refs = []model.ProfileRef{}
	}
	return model.MemberSearchResponse{Data: refs}, nil
}

// ResolveMember resolves one @username on tap. Same member-guard. NotFoundError → FE shows
// "user tidak ada" illustration.
func (usecase *MentionUsecase) ResolveMember(ctx fiber.Ctx, serverId string, userId string, username string) (model.ProfileRef, error) {
	// ... span + validate serverId UUID + CheckServerMember guard (sama SearchMembers) ...
	// username := strings.ToLower(strings.TrimSpace(username))
	// return usecase.MentionRepository.ResolveOne(ctxContext, serverId, username, minioFullUrl)
}
```

Controller (`server_controller.go` atau `notification_controller.go`) — pola sama handler existing:
```go
// GET /api/servers/:serverId/members?q=&limit=
func (controller *ServerController) SearchMembers(ctx fiber.Ctx) error {
	// span → ctx.Locals("userId") → serverId := ctx.Params("serverId") → q := ctx.Query("q")
	// response, err := controller.MentionUsecase.SearchMembers(ctx, serverId, userId, q)
	// err → SendError; else SendSuccessResponseWithData(response)
}

// GET /api/servers/:serverId/members/by-username/:username
func (controller *ServerController) ResolveMember(ctx fiber.Ctx) error {
	// span → userId, serverId, username := ctx.Params(...) → ResolveMember → Send...
}
```

### 5.8 Endpoints

| Method | Path | Fase | Status | Deskripsi |
|---|---|---|---|---|
| POST | `/api/devices` | 1 | DONE | Register device token |
| DELETE | `/api/devices` | 1 | DONE | Hapus token saat logout |
| POST | `/api/notifications/test-send` | 1 | DONE | Test push ke semua device sendiri |
| GET | `/api/notifications/unread-count` | 2 | TODO | Badge count (route SEBELUM `/:id`) |
| GET | `/api/notifications` | 2 | TODO | Feed cursor pagination |
| POST | `/api/notifications/:id/read` | 2 | TODO | Mark read |
| PUT | `/api/users/me/notification-preferences` | 2 | TODO | Toggle notif per type |
| GET | `/api/servers/:serverId/members?q=&limit=` | 2.5 | TODO | Autocomplete mention |
| GET | `/api/servers/:serverId/members/by-username/:username` | 2.5 | TODO | Resolve mention on click |

> **Route order penting**: `GET /notifications/unread-count` harus didaftarkan **sebelum** `POST /notifications/:id/read` di Fiber — kalau terbalik, Fiber match `unread-count` sebagai `:id`.

### 5.9 Security

- Token register/delete: scoped ke `ctx.Locals("userId")`. User cuma bisa daftar/hapus token miliknya.
- `DeleteDeviceToken` query: `WHERE user_id = $1 AND token = $2` — tidak bisa hapus token orang lain.
- Member search & resolve: requester wajib member server (cegah enumerasi member server private).
- Service account JSON: env only (`FIREBASE_SERVICE_ACCOUNT_BASE64_JSON`), tidak pernah di-log, tidak di-commit.
- Test-send: protected + hanya ke device sendiri.
- Mention parse server-side — tidak percaya list mention dari client.
- FCM token tidak pernah di-log atau masuk OTel span attribute (PII).
- `GetUserNotificationPrefs` dan `UpdateNotificationPrefs` scoped ke `userId` dari auth middleware — tidak bisa baca/ubah pref user lain.

---

## 6. Frontend design (`virdanmobileapp-flutter/`)

### 6.1 Dependencies (`pubspec.yaml`)

Tambah (cek versi terbaru via context7/pub.dev saat implement): `firebase_core`, `firebase_messaging`, `flutter_local_notifications`.

### 6.2 Bootstrap

- `main.dart`: `await Firebase.initializeApp()` sebelum `runApp`. Daftar background handler **top-level** `@pragma('vm:entry-point') Future<void> _fcmBackgroundHandler(RemoteMessage m)`.
- `android/app/build.gradle.kts`: apply Google Services plugin. `android/build.gradle.kts`: classpath. `AndroidManifest.xml`: permission `POST_NOTIFICATIONS` (Android 13+), default notification channel meta.

### 6.3 Folder baru `lib/core/notifications/`

- `fcm_service.dart` — `requestPermission()`, `getToken()`, listen `onTokenRefresh`, `onMessage` (foreground → local notif), `onMessageOpenedApp` + `getInitialMessage` (tap → deep-link). iOS: `getAPNSToken()` dulu (disiapkan, walau iOS ditunda).
- `local_notifications.dart` — setup `flutter_local_notifications` channel + display saat foreground.
- `notification_api.dart` — dio: register/delete device token, list feed, mark read, unread-count.
- Riverpod providers untuk wiring service.

### 6.4 Token lifecycle

- `core/storage/secure_storage.dart`: simpan FCM token terakhir (untuk tau perlu re-register / hapus saat logout).
- `features/auth/data/auth_repository.dart`:
  - setelah `login()` sukses → `getToken()` + register ke backend.
  - `logout()` → `DELETE /api/devices` (token ini) **sebelum** `_storage.clear()`.
- `onTokenRefresh` → register token baru.

### 6.5 Feed (ganti mock)

- `features/notifications/presentation/notifications_page.dart`: ganti `mockNotifications` → fetch `GET /api/notifications` (Riverpod + dio), pull-to-refresh, mark read on tap.
- **Mark-read guard (FE)**: saat tap notif, cek `readAt` dulu — **kalau `readAt != null` (sudah dibaca), JANGAN panggil `POST /:id/read`**. Cuma panggil API kalau masih unread. Cegah spam request mubazir (BE guard `read_at IS NULL` cuma cegah overwrite, tapi request-nya tetap kebuang kalau FE ga cek). Setelah sukses, update `readAt` lokal + decrement badge.
- Unread badge di tab bar (Fase 2) dari `unread-count`.

### 6.6 Mention (Fase 2.5)

- **Autocomplete**: di composer post + comment, detect `@` + token, debounce, query member search, tampilkan overlay list, insert `@username ` saat dipilih.
- **Highlight**: parse teks → `TextSpan`, warnai semua `@token` (ungu/biru), `TapGestureRecognizer` per span.
- **Tap**: resolve via `by-username` endpoint → ada: navigate profile page; 404: tampilkan "user tidak ada" + illustration.

### 6.7 Deep-link

- `core/router/app_router.dart`: dari `data` payload (`type`, `serverId`, `postId`, `commentId`) → navigate ke post/comment terkait.

---

## 7. Data flow contoh

**Like**: A like post B → commit like → `Notify([{like, recipient:B, actor:A, profileA@server, post}])` → goroutine: insert notif, ambil token B (2 HP), `SendEachForMulticast`, push muncul di device B → tap → buka post.

**Comment + mention owner**: A comment `"@B halo"` di post B → events `[{comment, recipient:B}, {mention, recipient:B}]` → dedup per recipient B → **1 notif: mention**.

**Reply**: B reply comment A di post C → event `{reply, recipient:A}` saja. C (post owner) tidak dapat notif.

---

## 8. Error handling

- Notifier goroutine: `recover()` panic, OTel `RecordErrorTelemetry`, **tidak** ganggu request path. Gagal kirim FCM = log only.
- Token register idempotent (upsert). Token sudah di user lain → reassign `user_id`.
- Cleanup token invalid wajib tiap kirim (`IsUnregistered`/`IsInvalidArgument`).
- Mention ke non-member / username tidak ada → skip diam (no error).
- Error response ikut `ApiError` existing.

---

## 9. Testing

- Backend: integration test (testcontainers, pola existing) — DB operations saja: device token upsert/delete/reassign, feed list+pagination, mark read, unread-count, mention resolve+insert, member search membership-guard, preferences toggle. **FCM TIDAK di-mock** (tidak ada `Notifier` interface) — method yang hit FCM (`TestSend`, `Notify`) di-skip di integration test, verifikasi manual via test-send (2 HP). DB side dari trigger (notif row ter-insert) tetap bisa di-test.
- Frontend: unit test `notification_api`, token register/delete logic, mention parser/highlight. E2E delivery manual via test-send (2 HP).

---

## 10. Fase & breakdown

### Fase 0 — Prasyarat (manual, no code)
1. Buat Firebase project (Spark/gratis).
2. Tambah Android app (`com.virdan.virdanmobileapp`), download `google-services.json`.
3. Generate service account JSON (Firebase Console → Project Settings → Service accounts).
4. Set env `FIREBASE_SERVICE_ACCOUNT_JSON`.

### Fase 1 — Plumbing (buktikan pipa E2E) — DONE
- BE: `config/firebase.go` (`NewFCMClient`, base64-only), migration `device_tokens`, `model/device_token.go`, `notification_repository.go` (device token methods), `NotificationUsecase` (RegisterDevice/UnregisterDevice/TestSend), `notification_controller.go`, endpoint `POST/DELETE /api/devices` + `POST /api/notifications/test-send`, wire `app.go` + `main.go`.
- FE: deps + `Firebase.initializeApp`, Android config (desugaring + POST_NOTIFICATIONS), `fcm_service.dart`, `notification_api.dart`, permission, token register di login+signup, delete saat logout.
- Acceptance: ✅ test-send dari 1 device muncul push di HP (verified 2026-05-30).

### Fase 2 — Feature notif inti + preferences
- BE: migration `notifications`, `model/notification.go`, `notification_repository.go` (+6 method: Insert/ListByRecipient/MarkRead/CountUnread/GetActorUsernameAndAvatar/GetUserNotificationPrefs), `post_repository.go` (CreatePostLikeIdempotent→bool + GetPostAuthorId + GetCommentAuthorId), `user_repository.go` (UpdateNotificationPrefs), `NotificationUsecase.Notify` (goroutine + dedup + persist + FCM), trigger like/comment/reply di `post_usecase.go`, `UserUsecase.UpdateNotificationPreferences`, endpoint feed + unread-count + mark-read + preferences.
- FE: ganti mock `notifications_page`, unread badge tab bar (`unreadCountProvider`), deep-link tap, notification settings toggle ke backend.
- Acceptance: like/comment/reply → push + muncul di feed; reply tidak notif post owner; self-action skip; toggle off → tidak ada notif.

> Detail full implementation code: `virdan/fase2-implementation.md`.

### Fase 2.5 — Mention
- BE: migration `mentions` + index username, mention repo, parser di create/edit post & create comment, endpoint member search + resolve, audit lowercase username write-path (fix gap kalau ada).
- FE: autocomplete dropdown, highlight `@`, tap-resolve, illustration not-found.
- Acceptance: `@username` di post/comment → notif + tersimpan + highlight + tap ke profil; non-member diabaikan; edit post re-sync.

### Fase 3 — Polish
- Foreground local-notif finalize, deep-link edge case.
- Swap `NotificationUsecase.Notify` goroutine → Redis Streams (no ubah call site di `post_usecase`). Tracing: ganti pola detached-child-span → **span links** + inject trace context ke message stream (propagator) di producer, extract + link di consumer (standard cross-process queue tracing).
- Cron cleanup token stale (pakai `device_tokens.updated_at`, >1 bulan idle).
- iOS (terpisah, butuh Apple Developer + APNs).

> Preferences (toggle) sudah dipindah ke Fase 2.

---

## 11. Referensi

- [Best practices for FCM registration token management](https://firebase.google.com/docs/cloud-messaging/manage-tokens)
- [FCM in Flutter — get started](https://firebase.google.com/docs/cloud-messaging/flutter/get-started)
- [Receive messages in Flutter](https://firebase.google.com/docs/cloud-messaging/flutter/receive)
- [Firebase Admin Go SDK](https://github.com/firebase/firebase-admin-go)
- [FCM Architectural Overview](https://firebase.google.com/docs/cloud-messaging/fcm-architecture)

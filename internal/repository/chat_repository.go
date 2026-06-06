package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

type ChatRepository struct {
	Log    *zap.Logger
	Config *koanf.Koanf
	DB     *pgxpool.Pool
}

func NewChatRepository(log *zap.Logger, config *koanf.Koanf, db *pgxpool.Pool) *ChatRepository {
	return &ChatRepository{Log: log, Config: config, DB: db}
}

// InsertConversationIdempotent inserts a conversation; ON CONFLICT on the canonical
// pair it does nothing. Returns true if a row was actually inserted. SINGLE statement
// — the transaction is owned by the usecase.
func (repository *ChatRepository) InsertConversationIdempotent(ctx context.Context, tx pgx.Tx, c model.DmConversation) (bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.InsertConversationIdempotent")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(attribute.String("db.system", "postgres"), attribute.String("db.operation", "INSERT"))

	query := `INSERT INTO dm_conversations (id, server_id, user_low, user_high, created_at, updated_at, created_by, updated_by)
		VALUES ($1,$2,$3,$4,$5,$5,$6,$6)
		ON CONFLICT (server_id, user_low, user_high) DO NOTHING`
	var tag pgconn.CommandTag
	tag, err = tx.Exec(ctx, query, c.Id, c.ServerId, c.UserLow, c.UserHigh, c.CreatedAt, c.CreatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("insert conversation failed", zap.Error(err))
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// GetConversationByPair reads the conversation for a canonical pair, within a tx.
func (repository *ChatRepository) GetConversationByPair(ctx context.Context, tx pgx.Tx, serverId, low, high string) (model.DmConversation, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetConversationByPair")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(attribute.String("db.system", "postgres"), attribute.String("db.operation", "SELECT"))

	var conversation model.DmConversation
	query := `SELECT id, server_id, user_low, user_high, last_message_at, created_at, updated_at, created_by, updated_by
		FROM dm_conversations WHERE server_id=$1 AND user_low=$2 AND user_high=$3`
	err = tx.QueryRow(ctx, query, serverId, low, high).Scan(
		&conversation.Id, &conversation.ServerId, &conversation.UserLow, &conversation.UserHigh, &conversation.LastMessageAt,
		&conversation.CreatedAt, &conversation.UpdatedAt, &conversation.CreatedBy, &conversation.UpdatedBy)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("get conversation by pair failed", zap.Error(err))
		return model.DmConversation{}, err
	}
	return conversation, nil
}

// InsertConversationStateIdempotent ensures ONE participant's state row exists.
// Usecase calls it once per participant. SINGLE statement.
func (repository *ChatRepository) InsertConversationStateIdempotent(ctx context.Context, tx pgx.Tx, conversationId, userId, serverId, peerUserId string, now time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.InsertConversationStateIdempotent")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(attribute.String("db.system", "postgres"), attribute.String("db.operation", "INSERT"))

	query := `INSERT INTO dm_conversation_states (conversation_id, user_id, server_id, peer_user_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$5)
		ON CONFLICT (conversation_id, user_id) DO NOTHING`
	_, err = tx.Exec(ctx, query, conversationId, userId, serverId, peerUserId, now)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("insert conversation state failed", zap.Error(err))
		return err
	}
	return nil
}

// GetConversationById returns conversation core fields (for authz + serverId).
func (repository *ChatRepository) GetConversationById(ctx context.Context, conversationId string) (model.DmConversation, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetConversationById")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(attribute.String("db.system", "postgres"), attribute.String("db.operation", "SELECT"))

	var conversation model.DmConversation
	query := `SELECT id, server_id, user_low, user_high, last_message_at, created_at, updated_at, created_by, updated_by
		FROM dm_conversations WHERE id=$1`
	err = repository.DB.QueryRow(ctx, query, conversationId).Scan(
		&conversation.Id, &conversation.ServerId, &conversation.UserLow, &conversation.UserHigh, &conversation.LastMessageAt,
		&conversation.CreatedAt, &conversation.UpdatedAt, &conversation.CreatedBy, &conversation.UpdatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.DmConversation{}, pgx.ErrNoRows
	}
	if err != nil {
		return model.DmConversation{}, err
	}
	return conversation, nil
}

// InsertMessageIdempotent inserts a message; ON CONFLICT on (conversation, sender,
// client_message_id) does nothing. Returns true if actually inserted. SINGLE statement;
// the usecase owns the tx and side-effect updates.
func (repository *ChatRepository) InsertMessageIdempotent(ctx context.Context, tx pgx.Tx, m model.DmMessage) (bool, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.InsertMessageIdempotent")
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
		attribute.String("conversation.id", m.ConversationId),
	)

	query := `INSERT INTO dm_messages (id, conversation_id, sender_id, type, content, client_message_id, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (conversation_id, sender_id, client_message_id) DO NOTHING`
	var tag pgconn.CommandTag
	tag, err = tx.Exec(ctx, query, m.Id, m.ConversationId, m.SenderId, m.Type, m.Content, m.ClientMessageId, m.CreatedAt)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("insert message failed", zap.Error(err))
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// GetMessageByClientId reads an existing message by its idempotency key (within tx).
// Called by the usecase only when InsertMessageIdempotent reported a conflict.
func (repository *ChatRepository) GetMessageByClientId(ctx context.Context, tx pgx.Tx, conversationId, senderId, clientMessageId string) (model.DmMessage, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetMessageByClientId")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(attribute.String("db.system", "postgres"), attribute.String("db.operation", "SELECT"))

	var message model.DmMessage
	query := `SELECT id, conversation_id, sender_id, type, content, client_message_id, created_at
		FROM dm_messages WHERE conversation_id=$1 AND sender_id=$2 AND client_message_id=$3`
	err = tx.QueryRow(ctx, query, conversationId, senderId, clientMessageId).Scan(
		&message.Id, &message.ConversationId, &message.SenderId, &message.Type, &message.Content, &message.ClientMessageId, &message.CreatedAt)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("get message by client id failed", zap.Error(err))
		return model.DmMessage{}, err
	}
	return message, nil
}

// UpdateConversationLastMessage bumps the conversation activity timestamp (within tx).
func (repository *ChatRepository) UpdateConversationLastMessage(ctx context.Context, tx pgx.Tx, conversationId, updatedBy string, at time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdateConversationLastMessage")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(attribute.String("db.system", "postgres"), attribute.String("db.operation", "UPDATE"))

	query := `UPDATE dm_conversations SET last_message_at=$1, updated_at=$1, updated_by=$2 WHERE id=$3`
	_, err = tx.Exec(ctx, query, at, updatedBy, conversationId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("update conversation last message failed", zap.Error(err))
	}
	return err
}

// BumpConversationStates updates both participant state rows in one statement:
// sender row gets last_message_at + preview (unread unchanged),
// recipient row gets the same plus unread_count+1.
func (repository *ChatRepository) BumpConversationStates(ctx context.Context, tx pgx.Tx, conversationId, senderId, preview string, at time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.BumpConversationStates")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(attribute.String("db.system", "postgres"), attribute.String("db.operation", "UPDATE"))

	query := `UPDATE dm_conversation_states
		SET last_message_at=$1, last_message_preview=$2,
		    unread_count = CASE WHEN user_id=$4 THEN unread_count ELSE unread_count + 1 END,
		    updated_at=$1
		WHERE conversation_id=$3`
	_, err = tx.Exec(ctx, query, at, preview, conversationId, senderId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("bump conversation states failed", zap.Error(err))
	}
	return err
}

// ListMessages returns a page of messages newest-first with sender identity
// resolved from server_member_profiles (LEFT JOIN + fallback). limit+1 pattern.
func (repository *ChatRepository) ListMessages(ctx context.Context, conversationId, serverId string, limit int, cursor *model.DmMessageCursor, minioFullUrl string) ([]model.DmMessageResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.ListMessages")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(attribute.String("db.system", "postgres"), attribute.String("conversation.id", conversationId))

	base := `SELECT m.id, m.conversation_id, m.sender_id, m.type, m.content, m.client_message_id, m.created_at,
			COALESCE(smp.nickname, 'Pengguna'), COALESCE(smp.username, ''), pai.object_key
		FROM dm_messages m
		LEFT JOIN server_member_profiles smp ON smp.server_id=$1 AND smp.user_id=m.sender_id
		LEFT JOIN profile_avatar_images pai ON pai.id=smp.avatar_image_id
		WHERE m.conversation_id=$2`
	args := []any{serverId, conversationId}
	if cursor != nil {
		base += ` AND (m.created_at < $3 OR (m.created_at = $3 AND m.id < $4))`
		args = append(args, cursor.CreatedAt, cursor.Id)
	}
	base += fmt.Sprintf(` ORDER BY m.created_at DESC, m.id DESC LIMIT $%d`, len(args)+1)
	args = append(args, limit)

	rows, err := repository.DB.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.DmMessageResponse
	for rows.Next() {
		var messageRow model.DmMessageResponse
		var objectKey *string
		if err = rows.Scan(
			&messageRow.Id, &messageRow.ConversationId, &messageRow.SenderId,
			&messageRow.Type, &messageRow.Content, &messageRow.ClientMessageId, &messageRow.CreatedAt,
			&messageRow.Sender.Nickname, &messageRow.Sender.Username, &objectKey,
		); err != nil {
			return nil, err
		}
		messageRow.Sender.AvatarUrl = chatAvatarUrl(minioFullUrl, objectKey)
		out = append(out, messageRow)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ListConversations returns the caller's inbox (existing conversations) in a
// server, newest-first, with peer identity. limit+1 pattern.
func (repository *ChatRepository) ListConversations(ctx context.Context, userId, serverId string, limit int, cursor *model.DmConversationCursor, minioFullUrl string) ([]model.DmConversationResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.ListConversations")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(attribute.String("db.system", "postgres"))

	base := `SELECT s.conversation_id, s.server_id, s.peer_user_id, s.unread_count, s.last_message_preview, s.last_message_at,
			COALESCE(smp.nickname, 'Pengguna'), COALESCE(smp.username, ''), pai.object_key
		FROM dm_conversation_states s
		LEFT JOIN server_member_profiles smp ON smp.server_id=s.server_id AND smp.user_id=s.peer_user_id
		LEFT JOIN profile_avatar_images pai ON pai.id=smp.avatar_image_id
		WHERE s.user_id=$1 AND s.server_id=$2 AND s.last_message_at IS NOT NULL`
	args := []any{userId, serverId}
	if cursor != nil {
		base += ` AND (s.last_message_at < $3 OR (s.last_message_at = $3 AND s.conversation_id < $4))`
		args = append(args, cursor.LastMessageAt, cursor.ConversationId)
	}
	base += fmt.Sprintf(` ORDER BY s.last_message_at DESC, s.conversation_id DESC LIMIT $%d`, len(args)+1)
	args = append(args, limit)

	rows, err := repository.DB.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.DmConversationResponse
	for rows.Next() {
		var conv model.DmConversationResponse
		var objectKey *string
		if err = rows.Scan(
			&conv.Id, &conv.ServerId, &conv.PeerUserId, &conv.UnreadCount,
			&conv.LastMessagePreview, &conv.LastMessageAt,
			&conv.Peer.Nickname, &conv.Peer.Username, &objectKey,
		); err != nil {
			return nil, err
		}
		conv.Peer.AvatarUrl = chatAvatarUrl(minioFullUrl, objectKey)
		out = append(out, conv)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ListMembers returns ALL members of a server (except caller) with per-server
// identity, enriched with the caller's DM state per member if it exists.
// Ordered by nickname ASC (tiebreak user_id). Optional ILIKE search. limit+1.
func (repository *ChatRepository) ListMembers(ctx context.Context, callerId, serverId, q string, limit int, cursor *model.DmMemberCursor, minioFullUrl string) ([]model.DmMemberResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.ListMembers")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(attribute.String("db.system", "postgres"), attribute.String("server.id", serverId))

	base := `SELECT sm.user_id, smp.nickname, smp.username, pai.object_key,
			st.conversation_id, COALESCE(st.unread_count,0), st.last_message_preview, st.last_message_at
		FROM server_members sm
		JOIN server_member_profiles smp ON smp.server_id=sm.server_id AND smp.user_id=sm.user_id
		LEFT JOIN profile_avatar_images pai ON pai.id=smp.avatar_image_id
		LEFT JOIN dm_conversation_states st ON st.server_id=sm.server_id AND st.user_id=$1 AND st.peer_user_id=sm.user_id
		WHERE sm.server_id=$2 AND sm.user_id<>$1`
	args := []any{callerId, serverId}
	n := 3
	if q != "" {
		base += fmt.Sprintf(` AND (smp.nickname ILIKE $%d OR smp.username ILIKE $%d)`, n, n)
		args = append(args, q+"%")
		n++
	}
	if cursor != nil {
		base += fmt.Sprintf(` AND (smp.nickname > $%d OR (smp.nickname = $%d AND sm.user_id > $%d))`, n, n, n+1)
		args = append(args, cursor.Nickname, cursor.UserId)
		n += 2
	}
	base += fmt.Sprintf(` ORDER BY smp.nickname ASC, sm.user_id ASC LIMIT $%d`, n)
	args = append(args, limit)

	rows, err := repository.DB.Query(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.DmMemberResponse
	for rows.Next() {
		var member model.DmMemberResponse
		var objectKey *string
		if err = rows.Scan(
			&member.UserId, &member.Identity.Nickname, &member.Identity.Username, &objectKey,
			&member.ConversationId, &member.UnreadCount, &member.LastMessagePreview, &member.LastMessageAt,
		); err != nil {
			return nil, err
		}
		member.Identity.AvatarUrl = chatAvatarUrl(minioFullUrl, objectKey)
		out = append(out, member)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// GetMessageCreatedAt returns a message's created_at (pool; single SELECT). Used by
// the usecase to derive the read-receipt timestamp from a lastReadMessageId.
func (repository *ChatRepository) GetMessageCreatedAt(ctx context.Context, conversationId, messageId string) (time.Time, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetMessageCreatedAt")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(attribute.String("db.system", "postgres"), attribute.String("db.operation", "SELECT"))

	var createdAt time.Time
	query := `SELECT created_at FROM dm_messages WHERE id=$1 AND conversation_id=$2`
	err = repository.DB.QueryRow(ctx, query, messageId, conversationId).Scan(&createdAt)
	if err != nil {
		return time.Time{}, err
	}
	return createdAt, nil
}

// MarkRead sets the caller's read pointer + resets unread (pool; single UPDATE).
// readAt is computed by the usecase (msg.created_at or now).
func (repository *ChatRepository) MarkRead(ctx context.Context, conversationId, userId string, lastReadMessageId *string, readAt, now time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.MarkRead")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(attribute.String("db.system", "postgres"), attribute.String("db.operation", "UPDATE"))

	query := `UPDATE dm_conversation_states SET last_read_message_id=$1, last_read_at=$2, unread_count=0, updated_at=$3 WHERE conversation_id=$4 AND user_id=$5`
	_, err = repository.DB.Exec(ctx, query, lastReadMessageId, readAt, now, conversationId, userId)
	if err != nil {
		util.GetLoggerWithTraceContext(ctx, repository.Log).Error("mark read failed", zap.Error(err))
	}
	return err
}

// GetMemberIdentity resolves per-server identity for a user (pool; single SELECT).
func (repository *ChatRepository) GetMemberIdentity(ctx context.Context, serverId, userId, minioFullUrl string) (model.DmIdentity, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetMemberIdentity")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()
	span.SetAttributes(attribute.String("db.system", "postgres"), attribute.String("db.operation", "SELECT"))

	var identity model.DmIdentity
	var objectKey *string
	query := `SELECT smp.nickname, smp.username, pai.object_key
		FROM server_member_profiles smp
		LEFT JOIN profile_avatar_images pai ON pai.id=smp.avatar_image_id
		WHERE smp.server_id=$1 AND smp.user_id=$2`
	err = repository.DB.QueryRow(ctx, query, serverId, userId).Scan(&identity.Nickname, &identity.Username, &objectKey)
	if err != nil {
		return model.DmIdentity{}, err
	}
	identity.AvatarUrl = chatAvatarUrl(minioFullUrl, objectKey)
	return identity, nil
}

// ── file-local helper ──

func chatAvatarUrl(minioFullUrl string, objectKey *string) *string {
	if objectKey == nil {
		return nil
	}
	u := minioFullUrl + "/" + *objectKey
	return &u
}

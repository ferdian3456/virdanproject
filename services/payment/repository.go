package payment

import (
	"context"
	"errors"
	"time"

	"github.com/ferdian3456/virdanproject/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

type Repository struct {
	Log    *zap.Logger
	Config *koanf.Koanf
	DB     *pgxpool.Pool
}

func NewRepository(log *zap.Logger, config *koanf.Koanf, db *pgxpool.Pool) *Repository {
	return &Repository{Log: log, Config: config, DB: db}
}

func (repository *Repository) GetActivePlus(ctx context.Context, serverId string) (bool, *time.Time, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetActivePlus")
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

	query := `
		SELECT plus_expires_at
		FROM server_plus_orders
		WHERE server_id = $1 AND status = 'PAID' AND plus_expires_at > $2
		ORDER BY plus_expires_at DESC
		LIMIT 1`

	now := time.Now().UTC()
	var expiresAt time.Time
	err = repository.DB.QueryRow(ctx, query, serverId, now).Scan(&expiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
			return false, nil, nil
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("GetActivePlus failed", zap.Error(err))
		return false, nil, err
	}
	return true, &expiresAt, nil
}

func (repository *Repository) InsertOrder(ctx context.Context, order ServerPlusOrder) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.InsertOrder")
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
		attribute.String("server.id", order.ServerId),
	)

	query := `INSERT INTO server_plus_orders
		(id, server_id, user_id, reference_id, xendit_session_id, base_idr, tax_idr, total_idr, status, created_at, updated_at, created_by, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`

	var sessionId any
	if order.XenditSessionId != "" {
		sessionId = order.XenditSessionId
	}
	_, err = repository.DB.Exec(ctx, query,
		order.Id, order.ServerId, order.UserId, order.ReferenceId, sessionId,
		order.BaseIdr, order.TaxIdr, order.TotalIdr, order.Status,
		order.CreatedAt, order.UpdatedAt, order.CreatedBy, order.UpdatedBy)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("InsertOrder failed", zap.Error(err))
		return err
	}
	return nil
}

func (repository *Repository) UpdateOrderSession(ctx context.Context, orderId, sessionId string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.UpdateOrderSession")
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
	)

	query := `UPDATE server_plus_orders SET xendit_session_id = $1, updated_at = $2 WHERE id = $3`
	_, err = repository.DB.Exec(ctx, query, sessionId, updatedAt, orderId)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("UpdateOrderSession failed", zap.Error(err))
	}
	return err
}

func (repository *Repository) GetOrderByReferenceId(ctx context.Context, referenceId string) (ServerPlusOrder, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetOrderByReferenceId")
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

	query := `SELECT id, server_id, user_id, status FROM server_plus_orders WHERE reference_id = $1 LIMIT 1`

	var order ServerPlusOrder
	err = repository.DB.QueryRow(ctx, query, referenceId).Scan(&order.Id, &order.ServerId, &order.UserId, &order.Status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &shared.NotFoundError{Code: shared.ERR_NOT_FOUND_CODE, Message: "Order not found", Param: "referenceId"}
			return order, err
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("GetOrderByReferenceId failed", zap.Error(err))
		return order, err
	}
	order.ReferenceId = referenceId
	return order, nil
}

func (repository *Repository) MarkOrderPaid(ctx context.Context, orderId, xenditPaymentId string, paidAt, expiresAt, updatedAt time.Time) (int64, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.MarkOrderPaid")
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
	)

	query := `UPDATE server_plus_orders
		SET status = 'PAID', xendit_payment_id = $1, paid_at = $2, plus_expires_at = $3, updated_at = $4
		WHERE id = $5 AND status = 'PENDING'`
	tag, execErr := repository.DB.Exec(ctx, query, xenditPaymentId, paidAt, expiresAt, updatedAt, orderId)
	if execErr != nil {
		err = execErr
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("MarkOrderPaid failed", zap.Error(err))
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (repository *Repository) MarkOrderFailed(ctx context.Context, orderId string, updatedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.MarkOrderFailed")
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
	)

	query := `UPDATE server_plus_orders SET status = 'FAILED', updated_at = $1 WHERE id = $2 AND status = 'PENDING'`
	_, err = repository.DB.Exec(ctx, query, updatedAt, orderId)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("MarkOrderFailed failed", zap.Error(err))
	}
	return err
}

func (repository *Repository) ListOrdersByUser(ctx context.Context, userId string, cursor *PlusOrderCursor, limit int) ([]PlusOrderHistoryItem, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.ListOrdersByUser")
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
	if cursor != nil && cursor.Id != "" && !cursor.CreatedAt.IsZero() {
		query := `
			SELECT o.id, o.server_id, s.name, o.total_idr, o.status, o.paid_at, o.plus_expires_at, o.created_at
			FROM server_plus_orders o
			INNER JOIN servers s ON s.id = o.server_id
			WHERE o.user_id = $1 AND (o.created_at < $2 OR (o.created_at = $2 AND o.id < $3))
			ORDER BY o.created_at DESC, o.id DESC
			LIMIT $4`
		rows, err = repository.DB.Query(ctx, query, userId, cursor.CreatedAt, cursor.Id, limit)
	} else {
		query := `
			SELECT o.id, o.server_id, s.name, o.total_idr, o.status, o.paid_at, o.plus_expires_at, o.created_at
			FROM server_plus_orders o
			INNER JOIN servers s ON s.id = o.server_id
			WHERE o.user_id = $1
			ORDER BY o.created_at DESC, o.id DESC
			LIMIT $2`
		rows, err = repository.DB.Query(ctx, query, userId, limit)
	}
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("ListOrdersByUser failed", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	items := make([]PlusOrderHistoryItem, 0, limit)
	for rows.Next() {
		var it PlusOrderHistoryItem
		if err = rows.Scan(&it.Id, &it.ServerId, &it.ServerName, &it.TotalIdr, &it.Status, &it.PaidAt, &it.PlusExpiresAt, &it.CreatedAt); err != nil {
			shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("ListOrdersByUser scan failed", zap.Error(err))
			return nil, err
		}
		items = append(items, it)
	}
	if err = rows.Err(); err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("ListOrdersByUser rows error", zap.Error(err))
		return nil, err
	}

	return items, nil
}

func (repository *Repository) GetOrderDetailByIdForUser(ctx context.Context, orderId, userId string) (PlusOrderDetailResponse, error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.GetOrderDetailByIdForUser")
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
		attribute.String("order.id", orderId),
	)

	query := `
		SELECT o.id, o.server_id, s.name, o.reference_id, o.base_idr, o.tax_idr, o.total_idr, o.status, o.paid_at, o.plus_expires_at, o.created_at
		FROM server_plus_orders o
		INNER JOIN servers s ON s.id = o.server_id
		WHERE o.id = $1 AND o.user_id = $2`

	var detail PlusOrderDetailResponse
	err = repository.DB.QueryRow(ctx, query, orderId, userId).Scan(
		&detail.Id, &detail.ServerId, &detail.ServerName, &detail.ReferenceId,
		&detail.BaseIdr, &detail.TaxIdr, &detail.TotalIdr, &detail.Status,
		&detail.PaidAt, &detail.PlusExpiresAt, &detail.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = &shared.NotFoundError{Code: shared.ERR_NOT_FOUND_CODE, Message: "Order not found", Param: "orderId"}
			return detail, err
		}
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("GetOrderDetailByIdForUser failed", zap.Error(err))
		return detail, err
	}
	return detail, nil
}

func (repository *Repository) InsertWebhookEventIdempotent(ctx context.Context, event XenditWebhookEvent) (inserted bool, err error) {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.InsertWebhookEventIdempotent")
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	span.SetAttributes(
		attribute.String("db.system", "postgres"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("xendit.event_id", event.EventId),
	)

	query := `INSERT INTO xendit_webhook_events
		(id, event_id, event_type, reference_id, payload, status, received_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (event_id) DO NOTHING`

	var refId any
	if event.ReferenceId != "" {
		refId = event.ReferenceId
	}
	tag, execErr := repository.DB.Exec(ctx, query,
		event.Id, event.EventId, event.EventType, refId, event.Payload, event.Status, event.ReceivedAt)
	if execErr != nil {
		err = execErr
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("InsertWebhookEventIdempotent failed", zap.Error(err))
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (repository *Repository) MarkWebhookProcessed(ctx context.Context, eventId, status string, processedAt time.Time) error {
	serviceName := repository.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-repository").Start(ctx, "repository.MarkWebhookProcessed")
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
	)

	query := `UPDATE xendit_webhook_events SET status = $1, processed_at = $2 WHERE event_id = $3`
	_, err = repository.DB.Exec(ctx, query, status, processedAt, eventId)
	if err != nil {
		shared.GetLoggerWithTraceContext(ctx, repository.Log).Error("MarkWebhookProcessed failed", zap.Error(err))
	}
	return err
}

package usecase

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"time"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/repository"
	"github.com/ferdian3456/virdanproject/internal/util"
	xenditpkg "github.com/ferdian3456/virdanproject/internal/xendit"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type ServerPlusUsecase struct {
	ServerPlusRepository *repository.ServerPlusRepository
	ServerRepository     *repository.ServerRepository
	XenditClient         *xenditpkg.Client
	DB                   *pgxpool.Pool
	Log                  *zap.Logger
	Config               *koanf.Koanf
}

func NewServerPlusUsecase(
	serverPlusRepository *repository.ServerPlusRepository,
	serverRepository *repository.ServerRepository,
	xenditClient *xenditpkg.Client,
	db *pgxpool.Pool,
	log *zap.Logger,
	config *koanf.Koanf,
) *ServerPlusUsecase {
	return &ServerPlusUsecase{
		ServerPlusRepository: serverPlusRepository,
		ServerRepository:     serverRepository,
		XenditClient:         xenditClient,
		DB:                   db,
		Log:                  log,
		Config:               config,
	}
}

// priceBreakdown computes base, tax, and total from constants.
func priceBreakdown() (base, tax, total int64) {
	base = constant.PLUS_PRICE_IDR
	tax = base * constant.PLUS_TAX_PERCENT / 100
	total = base + tax
	return
}

// GetPlusStatus returns the server's plus status plus the price breakdown.
// Guard: requester must be a member of the server.
func (usecase *ServerPlusUsecase) GetPlusStatus(ctx fiber.Ctx, userId, serverId string) (model.PlusStatusResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.GetPlusStatus")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.PlusStatusResponse

	v := util.NewValidator()
	v.UUID("serverId", serverId).Required()
	if err = v.Validate(); err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	memberCount, mErr := usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if mErr != nil {
		err = mErr
		return response, err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return response, err
	}

	active, expiresAt, aErr := usecase.ServerPlusRepository.GetActivePlus(ctxContext, serverId)
	if aErr != nil {
		err = aErr
		return response, err
	}

	base, tax, total := priceBreakdown()
	response = model.PlusStatusResponse{
		Active:       active,
		ExpiresAt:    expiresAt,
		DurationDays: constant.PLUS_DURATION_DAYS,
		Price:        model.PlusPriceResponse{BaseIdr: base, TaxIdr: tax, TotalIdr: total},
	}
	return response, nil
}

// Checkout creates a PENDING order and a Xendit PAY session. Rejected (409) when
// the server already has an active plus.
func (usecase *ServerPlusUsecase) Checkout(ctx fiber.Ctx, userId, serverId string) (model.PlusCheckoutResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.PlusCheckout")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response model.PlusCheckoutResponse

	v := util.NewValidator()
	v.UUID("serverId", serverId).Required()
	if err = v.Validate(); err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	memberCount, mErr := usecase.ServerRepository.CheckServerMember(ctxContext, serverId, userId)
	if mErr != nil {
		err = mErr
		return response, err
	}
	if memberCount == 0 {
		err = &model.ForbiddenError{Code: constant.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return response, err
	}

	active, _, aErr := usecase.ServerPlusRepository.GetActivePlus(ctxContext, serverId)
	if aErr != nil {
		err = aErr
		return response, err
	}
	if active {
		err = &model.ConflictError{Code: constant.ERR_CONFLICT_CODE, Message: "Server already has an active Virdan Plus", Param: "serverId"}
		return response, err
	}

	now := time.Now().UTC()
	orderId := uuid.New().String()
	referenceId := "virdan-plus-" + orderId
	base, tax, total := priceBreakdown()

	order := model.ServerPlusOrder{
		Id:          orderId,
		ServerId:    serverId,
		UserId:      userId,
		ReferenceId: referenceId,
		BaseIdr:     base,
		TaxIdr:      tax,
		TotalIdr:    total,
		Status:      "PENDING",
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   userId,
		UpdatedBy:   userId,
	}
	if err = usecase.ServerPlusRepository.InsertOrder(ctxContext, order); err != nil {
		return response, err
	}

	sessionId, paymentUrl, sErr := usecase.XenditClient.CreatePaymentSession(ctxContext, referenceId, "Virdan Plus (30 days)", total)
	if sErr != nil {
		// Order stays PENDING; the user can retry (new order). Log the Xendit error.
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("CreatePaymentSession failed", zap.Error(sErr))
		err = sErr
		return response, err
	}

	if uErr := usecase.ServerPlusRepository.UpdateOrderSession(ctxContext, orderId, sessionId, time.Now().UTC()); uErr != nil {
		// Non-fatal: the session already exists, continue. Log only.
		util.GetLoggerWithTraceContext(ctxContext, usecase.Log).Error("UpdateOrderSession failed (non-fatal)", zap.Error(uErr))
	}

	response = model.PlusCheckoutResponse{OrderId: orderId, PaymentUrl: paymentUrl}
	return response, nil
}

// ListMyOrders returns the user's global payment history (keyset paginated).
func (usecase *ServerPlusUsecase) ListMyOrders(ctx fiber.Ctx, userId, cursorStr string, limit int) (model.PlusOrderHistoryResponse, error) {
	ctxContext := ctx.Context()
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-usecase").Start(ctxContext, "usecase.ListMyPlusOrders")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	if limit <= 0 || limit > constant.MAX_LIMIT {
		limit = constant.DEFAULT_LIMIT
	}

	var cursor model.PlusOrderCursor
	if cursorStr != "" {
		decoded, decErr := util.DecodeCursor[model.PlusOrderCursor](cursorStr)
		if decErr != nil {
			err = &model.BadRequestError{Code: constant.ERR_BAD_REQUEST_CODE, Message: "Invalid cursor", Param: "cursor"}
			return model.PlusOrderHistoryResponse{}, err
		}
		cursor = *decoded
	}

	items, lErr := usecase.ServerPlusRepository.ListOrdersByUser(ctxContext, userId, &cursor, limit+1)
	if lErr != nil {
		err = lErr
		return model.PlusOrderHistoryResponse{}, err
	}

	response := model.PlusOrderHistoryResponse{Data: []model.PlusOrderHistoryItem{}}
	if len(items) > limit {
		response.Data = items[:limit]
		last := items[limit-1]
		response.Page.NextCursor = util.EncodeCursor(model.PlusOrderCursor{CreatedAt: last.CreatedAt, Id: last.Id})
	} else {
		response.Data = items
	}

	return response, nil
}

// HandleWebhook is called by the controller with the raw body + x-callback-token.
// It verifies the token, dedupes by event_id, then grants asynchronously in a
// goroutine. It returns an error only for an invalid token, an invalid payload, or
// a DB error inserting the event.
func (usecase *ServerPlusUsecase) HandleWebhook(ctx context.Context, callbackToken string, rawPayload []byte) error {
	serviceName := usecase.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-usecase").Start(ctx, "usecase.HandlePlusWebhook")
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	expectedToken := usecase.Config.String("XENDIT_WEBHOOK_TOKEN")
	if subtle.ConstantTimeCompare([]byte(callbackToken), []byte(expectedToken)) != 1 {
		err = &model.UnauthorizedError{Code: constant.ERR_UNAUTHORIZED_CODE, Message: "Invalid webhook token"}
		return err
	}

	var envelope struct {
		Event string `json:"event"`
		Data  struct {
			PaymentId   string `json:"payment_id"`
			ReferenceId string `json:"reference_id"`
			Status      string `json:"status"`
		} `json:"data"`
	}
	if jsonErr := json.Unmarshal(rawPayload, &envelope); jsonErr != nil {
		err = &model.BadRequestError{Code: constant.ERR_BAD_REQUEST_CODE, Message: "Invalid webhook payload"}
		return err
	}

	// event_id for idempotency: payment_id (unique per payment). Fallback to event+ref.
	eventId := envelope.Data.PaymentId
	if eventId == "" {
		eventId = envelope.Event + "-" + envelope.Data.ReferenceId
	}

	span.SetAttributes(
		attribute.String("xendit.event_type", envelope.Event),
		attribute.String("xendit.event_id", eventId),
	)

	now := time.Now().UTC()
	event := model.XenditWebhookEvent{
		Id:          uuid.New().String(),
		EventId:     eventId,
		EventType:   envelope.Event,
		ReferenceId: envelope.Data.ReferenceId,
		Payload:     rawPayload,
		Status:      "PENDING",
		ReceivedAt:  now,
	}
	inserted, insErr := usecase.ServerPlusRepository.InsertWebhookEventIdempotent(ctx, event)
	if insErr != nil {
		err = insErr
		return err
	}
	if !inserted {
		// Duplicate delivery — respond 200 without reprocessing.
		util.GetLoggerWithTraceContext(ctx, usecase.Log).Info("Duplicate xendit webhook skipped", zap.String("event_id", eventId))
		return nil
	}

	// Dispatch async (recover + carry parent span context).
	parentSpanCtx := trace.SpanContextFromContext(ctx)
	eventType := envelope.Event
	referenceId := envelope.Data.ReferenceId
	paymentId := envelope.Data.PaymentId
	status := envelope.Data.Status
	go func() {
		bg := trace.ContextWithSpanContext(context.Background(), parentSpanCtx)
		bg, cancel := context.WithTimeout(bg, 30*time.Second)
		defer cancel()
		bg, gspan := otel.Tracer(usecase.Config.String("OTEL_SERVICE_NAME")+"-usecase").Start(bg, "usecase.dispatchPlusWebhook")
		defer gspan.End()
		logger := util.GetLoggerWithTraceContext(bg, usecase.Log)
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic in plus webhook goroutine", zap.Any("recover", rec))
			}
		}()

		usecase.dispatchWebhook(bg, eventId, eventType, referenceId, paymentId, status, logger)
	}()

	return nil
}

// dispatchWebhook processes the event (runs in a goroutine). It grants plus for a
// payment.capture SUCCEEDED event.
func (usecase *ServerPlusUsecase) dispatchWebhook(ctx context.Context, eventId, eventType, referenceId, paymentId, status string, logger *zap.Logger) {
	processedAt := time.Now().UTC()

	switch eventType {
	case "payment.capture":
		if status != "SUCCEEDED" {
			logger.Info("payment.capture non-succeeded ignored", zap.String("status", status))
			_ = usecase.ServerPlusRepository.MarkWebhookProcessed(ctx, eventId, "PROCESSED", processedAt)
			return
		}
		order, oErr := usecase.ServerPlusRepository.GetOrderByReferenceId(ctx, referenceId)
		if oErr != nil {
			logger.Error("GetOrderByReferenceId failed", zap.Error(oErr))
			_ = usecase.ServerPlusRepository.MarkWebhookProcessed(ctx, eventId, "FAILED", processedAt)
			return
		}
		if order.Status == "PAID" {
			logger.Info("order already paid, skip", zap.String("order_id", order.Id))
			_ = usecase.ServerPlusRepository.MarkWebhookProcessed(ctx, eventId, "PROCESSED", processedAt)
			return
		}
		expiresAt := processedAt.AddDate(0, 0, constant.PLUS_DURATION_DAYS)
		affected, mErr := usecase.ServerPlusRepository.MarkOrderPaid(ctx, order.Id, paymentId, processedAt, expiresAt, processedAt)
		if mErr != nil {
			logger.Error("MarkOrderPaid failed", zap.Error(mErr))
			_ = usecase.ServerPlusRepository.MarkWebhookProcessed(ctx, eventId, "FAILED", processedAt)
			return
		}
		logger.Info("Virdan Plus granted", zap.String("order_id", order.Id), zap.Int64("rows", affected))
		_ = usecase.ServerPlusRepository.MarkWebhookProcessed(ctx, eventId, "PROCESSED", processedAt)

	case "payment.failure":
		order, oErr := usecase.ServerPlusRepository.GetOrderByReferenceId(ctx, referenceId)
		if oErr != nil {
			logger.Error("GetOrderByReferenceId failed", zap.Error(oErr))
			_ = usecase.ServerPlusRepository.MarkWebhookProcessed(ctx, eventId, "FAILED", processedAt)
			return
		}
		if fErr := usecase.ServerPlusRepository.MarkOrderFailed(ctx, order.Id, processedAt); fErr != nil {
			logger.Error("MarkOrderFailed failed", zap.Error(fErr))
			_ = usecase.ServerPlusRepository.MarkWebhookProcessed(ctx, eventId, "FAILED", processedAt)
			return
		}
		_ = usecase.ServerPlusRepository.MarkWebhookProcessed(ctx, eventId, "PROCESSED", processedAt)

	default:
		logger.Info("Unhandled xendit event type", zap.String("event_type", eventType))
		_ = usecase.ServerPlusRepository.MarkWebhookProcessed(ctx, eventId, "PROCESSED", processedAt)
	}
}

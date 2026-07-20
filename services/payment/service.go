package payment

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"time"

	"github.com/ferdian3456/virdanproject/services/server"
	"github.com/ferdian3456/virdanproject/shared"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Service struct {
	Repo         *Repository
	ServerRepo   *server.Repository
	XenditClient *XenditClient
	DB           *pgxpool.Pool
	Log          *zap.Logger
	Config       *koanf.Koanf
}

func NewService(repo *Repository, serverRepo *server.Repository, xenditClient *XenditClient, db *pgxpool.Pool, log *zap.Logger, config *koanf.Koanf) *Service {
	return &Service{
		Repo:         repo,
		ServerRepo:   serverRepo,
		XenditClient: xenditClient,
		DB:           db,
		Log:          log,
		Config:       config,
	}
}

func priceBreakdown() (base, tax, total int64) {
	base = shared.PLUS_PRICE_IDR
	tax = base * shared.PLUS_TAX_PERCENT / 100
	total = base + tax
	return
}

func (service *Service) GetPlusStatus(ctx fiber.Ctx, userId, serverId string) (PlusStatusResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetPlusStatus")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response PlusStatusResponse

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	if err = v.Validate(); err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	memberCount, mErr := service.ServerRepo.CheckServerMember(ctxContext, serverId, userId)
	if mErr != nil {
		err = mErr
		return response, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return response, err
	}

	active, expiresAt, aErr := service.Repo.GetActivePlus(ctxContext, serverId)
	if aErr != nil {
		err = aErr
		return response, err
	}

	base, tax, total := priceBreakdown()
	response = PlusStatusResponse{
		Active:       active,
		ExpiresAt:    expiresAt,
		DurationDays: shared.PLUS_DURATION_DAYS,
		Price:        PlusPriceResponse{BaseIdr: base, TaxIdr: tax, TotalIdr: total},
	}
	return response, nil
}

func (service *Service) Checkout(ctx fiber.Ctx, userId, serverId string) (PlusCheckoutResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.PlusCheckout")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response PlusCheckoutResponse

	v := shared.NewValidator()
	v.UUID("serverId", serverId).Required()
	if err = v.Validate(); err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("server.id", serverId))

	memberCount, mErr := service.ServerRepo.CheckServerMember(ctxContext, serverId, userId)
	if mErr != nil {
		err = mErr
		return response, err
	}
	if memberCount == 0 {
		err = &shared.ForbiddenError{Code: shared.ERR_FORBIDDEN_CODE, Message: "You are not a member of this server", Param: "serverId"}
		return response, err
	}

	active, _, aErr := service.Repo.GetActivePlus(ctxContext, serverId)
	if aErr != nil {
		err = aErr
		return response, err
	}
	if active {
		err = &shared.ConflictError{Code: shared.ERR_CONFLICT_CODE, Message: "Server already has an active Virdan Plus", Param: "serverId"}
		return response, err
	}

	now := time.Now().UTC()
	orderId := uuid.New().String()
	referenceId := "virdan-plus-" + orderId
	base, tax, total := priceBreakdown()

	order := ServerPlusOrder{
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
	if err = service.Repo.InsertOrder(ctxContext, order); err != nil {
		return response, err
	}

	sessionId, paymentUrl, sErr := service.XenditClient.CreatePaymentSession(ctxContext, referenceId, "Virdan Plus (30 days)", total)
	if sErr != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("CreatePaymentSession failed", zap.Error(sErr))
		err = sErr
		return response, err
	}

	if uErr := service.Repo.UpdateOrderSession(ctxContext, orderId, sessionId, time.Now().UTC()); uErr != nil {
		shared.GetLoggerWithTraceContext(ctxContext, service.Log).Error("UpdateOrderSession failed (non-fatal)", zap.Error(uErr))
	}

	response = PlusCheckoutResponse{OrderId: orderId, PaymentUrl: paymentUrl}
	return response, nil
}

func (service *Service) ListMyOrders(ctx fiber.Ctx, userId, cursorStr string, limit int) (PlusOrderHistoryResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.ListMyPlusOrders")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	span.SetAttributes(attribute.String("user.id", userId))

	if limit <= 0 || limit > shared.MAX_LIMIT {
		limit = shared.DEFAULT_LIMIT
	}

	var cursor PlusOrderCursor
	if cursorStr != "" {
		decoded, decErr := shared.DecodeCursor[PlusOrderCursor](cursorStr)
		if decErr != nil {
			err = &shared.BadRequestError{Code: shared.ERR_BAD_REQUEST_CODE, Message: "Invalid cursor", Param: "cursor"}
			return PlusOrderHistoryResponse{}, err
		}
		cursor = *decoded
	}

	items, lErr := service.Repo.ListOrdersByUser(ctxContext, userId, &cursor, limit+1)
	if lErr != nil {
		err = lErr
		return PlusOrderHistoryResponse{}, err
	}

	response := PlusOrderHistoryResponse{Data: []PlusOrderHistoryItem{}}
	if len(items) > limit {
		response.Data = items[:limit]
		last := items[limit-1]
		response.Page.NextCursor = shared.EncodeCursor(PlusOrderCursor{CreatedAt: last.CreatedAt, Id: last.Id})
	} else {
		response.Data = items
	}

	return response, nil
}

func (service *Service) GetOrderDetail(ctx fiber.Ctx, userId, orderId string) (PlusOrderDetailResponse, error) {
	ctxContext := ctx.Context()
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-service").Start(ctxContext, "service.GetPlusOrderDetail")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var response PlusOrderDetailResponse

	v := shared.NewValidator()
	v.UUID("orderId", orderId).Required()
	if err = v.Validate(); err != nil {
		return response, err
	}

	span.SetAttributes(attribute.String("user.id", userId), attribute.String("order.id", orderId))

	response, err = service.Repo.GetOrderDetailByIdForUser(ctxContext, orderId, userId)
	if err != nil {
		return response, err
	}
	return response, nil
}

func (service *Service) HandleWebhook(ctx context.Context, callbackToken string, rawPayload []byte) error {
	serviceName := service.Config.String("OTEL_SERVICE_NAME")
	ctx, span := otel.Tracer(serviceName+"-service").Start(ctx, "service.HandlePlusWebhook")
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctx, span, err)
		}
		span.End()
	}()

	expectedToken := service.Config.String("XENDIT_WEBHOOK_TOKEN")
	if subtle.ConstantTimeCompare([]byte(callbackToken), []byte(expectedToken)) != 1 {
		err = &shared.UnauthorizedError{Code: shared.ERR_UNAUTHORIZED_CODE, Message: "Invalid webhook token"}
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
		err = &shared.BadRequestError{Code: shared.ERR_BAD_REQUEST_CODE, Message: "Invalid webhook payload"}
		return err
	}

	eventId := envelope.Event + ":" + envelope.Data.PaymentId
	if envelope.Data.PaymentId == "" {
		eventId = envelope.Event + "-" + envelope.Data.ReferenceId
	}

	span.SetAttributes(
		attribute.String("xendit.event_type", envelope.Event),
		attribute.String("xendit.event_id", eventId),
	)

	now := time.Now().UTC()
	event := XenditWebhookEvent{
		Id:          uuid.New().String(),
		EventId:     eventId,
		EventType:   envelope.Event,
		ReferenceId: envelope.Data.ReferenceId,
		Payload:     rawPayload,
		Status:      "PENDING",
		ReceivedAt:  now,
	}
	inserted, insErr := service.Repo.InsertWebhookEventIdempotent(ctx, event)
	if insErr != nil {
		err = insErr
		return err
	}
	if !inserted {
		shared.GetLoggerWithTraceContext(ctx, service.Log).Info("Duplicate xendit webhook skipped", zap.String("event_id", eventId))
		return nil
	}

	parentSpanCtx := trace.SpanContextFromContext(ctx)
	eventType := envelope.Event
	referenceId := envelope.Data.ReferenceId
	paymentId := envelope.Data.PaymentId
	status := envelope.Data.Status
	go func() {
		bg := trace.ContextWithSpanContext(context.Background(), parentSpanCtx)
		bg, cancel := context.WithTimeout(bg, 30*time.Second)
		defer cancel()
		bg, gspan := otel.Tracer(service.Config.String("OTEL_SERVICE_NAME")+"-service").Start(bg, "service.dispatchPlusWebhook")
		defer gspan.End()
		logger := shared.GetLoggerWithTraceContext(bg, service.Log)
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic in plus webhook goroutine", zap.Any("recover", rec))
			}
		}()

		service.dispatchWebhook(bg, eventId, eventType, referenceId, paymentId, status, logger)
	}()

	return nil
}

func (service *Service) dispatchWebhook(ctx context.Context, eventId, eventType, referenceId, paymentId, status string, logger *zap.Logger) {
	processedAt := time.Now().UTC()

	switch eventType {
	case "payment.capture":
		if status != "SUCCEEDED" {
			logger.Info("payment.capture non-succeeded ignored", zap.String("status", status))
			_ = service.Repo.MarkWebhookProcessed(ctx, eventId, "PROCESSED", processedAt)
			return
		}
		order, oErr := service.Repo.GetOrderByReferenceId(ctx, referenceId)
		if oErr != nil {
			logger.Error("GetOrderByReferenceId failed", zap.Error(oErr))
			_ = service.Repo.MarkWebhookProcessed(ctx, eventId, "FAILED", processedAt)
			return
		}
		if order.Status == "PAID" {
			logger.Info("order already paid, skip", zap.String("order_id", order.Id))
			_ = service.Repo.MarkWebhookProcessed(ctx, eventId, "PROCESSED", processedAt)
			return
		}
		expiresAt := processedAt.AddDate(0, 0, shared.PLUS_DURATION_DAYS)
		affected, mErr := service.Repo.MarkOrderPaid(ctx, order.Id, paymentId, processedAt, expiresAt, processedAt)
		if mErr != nil {
			logger.Error("MarkOrderPaid failed", zap.Error(mErr))
			_ = service.Repo.MarkWebhookProcessed(ctx, eventId, "FAILED", processedAt)
			return
		}
		logger.Info("Virdan Plus granted", zap.String("order_id", order.Id), zap.Int64("rows", affected))
		_ = service.Repo.MarkWebhookProcessed(ctx, eventId, "PROCESSED", processedAt)

	case "payment.failure":
		order, oErr := service.Repo.GetOrderByReferenceId(ctx, referenceId)
		if oErr != nil {
			logger.Error("GetOrderByReferenceId failed", zap.Error(oErr))
			_ = service.Repo.MarkWebhookProcessed(ctx, eventId, "FAILED", processedAt)
			return
		}
		if fErr := service.Repo.MarkOrderFailed(ctx, order.Id, processedAt); fErr != nil {
			logger.Error("MarkOrderFailed failed", zap.Error(fErr))
			_ = service.Repo.MarkWebhookProcessed(ctx, eventId, "FAILED", processedAt)
			return
		}
		_ = service.Repo.MarkWebhookProcessed(ctx, eventId, "PROCESSED", processedAt)

	default:
		logger.Info("Unhandled xendit event type", zap.String("event_type", eventType))
		_ = service.Repo.MarkWebhookProcessed(ctx, eventId, "PROCESSED", processedAt)
	}
}

package payment

import (
	"strconv"

	"github.com/ferdian3456/virdanproject/shared"
	"github.com/gofiber/fiber/v3"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

type Controller struct {
	Service *Service
	Log     *zap.Logger
	Config  *koanf.Koanf
}

func NewController(service *Service, log *zap.Logger, config *koanf.Koanf) *Controller {
	return &Controller{
		Service: service,
		Log:     log,
		Config:  config,
	}
}

// GetPlusStatus godoc
// @Summary Get a server's Virdan Plus subscription status
// @description.markdown get_plus_status
// @Tags payment
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Success 200 {object} payment.PlusStatusResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/plus [get]
func (controller *Controller) GetPlusStatus(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetPlusStatus")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")

	var response PlusStatusResponse
	response, err = controller.Service.GetPlusStatus(ctx, userId, serverId)
	if err != nil {
		return shared.SendError(ctx, err)
	}
	return shared.SendSuccessResponseWithData(ctx, response)
}

// Checkout godoc
// @Summary Start a Virdan Plus checkout for a server
// @description.markdown plus_checkout
// @Tags payment
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Success 200 {object} payment.PlusCheckoutResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 409 {object} shared.ConflictError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/plus/checkout [post]
func (controller *Controller) Checkout(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.PlusCheckout")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")

	var response PlusCheckoutResponse
	response, err = controller.Service.Checkout(ctx, userId, serverId)
	if err != nil {
		return shared.SendError(ctx, err)
	}
	return shared.SendSuccessResponseWithData(ctx, response)
}

// ListMyOrders godoc
// @Summary List the logged-in user's Virdan Plus orders
// @description.markdown list_my_plus_orders
// @Tags payment
// @Produce json
// @Security BearerAuth
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Page size"
// @Success 200 {object} payment.PlusOrderHistoryResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /me/plus-orders [get]
func (controller *Controller) ListMyOrders(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.ListMyPlusOrders")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	cursor := ctx.Query("cursor", "")
	limitStr := ctx.Query("limit", "10")
	limit := 10
	if parsed, parseErr := strconv.Atoi(limitStr); parseErr == nil {
		limit = parsed
	}

	var response PlusOrderHistoryResponse
	response, err = controller.Service.ListMyOrders(ctx, userId, cursor, limit)
	if err != nil {
		return shared.SendError(ctx, err)
	}
	return shared.SendSuccessResponseWithData(ctx, response)
}

// GetOrderDetail godoc
// @Summary Get detail of one of the logged-in user's Virdan Plus orders
// @description.markdown get_plus_order_detail
// @Tags payment
// @Produce json
// @Security BearerAuth
// @Param orderId path string true "Order ID (UUID)"
// @Success 200 {object} payment.PlusOrderDetailResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 404 {object} shared.NotFoundError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /me/plus-orders/{orderId} [get]
func (controller *Controller) GetOrderDetail(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetPlusOrderDetail")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	orderId := ctx.Params("orderId")

	var response PlusOrderDetailResponse
	response, err = controller.Service.GetOrderDetail(ctx, userId, orderId)
	if err != nil {
		return shared.SendError(ctx, err)
	}
	return shared.SendSuccessResponseWithData(ctx, response)
}

// HandleWebhook godoc
// @Summary Handle Xendit payment webhook callback
// @description.markdown xendit_webhook
// @Tags payment
// @Accept json
// @Produce json
// @Param x-callback-token header string true "Xendit webhook callback token"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /webhooks/xendit [post]
func (controller *Controller) HandleWebhook(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.HandleXenditWebhook")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	callbackToken := ctx.Get("x-callback-token")
	rawBody := ctx.Body()

	if err = controller.Service.HandleWebhook(ctxContext, callbackToken, rawBody); err != nil {
		return shared.SendError(ctx, err)
	}
	return shared.SendSuccessResponseNoData(ctx)
}

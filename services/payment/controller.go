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

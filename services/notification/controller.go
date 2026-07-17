package notification

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

func (controller *Controller) RegisterDevice(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.RegisterDevice")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	var request DeviceTokenRegisterRequest
	if err = shared.ReadRequestBody(ctx, &request); err != nil {
		return shared.SendError(ctx, err)
	}

	if err = controller.Service.RegisterDevice(ctx, userId, request); err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseNoData(ctx)
}

func (controller *Controller) UnregisterDevice(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.UnregisterDevice")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	var request DeviceTokenDeleteRequest
	if err = shared.ReadRequestBody(ctx, &request); err != nil {
		return shared.SendError(ctx, err)
	}

	if err = controller.Service.UnregisterDevice(ctx, userId, request); err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseNoData(ctx)
}

func (controller *Controller) TestSend(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.TestSend")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	if err = controller.Service.TestSend(ctx, userId); err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseNoData(ctx)
}

func (controller *Controller) GetFeed(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetNotificationFeed")
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
	cursorStr := ctx.Query("cursor", "")
	limitStr := ctx.Query("limit", "10")

	limit := 10
	if parsed, parseErr := strconv.Atoi(limitStr); parseErr == nil {
		limit = parsed
	}

	var response NotificationListResponse
	response, err = controller.Service.GetFeed(ctx, userId, serverId, cursorStr, limit)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

func (controller *Controller) MarkRead(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.MarkNotificationRead")
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
	notifId := ctx.Params("id")

	if err = controller.Service.MarkRead(ctx, userId, serverId, notifId); err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseNoData(ctx)
}

func (controller *Controller) GetUnreadCount(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetUnreadNotificationCount")
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

	var response UnreadCountResponse
	response, err = controller.Service.GetUnreadCount(ctx, userId, serverId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

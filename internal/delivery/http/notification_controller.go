package http

import (
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/usecase"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/gofiber/fiber/v3"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

type NotificationController struct {
	NotificationUsecase *usecase.NotificationUsecase
	Log                 *zap.Logger
	Config              *koanf.Koanf
}

func NewNotificationController(notificationUsecase *usecase.NotificationUsecase, log *zap.Logger, config *koanf.Koanf) *NotificationController {
	return &NotificationController{
		NotificationUsecase: notificationUsecase,
		Log:                 log,
		Config:              config,
	}
}

// RegisterDevice godoc
// @Summary      Register device token for push notifications
// @Description.markdown register_device
// @Tags         notifications
// @Accept       json
// @Produce      json
// @Param        Authorization  header    string                          true  "Bearer access token"
// @Param        body           body      model.DeviceTokenRegisterRequest true  "Device token payload"
// @Success      200            {object}  map[string]string
// @Failure      400            {object}  model.BadRequestError
// @Failure      401            {object}  model.UnauthorizedError
// @Failure      500            {object}  model.BadRequestError
// @Router       /devices [post]
func (controller *NotificationController) RegisterDevice(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.RegisterDevice")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	var request model.DeviceTokenRegisterRequest
	if err = util.ReadRequestBody(ctx, &request); err != nil {
		return util.SendError(ctx, err)
	}

	if err = controller.NotificationUsecase.RegisterDevice(ctx, userId, request); err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// UnregisterDevice godoc
// @Summary      Unregister device token (call on logout)
// @Description.markdown unregister_device
// @Tags         notifications
// @Accept       json
// @Produce      json
// @Param        Authorization  header    string                          true  "Bearer access token"
// @Param        body           body      model.DeviceTokenDeleteRequest  true  "Device token payload"
// @Success      200            {object}  map[string]string
// @Failure      400            {object}  model.BadRequestError
// @Failure      401            {object}  model.UnauthorizedError
// @Failure      500            {object}  model.BadRequestError
// @Router       /devices [delete]
func (controller *NotificationController) UnregisterDevice(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.UnregisterDevice")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	var request model.DeviceTokenDeleteRequest
	if err = util.ReadRequestBody(ctx, &request); err != nil {
		return util.SendError(ctx, err)
	}

	if err = controller.NotificationUsecase.UnregisterDevice(ctx, userId, request); err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// TestSend godoc
// @Summary      Send test push notification to all registered devices
// @Description.markdown test_send_notification
// @Tags         notifications
// @Produce      json
// @Param        Authorization  header    string  true  "Bearer access token"
// @Success      200            {object}  map[string]string
// @Failure      401            {object}  model.UnauthorizedError
// @Failure      404            {object}  model.NotFoundError
// @Failure      500            {object}  model.BadRequestError
// @Router       /notifications/test-send [post]
func (controller *NotificationController) TestSend(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.TestSend")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	if err = controller.NotificationUsecase.TestSend(ctx, userId); err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

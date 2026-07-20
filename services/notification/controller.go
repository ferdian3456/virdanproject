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

// RegisterDevice godoc
// @Summary Register a push notification device token
// @description.markdown register_device
// @Tags notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body DeviceTokenRegisterRequest true "Device token payload"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /devices/ [post]
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

// UnregisterDevice godoc
// @Summary Unregister a push notification device token
// @description.markdown unregister_device
// @Tags notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body DeviceTokenDeleteRequest true "Device token to remove"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /devices/ [delete]
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

// TestSend godoc
// @Summary Send a test push notification to the caller's own registered devices
// @description.markdown test_send_notification
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 404 {object} shared.NotFoundError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /notifications/test-send [post]
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

// GetFeed godoc
// @Summary Get the caller's notification feed for a server
// @description.markdown get_notification_feed
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Page size"
// @Success 200 {object} NotificationListResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/notifications [get]
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

// MarkRead godoc
// @Summary Mark a single notification as read
// @description.markdown mark_notification_read
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param id path string true "Notification ID (UUID)"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/notifications/{id}/read [post]
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

// GetUnreadCount godoc
// @Summary Get the caller's unread notification count for a server
// @description.markdown get_unread_notification_count
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Success 200 {object} UnreadCountResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/notifications/unread-count [get]
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

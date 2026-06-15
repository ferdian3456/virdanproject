package http

import (
	"strconv"

	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/usecase"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/gofiber/fiber/v3"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

type ServerPlusController struct {
	ServerPlusUsecase *usecase.ServerPlusUsecase
	Log               *zap.Logger
	Config            *koanf.Koanf
}

func NewServerPlusController(serverPlusUsecase *usecase.ServerPlusUsecase, log *zap.Logger, config *koanf.Koanf) *ServerPlusController {
	return &ServerPlusController{
		ServerPlusUsecase: serverPlusUsecase,
		Log:               log,
		Config:            config,
	}
}

// GetPlusStatus godoc
// @Summary      Get server Virdan Plus status + price
// @Tags         plus
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        serverId path string true "Server UUID"
// @Success      200 {object} model.PlusStatusResponse
// @Failure      403 {object} model.ValidationError
// @Failure      404 {object} model.ValidationError
// @Router       /servers/{serverId}/plus [get]
func (controller *ServerPlusController) GetPlusStatus(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetPlusStatus")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")

	var response model.PlusStatusResponse
	response, err = controller.ServerPlusUsecase.GetPlusStatus(ctx, userId, serverId)
	if err != nil {
		return util.SendError(ctx, err)
	}
	return util.SendSuccessResponseWithData(ctx, response)
}

// Checkout godoc
// @Summary      Start Virdan Plus checkout (Xendit payment session)
// @Tags         plus
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        serverId path string true "Server UUID"
// @Success      200 {object} model.PlusCheckoutResponse
// @Failure      403 {object} model.ValidationError
// @Failure      409 {object} model.ValidationError
// @Router       /servers/{serverId}/plus/checkout [post]
func (controller *ServerPlusController) Checkout(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.PlusCheckout")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")

	var response model.PlusCheckoutResponse
	response, err = controller.ServerPlusUsecase.Checkout(ctx, userId, serverId)
	if err != nil {
		return util.SendError(ctx, err)
	}
	return util.SendSuccessResponseWithData(ctx, response)
}

// ListMyOrders godoc
// @Summary      List my Virdan Plus payment history (global)
// @Tags         plus
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        cursor query string false "Pagination cursor"
// @Param        limit query int false "Page size (max 20)"
// @Success      200 {object} model.PlusOrderHistoryResponse
// @Router       /me/plus-orders [get]
func (controller *ServerPlusController) ListMyOrders(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.ListMyPlusOrders")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
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

	var response model.PlusOrderHistoryResponse
	response, err = controller.ServerPlusUsecase.ListMyOrders(ctx, userId, cursor, limit)
	if err != nil {
		return util.SendError(ctx, err)
	}
	return util.SendSuccessResponseWithData(ctx, response)
}

// HandleWebhook godoc — no auth middleware; the token is verified inside the usecase.
// @Summary      Xendit webhook receiver
// @Tags         plus
// @Accept       json
// @Produce      json
// @Param        x-callback-token header string true "Xendit callback token"
// @Success      200 {object} map[string]string
// @Failure      401 {object} model.ValidationError
// @Router       /webhooks/xendit [post]
func (controller *ServerPlusController) HandleWebhook(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.HandleXenditWebhook")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	callbackToken := ctx.Get("x-callback-token")
	rawBody := ctx.Body()

	if err = controller.ServerPlusUsecase.HandleWebhook(ctxContext, callbackToken, rawBody); err != nil {
		return util.SendError(ctx, err)
	}
	return util.SendSuccessResponseNoData(ctx)
}

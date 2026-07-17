package chat

import (
	"github.com/ferdian3456/virdanproject/shared"
	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

type Controller struct {
	Log     *zap.Logger
	Config  *koanf.Koanf
	Service *Service
	Hub     *shared.WsHub
}

func NewController(log *zap.Logger, config *koanf.Koanf, service *Service, hub *shared.WsHub) *Controller {
	return &Controller{Log: log, Config: config, Service: service, Hub: hub}
}

func (controller *Controller) HandleWS(conn *websocket.Conn) {
	userId, ok := conn.Locals("userId").(string)
	if !ok || userId == "" {
		_ = conn.Close()
		return
	}
	controller.Hub.Serve(conn, userId, controller.Service.HandleInboundFrame, controller.Service.BroadcastPresence)
}

func (controller *Controller) GetOrCreateConversation(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetOrCreateConversation")
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
	var payload GetOrCreateConversationRequest
	if err = shared.ReadRequestBody(ctx, &payload); err != nil {
		return shared.SendError(ctx, err)
	}
	var resp DmConversationResponse
	resp, err = controller.Service.GetOrCreateConversation(ctx, serverId, userId, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}
	return shared.SendSuccessResponseWithData(ctx, resp)
}

func (controller *Controller) ListMembers(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.ListMembers")
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
	q := ctx.Query("q")
	cursor := ctx.Query("cursor")
	limitStr := ctx.Query("limit")

	var resp DmMemberListResponse
	resp, err = controller.Service.ListMembers(ctx, serverId, userId, q, cursor, limitStr)
	if err != nil {
		return shared.SendError(ctx, err)
	}
	return shared.SendSuccessResponseWithData(ctx, resp)
}

func (controller *Controller) SendMessage(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.SendMessage")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	conversationId := ctx.Params("conversationId")
	var payload SendMessageRequest
	if err = shared.ReadRequestBody(ctx, &payload); err != nil {
		return shared.SendError(ctx, err)
	}
	var resp DmMessageResponse
	resp, err = controller.Service.SendMessage(ctx, conversationId, userId, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}
	return shared.SendSuccessResponseWithData(ctx, resp)
}

func (controller *Controller) ListMessages(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.ListMessages")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	conversationId := ctx.Params("conversationId")
	cursor := ctx.Query("cursor")
	limitStr := ctx.Query("limit")

	var resp DmMessageListResponse
	resp, err = controller.Service.ListMessages(ctx, conversationId, userId, cursor, limitStr)
	if err != nil {
		return shared.SendError(ctx, err)
	}
	return shared.SendSuccessResponseWithData(ctx, resp)
}

func (controller *Controller) MarkRead(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.MarkRead")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	conversationId := ctx.Params("conversationId")
	var payload MarkReadRequest
	if err = shared.ReadRequestBody(ctx, &payload); err != nil {
		return shared.SendError(ctx, err)
	}
	if err = controller.Service.MarkRead(ctx, conversationId, userId, payload); err != nil {
		return shared.SendError(ctx, err)
	}
	return shared.SendSuccessResponseNoData(ctx)
}

func (controller *Controller) ListConversations(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.ListConversations")
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
	cursor := ctx.Query("cursor")
	limitStr := ctx.Query("limit")

	var resp DmConversationListResponse
	resp, err = controller.Service.ListConversations(ctx, serverId, userId, cursor, limitStr)
	if err != nil {
		return shared.SendError(ctx, err)
	}
	return shared.SendSuccessResponseWithData(ctx, resp)
}

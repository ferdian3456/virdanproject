package http

import (
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/usecase"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/ferdian3456/virdanproject/internal/ws"
	"github.com/gofiber/fiber/v3"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

type ChatController struct {
	Log         *zap.Logger
	Config      *koanf.Koanf
	ChatUsecase *usecase.ChatUsecase
	Hub         *ws.Hub
}

func NewChatController(log *zap.Logger, config *koanf.Koanf, chatUsecase *usecase.ChatUsecase, hub *ws.Hub) *ChatController {
	return &ChatController{Log: log, Config: config, ChatUsecase: chatUsecase, Hub: hub}
}

func (controller *ChatController) GetOrCreateConversation(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetOrCreateConversation")
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
	var payload model.GetOrCreateConversationRequest
	if err = util.ReadRequestBody(ctx, &payload); err != nil {
		return util.SendError(ctx, err)
	}
	var resp model.DmConversationResponse
	resp, err = controller.ChatUsecase.GetOrCreateConversation(ctx, serverId, userId, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}
	return util.SendSuccessResponseWithData(ctx, resp)
}

func (controller *ChatController) ListMembers(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.ListMembers")
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
	q := ctx.Query("q")
	cursor := ctx.Query("cursor")
	limitStr := ctx.Query("limit")

	var resp model.DmMemberListResponse
	resp, err = controller.ChatUsecase.ListMembers(ctx, serverId, userId, q, cursor, limitStr)
	if err != nil {
		return util.SendError(ctx, err)
	}
	return util.SendSuccessResponseWithData(ctx, resp)
}

func (controller *ChatController) SendMessage(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.SendMessage")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	conversationId := ctx.Params("conversationId")
	var payload model.SendMessageRequest
	if err = util.ReadRequestBody(ctx, &payload); err != nil {
		return util.SendError(ctx, err)
	}
	var resp model.DmMessageResponse
	resp, err = controller.ChatUsecase.SendMessage(ctx, conversationId, userId, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}
	return util.SendSuccessResponseWithData(ctx, resp)
}

func (controller *ChatController) ListMessages(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.ListMessages")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	conversationId := ctx.Params("conversationId")
	cursor := ctx.Query("cursor")
	limitStr := ctx.Query("limit")

	var resp model.DmMessageListResponse
	resp, err = controller.ChatUsecase.ListMessages(ctx, conversationId, userId, cursor, limitStr)
	if err != nil {
		return util.SendError(ctx, err)
	}
	return util.SendSuccessResponseWithData(ctx, resp)
}

func (controller *ChatController) MarkRead(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.MarkRead")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	conversationId := ctx.Params("conversationId")
	var payload model.MarkReadRequest
	if err = util.ReadRequestBody(ctx, &payload); err != nil {
		return util.SendError(ctx, err)
	}
	if err = controller.ChatUsecase.MarkRead(ctx, conversationId, userId, payload); err != nil {
		return util.SendError(ctx, err)
	}
	return util.SendSuccessResponseNoData(ctx)
}

func (controller *ChatController) ListConversations(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.ListConversations")
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
	cursor := ctx.Query("cursor")
	limitStr := ctx.Query("limit")

	var resp model.DmConversationListResponse
	resp, err = controller.ChatUsecase.ListConversations(ctx, serverId, userId, cursor, limitStr)
	if err != nil {
		return util.SendError(ctx, err)
	}
	return util.SendSuccessResponseWithData(ctx, resp)
}

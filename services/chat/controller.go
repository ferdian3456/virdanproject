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

// GetOrCreateConversation godoc
// @Summary Get or create a direct-message conversation with another server member
// @description.markdown get_or_create_conversation
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param request body chat.GetOrCreateConversationRequest true "Peer user ID"
// @Success 200 {object} chat.DmConversationResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/conversations [post]
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

// ListMembers godoc
// @Summary List server members available to start a DM conversation with
// @description.markdown list_dm_members
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param q query string false "Search query (nickname)"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Page size"
// @Success 200 {object} chat.DmMemberListResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/members/dm [get]
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

// SendMessage godoc
// @Summary Send a direct message in a conversation
// @description.markdown send_dm_message
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param conversationId path string true "Conversation ID (UUID)"
// @Param request body chat.SendMessageRequest true "Message content"
// @Success 200 {object} chat.DmMessageResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 404 {object} shared.NotFoundError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /conversations/{conversationId}/messages [post]
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

// ListMessages godoc
// @Summary List messages in a direct-message conversation
// @description.markdown list_dm_messages
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param conversationId path string true "Conversation ID (UUID)"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Page size"
// @Success 200 {object} chat.DmMessageListResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 404 {object} shared.NotFoundError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /conversations/{conversationId}/messages [get]
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

// MarkRead godoc
// @Summary Mark a direct-message conversation as read up to a given message
// @description.markdown mark_conversation_read
// @Tags chat
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param conversationId path string true "Conversation ID (UUID)"
// @Param request body chat.MarkReadRequest true "Last read message ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 404 {object} shared.NotFoundError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /conversations/{conversationId}/read [post]
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

// ListConversations godoc
// @Summary List the caller's direct-message conversations in a server
// @description.markdown list_conversations
// @Tags chat
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Page size"
// @Success 200 {object} chat.DmConversationListResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/conversations [get]
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

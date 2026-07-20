package server

import (
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

// CreateServer godoc
// @Summary Create a new server
// @description.markdown create_server
// @Tags servers
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param name formData string true "Server name"
// @Param shortName formData string true "Short name (2-10 chars)"
// @Param description formData string false "Server description"
// @Param categoryId formData int true "Category ID"
// @Param isPrivate formData bool true "Whether the server is private"
// @Param nickname formData string true "Owner's nickname for this server"
// @Param username formData string true "Owner's username for this server"
// @Param bio formData string false "Owner's per-server bio"
// @Param serverAvatar formData file false "Server avatar image"
// @Param profileAvatar formData file false "Owner's profile avatar for this server"
// @Param avatarImageId formData string false "Reuse an existing profile avatar image UUID"
// @Success 200 {object} server.ServerCreateResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 404 {object} shared.NotFoundError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/create [post]
func (controller *Controller) CreateServer(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.CreateServer")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	err = shared.ReadMultipartBody(ctx)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	userId := ctx.Locals("userId").(string)

	var response ServerCreateResponse
	response, err = controller.Service.CreateServer(ctx, userId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// GetDiscoveryServer godoc
// @Summary Discover public servers
// @description.markdown get_discovery_server
// @Tags servers
// @Produce json
// @Security BearerAuth
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Page size"
// @Param categoryId query int false "Filter by category ID"
// @Success 200 {object} server.DiscoveryServerResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/ [get]
func (controller *Controller) GetDiscoveryServer(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetDiscoveryServer")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	cursor := ctx.Query("cursor")
	limitStr := ctx.Query("limit")
	categoryStr := ctx.Query("categoryId")

	var response DiscoveryServerResponse
	response, err = controller.Service.GetDiscoveryServer(ctx, userId, cursor, limitStr, categoryStr)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// GetUserServer godoc
// @Summary List servers the logged-in user is a member of
// @description.markdown get_user_server
// @Tags servers
// @Produce json
// @Security BearerAuth
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Page size"
// @Success 200 {object} server.ServerUserListResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/me [get]
func (controller *Controller) GetUserServer(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetUserServer")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	cursor := ctx.Query("cursor")
	limitStr := ctx.Query("limit")

	var response ServerUserListResponse
	response, err = controller.Service.GetUserServer(ctx, userId, cursor, limitStr)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// GetCategoryServer godoc
// @Summary List server categories
// @description.markdown get_server_categories
// @Tags servers
// @Produce json
// @Security BearerAuth
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Page size"
// @Success 200 {object} server.ServerCategoryListResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/categories [get]
func (controller *Controller) GetCategoryServer(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetCategoryServer")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	cursor := ctx.Query("cursor")
	limitStr := ctx.Query("limit")

	var response ServerCategoryListResponse
	response, err = controller.Service.GetCategoryServer(ctx, cursor, limitStr)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// GetServerById godoc
// @Summary Get a server's detail by ID
// @description.markdown get_server_by_id
// @Tags servers
// @Produce json
// @Security BearerAuth
// @Param id path string true "Server ID (UUID)"
// @Success 200 {object} server.ServerDetailResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{id} [get]
func (controller *Controller) GetServerById(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetServerById")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("id")

	var response ServerDetailResponse
	response, err = controller.Service.GetServerById(ctx, userId, serverId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// JoinServer godoc
// @Summary Join a public server directly by ID
// @description.markdown join_server
// @Tags servers
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param nickname formData string true "Nickname for this server"
// @Param username formData string true "Username for this server"
// @Param bio formData string false "Per-server bio"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 404 {object} shared.NotFoundError
// @Failure 409 {object} shared.ConflictError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/join [post]
func (controller *Controller) JoinServer(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.JoinServer")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	err = shared.ReadMultipartBody(ctx)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")

	err = controller.Service.JoinServer(ctx, userId, serverId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseNoData(ctx)
}

// JoinServerFromInvite godoc
// @Summary Join a server using an invite code
// @description.markdown join_server_from_invite
// @Tags servers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body server.ServerJoinByInviteRequest true "Invite code + profile payload"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 409 {object} shared.ConflictError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/join [post]
func (controller *Controller) JoinServerFromInvite(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.JoinServerFromInvite")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	var payload ServerJoinByInviteRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	err = controller.Service.JoinServerFromInvite(ctx, userId, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseNoData(ctx)
}

// GetServerInfoForInvite godoc
// @Summary Get server preview info for an invite code (public)
// @description.markdown get_invite_info
// @Tags servers
// @Produce json
// @Param inviteCode path string true "Invite code"
// @Success 200 {object} server.ServerInfoForInviteResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 404 {object} shared.NotFoundError
// @Router /servers/invites/{inviteCode} [get]
func (controller *Controller) GetServerInfoForInvite(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetServerInfoForInvite")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	inviteCode := ctx.Params("inviteCode")

	var response ServerInfoForInviteResponse
	response, err = controller.Service.GetServerInfoForInvite(ctx, inviteCode)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// UpdateServerName godoc
// @Summary Update a server's name (owner only)
// @description.markdown update_server_name
// @Tags servers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Server ID (UUID)"
// @Param request body server.ServerUpdateNameRequest true "New name"
// @Success 200 {object} server.ServerUpdateResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{id}/name [put]
func (controller *Controller) UpdateServerName(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.UpdateServerName")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("id")

	var payload ServerUpdateNameRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	var response ServerUpdateResponse
	response, err = controller.Service.UpdateServerName(ctx, userId, serverId, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// UpdateServerShortName godoc
// @Summary Update a server's short name (owner only)
// @description.markdown update_server_short_name
// @Tags servers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Server ID (UUID)"
// @Param request body server.ServerUpdateShortNameRequest true "New short name"
// @Success 200 {object} server.ServerUpdateResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{id}/shortName [put]
func (controller *Controller) UpdateServerShortName(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.UpdateServerShortName")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("id")

	var payload ServerUpdateShortNameRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	var response ServerUpdateResponse
	response, err = controller.Service.UpdateServerShortName(ctx, userId, serverId, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// UpdateServerCategory godoc
// @Summary Update a server's category (owner only)
// @description.markdown update_server_category
// @Tags servers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Server ID (UUID)"
// @Param request body server.ServerUpdateCategoryRequest true "New category ID"
// @Success 200 {object} server.ServerUpdateResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 404 {object} shared.NotFoundError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{id}/category [put]
func (controller *Controller) UpdateServerCategory(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.UpdateServerCategory")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("id")

	var payload ServerUpdateCategoryRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	var response ServerUpdateResponse
	response, err = controller.Service.UpdateServerCategory(ctx, userId, serverId, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// UpdateServerDescription godoc
// @Summary Update a server's description (owner only)
// @description.markdown update_server_description
// @Tags servers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Server ID (UUID)"
// @Param request body server.ServerUpdateDescriptionRequest true "New description"
// @Success 200 {object} server.ServerUpdateResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{id}/description [put]
func (controller *Controller) UpdateServerDescription(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.UpdateServerDescription")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("id")

	var payload ServerUpdateDescriptionRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	var response ServerUpdateResponse
	response, err = controller.Service.UpdateServerDescription(ctx, userId, serverId, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// UpdateServerSettings godoc
// @Summary Update a server's settings (owner only)
// @description.markdown update_server_settings
// @Tags servers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Server ID (UUID)"
// @Param request body server.ServerUpdateSettingsRequest true "New settings"
// @Success 200 {object} server.ServerUpdateResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{id}/settings [put]
func (controller *Controller) UpdateServerSettings(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.UpdateServerSettings")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("id")

	var payload ServerUpdateSettingsRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	var response ServerUpdateResponse
	response, err = controller.Service.UpdateServerSettings(ctx, userId, serverId, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// UpdateServerAvatar godoc
// @Summary Update a server's avatar (owner only)
// @description.markdown update_server_avatar
// @Tags servers
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path string true "Server ID (UUID)"
// @Param avatar formData file true "Avatar image"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{id}/avatar [put]
func (controller *Controller) UpdateServerAvatar(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.UpdateServerAvatar")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	err = shared.ReadMultipartBody(ctx)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("id")

	err = controller.Service.UpdateServerAvatar(ctx, userId, serverId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseNoData(ctx)
}

// UpdateServerBanner godoc
// @Summary Update a server's banner (owner only)
// @description.markdown update_server_banner
// @Tags servers
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path string true "Server ID (UUID)"
// @Param banner formData file true "Banner image"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{id}/banner [put]
func (controller *Controller) UpdateServerBanner(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.UpdateServerBanner")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	err = shared.ReadMultipartBody(ctx)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("id")

	err = controller.Service.UpdateServerBanner(ctx, userId, serverId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseNoData(ctx)
}

// DeleteServer godoc
// @Summary Delete a server permanently (owner only)
// @description.markdown delete_server
// @Tags servers
// @Produce json
// @Security BearerAuth
// @Param id path string true "Server ID (UUID)"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{id} [delete]
func (controller *Controller) DeleteServer(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.DeleteServer")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("id")

	err = controller.Service.DeleteServer(ctx, userId, serverId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseNoData(ctx)
}

// LeaveServer godoc
// @Summary Leave a server (sole owner leaving deletes the server)
// @description.markdown leave_server
// @Tags servers
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 404 {object} shared.NotFoundError
// @Failure 409 {object} shared.ConflictError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/membership [delete]
func (controller *Controller) LeaveServer(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.LeaveServer")
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

	err = controller.Service.LeaveServer(ctx, userId, serverId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseNoData(ctx)
}

// CreateInviteLink godoc
// @Summary Create an invite link for a server
// @description.markdown create_invite_link
// @Tags servers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param request body server.ServerInviteLinkRequest true "Invite options"
// @Success 200 {object} server.ServerInviteLinkResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/invites [post]
func (controller *Controller) CreateInviteLink(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.CreateInviteLink")
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

	var payload ServerInviteLinkRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	var response ServerInviteLinkResponse
	response, err = controller.Service.CreateInviteLink(ctx, userId, serverId, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// GetServerMembers godoc
// @Summary List a server's members
// @description.markdown get_server_members
// @Tags servers
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Page size"
// @Success 200 {object} server.ServerMemberListResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/members [get]
func (controller *Controller) GetServerMembers(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetServerMembers")
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

	var response ServerMemberListResponse
	response, err = controller.Service.GetServerMembers(ctx, serverId, userId, cursor, limitStr)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// GetMyRoleInServer godoc
// @Summary Get the logged-in user's role in a server
// @description.markdown get_my_role_in_server
// @Tags servers
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/members/me [get]
func (controller *Controller) GetMyRoleInServer(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetMyRoleInServer")
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

	var roleName string
	roleName, err = controller.Service.GetMyRoleInServer(ctx, serverId, userId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, fiber.Map{"role": roleName})
}

// KickMember godoc
// @Summary Kick a member from a server (Owner/Admin only)
// @description.markdown kick_member
// @Tags servers
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param userId path string true "Target user ID (UUID)"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 404 {object} shared.NotFoundError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/members/{userId} [delete]
func (controller *Controller) KickMember(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.KickMember")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	callerId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")
	targetUserId := ctx.Params("userId")

	err = controller.Service.KickMember(ctx, serverId, targetUserId, callerId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseNoData(ctx)
}

// AssignMemberRole godoc
// @Summary Assign Admin or Member role to a member (owner only)
// @description.markdown assign_member_role
// @Tags servers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param userId path string true "Target user ID (UUID)"
// @Param request body server.AssignMemberRoleRequest true "New role"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 404 {object} shared.NotFoundError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/members/{userId}/role [put]
func (controller *Controller) AssignMemberRole(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.AssignMemberRole")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	callerId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")
	targetUserId := ctx.Params("userId")

	var payload AssignMemberRoleRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	err = controller.Service.AssignMemberRole(ctx, serverId, targetUserId, callerId, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseNoData(ctx)
}

// TransferOwnership godoc
// @Summary Transfer server ownership to another member (owner only)
// @description.markdown transfer_ownership
// @Tags servers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param request body server.TransferOwnershipRequest true "New owner user ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 404 {object} shared.NotFoundError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/ownership [put]
func (controller *Controller) TransferOwnership(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.TransferOwnership")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	callerId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")

	var payload TransferOwnershipRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	err = controller.Service.TransferOwnership(ctx, serverId, callerId, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseNoData(ctx)
}

// GetProfileHistory godoc
// @Summary Get the logged-in user's cross-server profile history
// @description.markdown get_profile_history
// @Tags servers
// @Produce json
// @Security BearerAuth
// @Success 200 {object} server.GetProfileHistoryResponse
// @Failure 401 {object} shared.UnauthorizedError
// @Router /profiles/history [get]
func (controller *Controller) GetProfileHistory(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetProfileHistory")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	var response GetProfileHistoryResponse
	response, err = controller.Service.GetProfileHistory(ctx, userId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// GetServerProfileMe godoc
// @Summary Get the logged-in user's per-server profile
// @description.markdown get_server_profile_me
// @Tags servers
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Success 200 {object} server.ServerMemberProfileResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/profile/me [get]
func (controller *Controller) GetServerProfileMe(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetServerProfileMe")
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

	var response ServerMemberProfileResponse
	response, err = controller.Service.GetServerProfileMe(ctx, userId, serverId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// UpdateServerProfile godoc
// @Summary Update the logged-in user's per-server profile
// @description.markdown update_server_profile
// @Tags servers
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param nickname formData string false "New nickname"
// @Param username formData string false "New username"
// @Param bio formData string false "New bio"
// @Param profileAvatar formData file false "New profile avatar image"
// @Param avatarImageId formData string false "Reuse an existing profile avatar image UUID"
// @Success 200 {object} server.ServerProfileUpdateResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/profile [put]
func (controller *Controller) UpdateServerProfile(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.UpdateServerProfile")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	err = shared.ReadMultipartBody(ctx)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")

	var response ServerProfileUpdateResponse
	response, err = controller.Service.UpdateServerProfile(ctx, userId, serverId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// GetServerProfileByUserId godoc
// @Summary Get another member's per-server profile
// @description.markdown get_server_member_profile
// @Tags servers
// @Produce json
// @Security BearerAuth
// @Param serverId path string true "Server ID (UUID)"
// @Param userId path string true "Target user ID (UUID)"
// @Success 200 {object} server.ServerMemberProfileResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 403 {object} shared.ForbiddenError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /servers/{serverId}/members/{userId}/profile [get]
func (controller *Controller) GetServerProfileByUserId(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetServerProfileByUserId")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	requesterUserId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")
	targetUserId := ctx.Params("userId")

	var response ServerMemberProfileResponse
	response, err = controller.Service.GetServerProfileByUserId(ctx, requesterUserId, serverId, targetUserId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

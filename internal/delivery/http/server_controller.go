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

type ServerController struct {
	ServerUsecase *usecase.ServerUsecase
	Log           *zap.Logger
	Config        *koanf.Koanf
}

func NewServerController(serverUsecase *usecase.ServerUsecase, zap *zap.Logger, koanf *koanf.Koanf) *ServerController {
	return &ServerController{
		ServerUsecase: serverUsecase,
		Log:           zap,
		Config:        koanf,
	}
}

// CreateServer godoc
// @Summary      Create a new server
// @Description.markdown create_server
// @Tags         servers
// @Accept       multipart/form-data
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        name formData string true "Server name"
// @Param        shortName formData string true "Short name (max 10 chars)"
// @Param        categoryId formData int true "Category ID"
// @Param        description formData string false "Description"
// @Param        isPrivate formData bool false "Private server"
// @Param        nickname formData string true "Your nickname in this server"
// @Param        bio formData string false "Your bio in this server"
// @Param        avatarImageId formData string false "Existing avatar image ID"
// @Param        serverAvatar formData file false "Server avatar image"
// @Param        profileAvatar formData file false "New profile avatar image (per-server)"
// @Success      200   {object}  model.ServerCreateResponse
// @Failure      400   {object}  model.ValidationError
// @Failure      500   {object}  model.ValidationError
// @Router       /servers/create [post]
func (controller *ServerController) CreateServer(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.CreateServer")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	err = util.ReadMultipartBody(ctx)
	if err != nil {
		return util.SendError(ctx, err)
	}

	userId := ctx.Locals("userId").(string)

	var response model.ServerCreateResponse
	response, err = controller.ServerUsecase.CreateServer(ctx, userId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// GetDiscoveryServer godoc
// @Summary      Get discoverable servers
// @Description.markdown get_discovery_server
// @Tags         servers
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        limit query int false "Items per page"
// @Param        categoryId query int false "Filter by category ID"
// @Param        cursor query string false "Pagination cursor"
// @Success      200   {object}  model.DiscoveryServerResponse
// @Failure      400   {object}  model.ValidationError
// @Failure      500   {object}  model.ValidationError
// @Router       /servers/ [get]
func (controller *ServerController) GetDiscoveryServer(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.GetDiscoveryServer")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	cursor := ctx.Query("cursor")
	limitStr := ctx.Query("limit")
	categoryStr := ctx.Query("categoryId")

	var response model.DiscoveryServerResponse
	response, err = controller.ServerUsecase.GetDiscoveryServer(ctx, userId, cursor, limitStr, categoryStr)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// GetUserServer godoc
// @Summary      Get servers the user belongs to
// @Description.markdown get_user_server
// @Tags         servers
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        limit query int false "Items per page"
// @Param        cursor query string false "Pagination cursor"
// @Success      200   {object}  model.ServerUserListResponse
// @Failure      400   {object}  model.ValidationError
// @Failure      500   {object}  model.ValidationError
// @Router       /servers/me [get]
func (controller *ServerController) GetUserServer(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.GetUserServer")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	cursor := ctx.Query("cursor")
	limitStr := ctx.Query("limit")

	var response model.ServerUserListResponse
	response, err = controller.ServerUsecase.GetUserServer(ctx, userId, cursor, limitStr)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// GetCategoryServer godoc
// @Summary      Get server categories
// @Description.markdown get_category_server
// @Tags         servers
// @Produce      json
// @Param        limit query int false "Items per page"
// @Param        cursor query string false "Pagination cursor"
// @Success      200   {object}  model.ServerCategoryListResponse
// @Failure      400   {object}  model.ValidationError
// @Failure      500   {object}  model.ValidationError
// @Router       /servers/categories [get]
func (controller *ServerController) GetCategoryServer(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.GetCategoryServer")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	cursor := ctx.Query("cursor")
	limitStr := ctx.Query("limit")

	var response model.ServerCategoryListResponse
	response, err = controller.ServerUsecase.GetCategoryServer(ctx, cursor, limitStr)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// GetServerById godoc
// @Summary      Get server by ID
// @Description.markdown get_server_by_id
// @Tags         servers
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        id path string true "Server UUID"
// @Success      200   {object}  model.ServerDetailResponse
// @Failure      400   {object}  model.ValidationError
// @Failure      404   {object}  model.ValidationError
// @Failure      500   {object}  model.ValidationError
// @Router       /servers/{id} [get]
func (controller *ServerController) GetServerById(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.GetServerById")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("id")

	var response model.ServerDetailResponse
	response, err = controller.ServerUsecase.GetServerById(ctx, userId, serverId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// JoinServer godoc
// @Summary      Join a public server
// @Description.markdown join_server
// @Tags         servers
// @Accept       multipart/form-data
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        serverId path string true "Server UUID"
// @Param        nickname formData string true "Your nickname in this server"
// @Param        bio formData string false "Your bio in this server"
// @Param        avatarImageId formData string false "Existing avatar image ID (picker)"
// @Param        profileAvatar formData file false "New profile avatar image (per-server)"
// @Success      200
// @Failure      400   {object}  model.ValidationError
// @Failure      409   {object}  model.ValidationError
// @Failure      500   {object}  model.ValidationError
// @Router       /servers/{serverId}/join [post]
func (controller *ServerController) JoinServer(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.JoinServer")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	err = util.ReadMultipartBody(ctx)
	if err != nil {
		return util.SendError(ctx, err)
	}

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")

	err = controller.ServerUsecase.JoinServer(ctx, userId, serverId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// JoinServerFromInvite godoc
// @Summary      Join server using invite code
// @Description.markdown join_server_from_invite
// @Tags         server-invites
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        body body model.ServerJoinByInviteRequest true "Payload"
// @Success      200
// @Failure      400   {object}  model.ValidationError
// @Failure      409   {object}  model.ValidationError
// @Failure      500   {object}  model.ValidationError
// @Router       /servers/join [post]
func (controller *ServerController) JoinServerFromInvite(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.JoinServerFromInvite")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	var payload model.ServerJoinByInviteRequest
	err = util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	err = controller.ServerUsecase.JoinServerFromInvite(ctx, userId, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// GetServerInfoForInvite godoc
// @Summary      Get server info for invite preview
// @Description.markdown get_server_info_for_invite
// @Tags         server-invites
// @Produce      json
// @Param        inviteCode path string true "Invite code (8 characters)"
// @Success      200   {object}  model.ServerInfoForInviteResponse
// @Failure      400   {object}  model.ValidationError
// @Failure      404   {object}  model.ValidationError
// @Failure      500   {object}  model.ValidationError
// @Router       /servers/invites/{inviteCode} [get]
func (controller *ServerController) GetServerInfoForInvite(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.GetServerInfoForInvite")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	inviteCode := ctx.Params("inviteCode")

	var response model.ServerInfoForInviteResponse
	response, err = controller.ServerUsecase.GetServerInfoForInvite(ctx, inviteCode)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// UpdateServerName godoc
// @Summary      Update server name
// @Description.markdown update_server_name
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        id path string true "Server UUID"
// @Param        body body model.ServerUpdateNameRequest true "Payload"
// @Success      200   {object}  model.ServerUpdateResponse
// @Failure      400   {object}  model.ValidationError
// @Failure      403   {object}  model.ValidationError
// @Failure      500   {object}  model.ValidationError
// @Router       /servers/{id}/name [put]
func (controller *ServerController) UpdateServerName(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.UpdateServerName")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("id")

	var payload model.ServerUpdateNameRequest
	err = util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	var response model.ServerUpdateResponse
	response, err = controller.ServerUsecase.UpdateServerName(ctx, userId, serverId, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// UpdateServerShortName godoc
// @Summary      Update server short name
// @Description.markdown update_server_short_name
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        id path string true "Server UUID"
// @Param        body body model.ServerUpdateShortNameRequest true "Payload"
// @Success      200   {object}  model.ServerUpdateResponse
// @Failure      400   {object}  model.ValidationError
// @Failure      403   {object}  model.ValidationError
// @Failure      500   {object}  model.ValidationError
// @Router       /servers/{id}/shortName [put]
func (controller *ServerController) UpdateServerShortName(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.UpdateServerShortName")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("id")

	var payload model.ServerUpdateShortNameRequest
	err = util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	var response model.ServerUpdateResponse
	response, err = controller.ServerUsecase.UpdateServerShortName(ctx, userId, serverId, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// UpdateServerCategory godoc
// @Summary      Update server category
// @Description.markdown update_server_category
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        id path string true "Server UUID"
// @Param        body body model.ServerUpdateCategoryRequest true "Payload"
// @Success      200   {object}  model.ServerUpdateResponse
// @Failure      400   {object}  model.ValidationError
// @Failure      403   {object}  model.ValidationError
// @Failure      500   {object}  model.ValidationError
// @Router       /servers/{id}/category [put]
func (controller *ServerController) UpdateServerCategory(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.UpdateServerCategory")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("id")

	var payload model.ServerUpdateCategoryRequest
	err = util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	var response model.ServerUpdateResponse
	response, err = controller.ServerUsecase.UpdateServerCategory(ctx, userId, serverId, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// UpdateServerDescription godoc
// @Summary      Update server description
// @Description.markdown update_server_description
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        id path string true "Server UUID"
// @Param        body body model.ServerUpdateDescriptionRequest true "Payload"
// @Success      200   {object}  model.ServerUpdateResponse
// @Failure      400   {object}  model.ValidationError
// @Failure      403   {object}  model.ValidationError
// @Failure      500   {object}  model.ValidationError
// @Router       /servers/{id}/description [put]
func (controller *ServerController) UpdateServerDescription(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.UpdateServerDescription")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("id")

	var payload model.ServerUpdateDescriptionRequest
	err = util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	var response model.ServerUpdateResponse
	response, err = controller.ServerUsecase.UpdateServerDescription(ctx, userId, serverId, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// UpdateServerSettings godoc
// @Summary      Update server settings
// @Description.markdown update_server_settings
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        id path string true "Server UUID"
// @Param        body body model.ServerUpdateSettingsRequest true "Payload"
// @Success      200   {object}  model.ServerUpdateResponse
// @Failure      400   {object}  model.ValidationError
// @Failure      403   {object}  model.ValidationError
// @Failure      500   {object}  model.ValidationError
// @Router       /servers/{id}/settings [put]
func (controller *ServerController) UpdateServerSettings(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.UpdateServerSettings")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("id")

	var payload model.ServerUpdateSettingsRequest
	err = util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	var response model.ServerUpdateResponse
	response, err = controller.ServerUsecase.UpdateServerSettings(ctx, userId, serverId, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// UpdateServerAvatar godoc
// @Summary      Update server avatar
// @Description.markdown update_server_avatar
// @Tags         servers
// @Accept       multipart/form-data
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        id path string true "Server UUID"
// @Param        avatar formData file true "Avatar image file"
// @Success      200
// @Failure      400   {object}  model.ValidationError
// @Failure      403   {object}  model.ValidationError
// @Failure      500   {object}  model.ValidationError
// @Router       /servers/{id}/avatar [put]
func (controller *ServerController) UpdateServerAvatar(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.UpdateServerAvatar")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	err = util.ReadMultipartBody(ctx)
	if err != nil {
		return util.SendError(ctx, err)
	}

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("id")

	err = controller.ServerUsecase.UpdateServerAvatar(ctx, userId, serverId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// UpdateServerBanner godoc
// @Summary      Update server banner
// @Description.markdown update_server_banner
// @Tags         servers
// @Accept       multipart/form-data
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        id path string true "Server UUID"
// @Param        banner formData file true "Banner image file"
// @Success      200
// @Failure      400   {object}  model.ValidationError
// @Failure      403   {object}  model.ValidationError
// @Failure      500   {object}  model.ValidationError
// @Router       /servers/{id}/banner [put]
func (controller *ServerController) UpdateServerBanner(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.UpdateServerBanner")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	err = util.ReadMultipartBody(ctx)
	if err != nil {
		return util.SendError(ctx, err)
	}

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("id")

	err = controller.ServerUsecase.UpdateServerBanner(ctx, userId, serverId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// DeleteServer godoc
// @Summary      Delete a server
// @Description.markdown delete_server
// @Tags         servers
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        id path string true "Server UUID"
// @Success      200
// @Failure      400   {object}  model.ValidationError
// @Failure      403   {object}  model.ValidationError
// @Failure      500   {object}  model.ValidationError
// @Router       /servers/{id} [delete]
func (controller *ServerController) DeleteServer(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.DeleteServer")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)
	serverId := ctx.Params("id")

	err = controller.ServerUsecase.DeleteServer(ctx, userId, serverId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// LeaveServer godoc
// @Summary      Leave a server
// @Description.markdown leave_server
// @Tags         servers
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        serverId path string true "Server UUID"
// @Success      200
// @Failure      400   {object}  model.ValidationError
// @Failure      404   {object}  model.ValidationError
// @Failure      409   {object}  model.ValidationError
// @Failure      500   {object}  model.ValidationError
// @Router       /servers/{serverId}/membership [delete]
func (controller *ServerController) LeaveServer(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.LeaveServer")
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

	err = controller.ServerUsecase.LeaveServer(ctx, userId, serverId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// CreateInviteLink godoc
// @Summary      Create server invite link
// @Description.markdown create_invite_link
// @Tags         server-invites
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        serverId path string true "Server UUID"
// @Param        body body model.ServerInviteLinkRequest true "Payload"
// @Success      200   {object}  model.ServerInviteLinkResponse
// @Failure      400   {object}  model.ValidationError
// @Failure      403   {object}  model.ValidationError
// @Failure      500   {object}  model.ValidationError
// @Router       /servers/{serverId}/invites [post]
func (controller *ServerController) CreateInviteLink(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.CreateInviteLink")
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

	var payload model.ServerInviteLinkRequest
	err = util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	var response model.ServerInviteLinkResponse
	response, err = controller.ServerUsecase.CreateInviteLink(ctx, userId, serverId, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

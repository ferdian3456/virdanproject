package http

import (
	"errors"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/usecase"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/knadh/koanf/v2"
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

// func (controller *ServerController) GetUserServer(ctx fiber.Ctx) error {
// 	userId := ctx.Locals("userId").(uuid.UUID)

// 	var validationErr *model.ValidationError

// 	response, err := controller.ServerUsecase.GetUserServer(ctx, userId)
// 	if err != nil {
// 		if errors.As(err, &validationErr) {
// 			return util.SendErrorResponseNotFound(ctx, err)
// 		}

// 		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
// 	}

// 	return util.SendSuccessResponseWithData(ctx, response)
// }

// func (controller *ServerController) JoinServer(ctx fiber.Ctx) error {
// 	userId := ctx.Locals("userId").(uuid.UUID)

// 	var validationErr *model.ValidationError

// 	response, err := controller.ServerUsecase.JoinServer(ctx, userId)
// 	if err != nil {
// 		if errors.As(err, &validationErr) {
// 			return util.SendErrorResponseNotFound(ctx, err)
// 		}

// 		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
// 	}

// 	return util.SendSuccessResponseWithData(ctx, response)
// }

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
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/{serverId}/invites [post]
func (controller *ServerController) CreateInviteLink(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	var payload model.ServerInviteLinkRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.RecordAndSendValidationError(ctx, controller.Log, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		}, "ServerController.CreateInviteLink")
	}

	var validationErr *model.ValidationError

	response, err := controller.ServerUsecase.CreateInviteLink(ctx, userId, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "ServerController.CreateInviteLink")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// JoinServerFromInvite godoc
// @Summary      Join server using invite code
// @Description.markdown join_server_from_invite
// @Tags         server-invites
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        body body model.ServerJoinRequest true "Payload"
// @Success      200
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/join [post]
func (controller *ServerController) JoinServerFromInvite(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	var payload model.ServerJoinRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.RecordAndSendValidationError(ctx, controller.Log, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		}, "ServerController.JoinServerFromInvite")
	}

	var validationErr *model.ValidationError

	err = controller.ServerUsecase.JoinServerFromInvite(ctx, userId, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "ServerController.JoinServerFromInvite")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
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
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/invites/{inviteCode} [get]
func (controller *ServerController) GetServerInfoForInvite(ctx fiber.Ctx) error {
	var validationErr *model.ValidationError

	inviteCode := ctx.Params("inviteCode")

	response, err := controller.ServerUsecase.GetServerInfoForInvite(ctx, inviteCode)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "ServerController.GetServerInfoForInvite")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// CreateServer godoc
// @Summary      Create a new server
// @Description.markdown create_server
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        body body model.ServerCreateRequest true "Payload"
// @Success      200   {object}  model.ServerCreateResponse
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/create [post]
func (controller *ServerController) CreateServer(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	var payload model.ServerCreateRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.RecordAndSendValidationError(ctx, controller.Log, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		}, "ServerController.CreateServer")
	}

	var validationErr *model.ValidationError

	response, err := controller.ServerUsecase.CreateServer(ctx, userId, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "ServerController.CreateServer")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
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
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/ [get]
func (controller *ServerController) GetDiscoveryServer(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	var validationErr *model.ValidationError

	response, err := controller.ServerUsecase.GetDiscoveryServer(ctx, userId)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "ServerController.GetDiscoveryServer")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
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
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/me [get]
func (controller *ServerController) GetUserServer(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	var validationErr *model.ValidationError

	response, err := controller.ServerUsecase.GetUserServer(ctx, userId)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "ServerController.GetUserServer")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// JoinServer godoc
// @Summary      Join a public server
// @Description.markdown join_server
// @Tags         servers
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        serverId path string true "Server UUID"
// @Success      200
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/{serverId}/join [post]
func (controller *ServerController) JoinServer(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	var validationErr *model.ValidationError

	err := controller.ServerUsecase.JoinServer(ctx, userId)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "ServerController.JoinServer")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// func (controller *ServerController) GetServerById(ctx fiber.Ctx) error {
// 	serverId := ctx.Params("id")

// 	var validationErr *model.ValidationError

// 	response, err := controller.ServerUsecase.GetServerById(ctx, serverId)
// 	if err != nil {
// 		if errors.As(err, &validationErr) {
// 			return util.SendErrorResponseNotFound(ctx, err)
// 		}

// 		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
// 	}

// 	return util.SendSuccessResponseWithData(ctx, response)
// }

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
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/{id}/name [put]
func (controller *ServerController) UpdateServerName(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)
	serverIdParam := ctx.Params("id")

	var payload model.ServerUpdateNameRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.RecordAndSendValidationError(ctx, controller.Log, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		}, "ServerController.UpdateServerName")
	}

	var validationErr *model.ValidationError

	response, err := controller.ServerUsecase.UpdateServerName(ctx, userId, serverIdParam, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "ServerController.UpdateServerName")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
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
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/{id}/shortName [put]
func (controller *ServerController) UpdateServerShortName(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)
	serverIdParam := ctx.Params("id")

	var payload model.ServerUpdateShortNameRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.RecordAndSendValidationError(ctx, controller.Log, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		}, "ServerController.UpdateServerShortName")
	}

	var validationErr *model.ValidationError

	response, err := controller.ServerUsecase.UpdateServerShortName(ctx, userId, serverIdParam, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "ServerController.UpdateServerShortName")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
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
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/{id}/category [put]
func (controller *ServerController) UpdateServerCategory(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)
	serverIdParam := ctx.Params("id")

	var payload model.ServerUpdateCategoryRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.RecordAndSendValidationError(ctx, controller.Log, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		}, "ServerController.UpdateServerCategory")
	}

	var validationErr *model.ValidationError

	response, err := controller.ServerUsecase.UpdateServerCategory(ctx, userId, serverIdParam, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "ServerController.UpdateServerCategory")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
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
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/{id}/description [put]
func (controller *ServerController) UpdateServerDescription(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)
	serverIdParam := ctx.Params("id")

	var payload model.ServerUpdateDescriptionRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.RecordAndSendValidationError(ctx, controller.Log, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		}, "ServerController.UpdateServerDescription")
	}

	var validationErr *model.ValidationError

	response, err := controller.ServerUsecase.UpdateServerDescription(ctx, userId, serverIdParam, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "ServerController.UpdateServerDescription")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
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
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/{id} [delete]
func (controller *ServerController) DeleteServer(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)
	serverIdParam := ctx.Params("id")

	var validationErr *model.ValidationError

	err := controller.ServerUsecase.DeleteServer(ctx, userId, serverIdParam)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "ServerController.DeleteServer")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
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
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/{id}/avatar [put]
func (controller *ServerController) UpdateServerAvatar(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)
	serverIdParam := ctx.Params("id")

	var validationErr *model.ValidationError

	err := controller.ServerUsecase.UpdateServerAvatar(ctx, userId, serverIdParam)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "ServerController.UpdateServerAvatar")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
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
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/{id}/banner [put]
func (controller *ServerController) UpdateServerBanner(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)
	serverIdParam := ctx.Params("id")

	var validationErr *model.ValidationError

	err := controller.ServerUsecase.UpdateServerBanner(ctx, userId, serverIdParam)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "ServerController.UpdateServerBanner")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// UpdateServerSettings godoc
// @Summary      Update server settings
// @Description.markdown update_server_settings
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        id path string true "Server UUID"
// @Param        body body model.ServerSettingsCreateRequest true "Payload"
// @Success      200
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/{id}/settings [put]
func (controller *ServerController) UpdateServerSettings(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)
	serverIdParam := ctx.Params("id")

	var payload model.ServerSettingsCreateRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.RecordAndSendValidationError(ctx, controller.Log, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		}, "ServerController.UpdateServerSettings")
	}

	var validationErr *model.ValidationError

	err = controller.ServerUsecase.UpdateServerSettings(ctx, userId, serverIdParam, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "ServerController.UpdateServerSettings")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// GetCategoryServer godoc
// @Summary      Get server categories
// @Description.markdown get_category_server
// @Tags         servers
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        limit query int false "Items per page"
// @Param        cursor query string false "Pagination cursor"
// @Success      200   {object}  model.ServerCategoryListResponse
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/categories [get]
func (controller *ServerController) GetCategoryServer(ctx fiber.Ctx) error {
	var validationErr *model.ValidationError

	response, err := controller.ServerUsecase.GetCategoryServer(ctx)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "ServerController.GetCategoryServer")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
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
// @Failure 	 500   {object}  model.ValidationError
// @Router       /servers/{id} [get]
func (controller *ServerController) GetServerById(ctx fiber.Ctx) error {
	var validationErr *model.ValidationError

	response, err := controller.ServerUsecase.GetServerById(ctx)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "ServerController.GetServerById")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

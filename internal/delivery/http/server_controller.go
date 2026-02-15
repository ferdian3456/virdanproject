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

package http

import (
	"errors"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/usecase"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/gofiber/fiber/v2"
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

// func (controller *ServerController) GetUserServer(ctx *fiber.Ctx) error {
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

// func (controller *ServerController) JoinServer(ctx *fiber.Ctx) error {
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

func (controller *ServerController) CreateInviteLink(ctx *fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	var payload model.ServerInviteLinkRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendErrorResponse(ctx, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		})
	}

	var validationErr *model.ValidationError

	response, err := controller.ServerUsecase.CreateInviteLink(ctx, userId, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponseNotFound(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

func (controller *ServerController) JoinServerFromInvite(ctx *fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	var payload model.ServerJoinRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendErrorResponse(ctx, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		})
	}

	var validationErr *model.ValidationError

	err = controller.ServerUsecase.JoinServerFromInvite(ctx, userId, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponseNotFound(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

func (controller *ServerController) GetServerInfoForInvite(ctx *fiber.Ctx) error {
	var validationErr *model.ValidationError

	inviteCode := ctx.Params("inviteCode")

	response, err := controller.ServerUsecase.GetServerInfoForInvite(ctx, inviteCode)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponseNotFound(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

func (controller *ServerController) CreateServer(ctx *fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	var payload model.ServerCreateRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendErrorResponse(ctx, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		})
	}

	var validationErr *model.ValidationError

	err = controller.ServerUsecase.CreateServer(ctx, userId, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponseNotFound(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

func (controller *ServerController) GetDiscoveryServer(ctx *fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	var validationErr *model.ValidationError

	response, err := controller.ServerUsecase.GetDiscoveryServer(ctx, userId)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponseNotFound(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

func (controller *ServerController) GetUserServer(ctx *fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	var validationErr *model.ValidationError

	response, err := controller.ServerUsecase.GetUserServer(ctx, userId)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponseNotFound(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

func (controller *ServerController) JoinServer(ctx *fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	var validationErr *model.ValidationError

	err := controller.ServerUsecase.JoinServer(ctx, userId)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponseNotFound(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// func (controller *ServerController) GetServerById(ctx *fiber.Ctx) error {
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

func (controller *ServerController) UpdateServerName(ctx *fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)
	serverIdParam := ctx.Params("id")

	var payload model.ServerUpdateNameRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendErrorResponse(ctx, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		})
	}

	var validationErr *model.ValidationError

	err = controller.ServerUsecase.UpdateServerName(ctx, userId, serverIdParam, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponseNotFound(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

func (controller *ServerController) UpdateServerShortName(ctx *fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)
	serverIdParam := ctx.Params("id")

	var payload model.ServerUpdateShortNameRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendErrorResponse(ctx, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		})
	}

	var validationErr *model.ValidationError

	err = controller.ServerUsecase.UpdateServerShortName(ctx, userId, serverIdParam, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponseNotFound(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

func (controller *ServerController) UpdateServerCategory(ctx *fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)
	serverIdParam := ctx.Params("id")

	var payload model.ServerUpdateCategoryRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendErrorResponse(ctx, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		})
	}

	var validationErr *model.ValidationError

	err = controller.ServerUsecase.UpdateServerCategory(ctx, userId, serverIdParam, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponseNotFound(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

func (controller *ServerController) UpdateServerDescription(ctx *fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)
	serverIdParam := ctx.Params("id")

	var payload model.ServerUpdateDescriptionRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendErrorResponse(ctx, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		})
	}

	var validationErr *model.ValidationError

	err = controller.ServerUsecase.UpdateServerDescription(ctx, userId, serverIdParam, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponseNotFound(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

func (controller *ServerController) DeleteServer(ctx *fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)
	serverIdParam := ctx.Params("id")

	var validationErr *model.ValidationError

	err := controller.ServerUsecase.DeleteServer(ctx, userId, serverIdParam)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponseNotFound(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

func (controller *ServerController) UpdateServerAvatar(ctx *fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)
	serverIdParam := ctx.Params("id")

	var validationErr *model.ValidationError

	err := controller.ServerUsecase.UpdateServerAvatar(ctx, userId, serverIdParam)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponseNotFound(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

func (controller *ServerController) UpdateServerBanner(ctx *fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)
	serverIdParam := ctx.Params("id")

	var validationErr *model.ValidationError

	err := controller.ServerUsecase.UpdateServerBanner(ctx, userId, serverIdParam)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponseNotFound(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

func (controller *ServerController) UpdateServerSettings(ctx *fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)
	serverIdParam := ctx.Params("id")

	var payload model.ServerSettingsCreateRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendErrorResponse(ctx, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		})
	}

	var validationErr *model.ValidationError

	err = controller.ServerUsecase.UpdateServerSettings(ctx, userId, serverIdParam, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponseNotFound(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

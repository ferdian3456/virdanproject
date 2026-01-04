package http

import (
	"errors"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/usecase"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/google/uuid"

	"github.com/gofiber/fiber/v2"
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
)

type UserController struct {
	UserUsecase *usecase.UserUsecase
	Log         *zap.Logger
	Config      *koanf.Koanf
}

func NewUserController(userUsecase *usecase.UserUsecase, zap *zap.Logger, koanf *koanf.Koanf) *UserController {
	return &UserController{
		UserUsecase: userUsecase,
		Log:         zap,
		Config:      koanf,
	}
}

// func (controller UserController) Register(ctx *fiber.Ctx) error {
// 	var payload model.UserCreateRequest
// 	err := util.ReadRequestBody(ctx, &payload)
// 	if err != nil {
// 		return util.SendErrorResponse(ctx, &model.ValidationError{
// 			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
// 			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
// 		})
// 	}

// 	var validationErr *model.ValidationError

// 	response, err := controller.UserUsecase.Register(ctx, payload)
// 	if err != nil {
// 		if errors.As(err, &validationErr) {
// 			return util.SendErrorResponse(ctx, err)
// 		}

// 		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
// 	}

// 	return util.SendSuccessResponseWithData(ctx, response)
// }

func (controller UserController) Login(ctx *fiber.Ctx) error {
	var payload model.UserLoginRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendErrorResponse(ctx, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		})
	}

	var validationErr *model.ValidationError

	response, err := controller.UserUsecase.Login(ctx, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponse(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

func (controller UserController) GetUserInfo(ctx *fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	var validationErr *model.ValidationError

	response, err := controller.UserUsecase.GetUserInfo(ctx, userId)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponseNotFound(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

func (controller UserController) Logout(ctx *fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	err := controller.UserUsecase.Logout(ctx, userId)
	if err != nil {
		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

func (controller UserController) UpdateAvatar(ctx *fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	var validationErr *model.ValidationError

	err := controller.UserUsecase.UpdateAvatar(ctx, userId)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponseNotFound(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

func (controller UserController) StartSignup(ctx *fiber.Ctx) error {
	var payload model.UserSignupStartRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendErrorResponse(ctx, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		})
	}
	var validationErr *model.ValidationError

	response, err := controller.UserUsecase.StartSignup(ctx, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponse(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

func (controller UserController) VerifyOtp(ctx *fiber.Ctx) error {
	var payload model.UserVerifyOTPRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendErrorResponse(ctx, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		})
	}
	var validationErr *model.ValidationError

	err = controller.UserUsecase.VerifyOtp(ctx, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponse(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

func (controller UserController) VerifyUsername(ctx *fiber.Ctx) error {
	var payload model.UserVerifyUsernameRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendErrorResponse(ctx, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		})
	}
	var validationErr *model.ValidationError

	err = controller.UserUsecase.VerifyUsername(ctx, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponse(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

func (controller UserController) GetSignupStatus(ctx *fiber.Ctx) error {
	sessionId := ctx.Params("sessionId")

	var validationErr *model.ValidationError

	response, err := controller.UserUsecase.GetSignupStatus(ctx, sessionId)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponse(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

func (controller UserController) VerifyPassword(ctx *fiber.Ctx) error {
	var payload model.UserVerifyPasswordRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendErrorResponse(ctx, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		})
	}
	var validationErr *model.ValidationError

	response, err := controller.UserUsecase.VerifyPassword(ctx, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.SendErrorResponse(ctx, err)
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

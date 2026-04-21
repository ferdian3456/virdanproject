package http

import (
	"errors"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/usecase"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/google/uuid"

	"github.com/gofiber/fiber/v3"
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

// func (controller UserController) Register(ctx fiber.Ctx) error {
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

// Login godoc
// @Summary      Login user
// @Description.markdown login
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param 		 body body model.UserLoginRequest true "Payload"
// @Success      200   {object}  model.TokenResponse
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /auth/login [post]
func (controller UserController) Login(ctx fiber.Ctx) error {
	var payload model.UserLoginRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	response, err := controller.UserUsecase.Login(ctx, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// GetUserInfo godoc
// @Summary      Get current user info
// @Description.markdown get_user_info
// @Tags         users
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Success      200   {object}  model.UserResponse
// @Failure      400   {object}  model.ValidationError
// @Failure      404   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /users/me [get]
func (controller UserController) GetUserInfo(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	var validationErr *model.ValidationError

	response, err := controller.UserUsecase.GetUserInfo(ctx, userId)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationErrorNotFound(ctx, controller.Log, validationErr, "UserController.GetUserInfo")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// Logout godoc
// @Summary      Logout user
// @Description.markdown logout
// @Tags         users
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Success      200
// @Failure      404   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /users/logout [post]
func (controller UserController) Logout(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	err := controller.UserUsecase.Logout(ctx, userId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// UpdateAvatar godoc
// @Summary      Update user avatar
// @Description.markdown update_avatar
// @Tags         users
// @Accept       multipart/form-data
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        avatar formData file true "Avatar image file"
// @Success      200
// @Failure      400   {object}  model.ValidationError
// @Failure      404   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /users/avatar [put]
func (controller UserController) UpdateAvatar(ctx fiber.Ctx) error {
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

// StartSignup godoc
// @Summary      Start signup process
// @Description.markdown start_signup
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param 		 body body model.UserSignupStartRequest true "Payload"
// @Success      200   {object}  model.UserSignupStartResponse
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /auth/signup/start [post]
func (controller UserController) StartSignup(ctx fiber.Ctx) error {
	var payload model.UserSignupStartRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	response, err := controller.UserUsecase.StartSignup(ctx, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// VerifyOtp godoc
// @Summary      Verify OTP code
// @Description.markdown verify_otp
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param 		 body body model.UserVerifyOTPRequest true "Payload"
// @Success      200
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /auth/signup/otp [post]
func (controller UserController) VerifyOtp(ctx fiber.Ctx) error {
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
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "UserController.VerifyOtp")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// ResendOtp godoc
// @Summary      Resend OTP code
// @Description.markdown resend_otp
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param 		 body body model.UserResendOTPRequest true "Payload"
// @Success      200   {object}  model.UserSignupStartResponse
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /auth/signup/resend-otp [post]
func (controller UserController) ResendOtp(ctx fiber.Ctx) error {
	var payload model.UserResendOTPRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendErrorResponse(ctx, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		})
	}
	var validationErr *model.ValidationError

	response, err := controller.UserUsecase.ResendOtp(ctx, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "UserController.ResendOtp")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// VerifyUsername godoc
// @Summary      Verify and set username
// @Description.markdown verify_username
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param 		 body body model.UserVerifyUsernameRequest true "Payload"
// @Success      200
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /auth/signup/username [post]
func (controller UserController) VerifyUsername(ctx fiber.Ctx) error {
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
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "UserController.VerifyUsername")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// GetSignupStatus godoc
// @Summary      Get signup session status
// @Description.markdown get_signup_status
// @Tags         auth
// @Produce      json
// @Param        sessionId path string true "Signup session UUID"
// @Success      200   {object}  model.UserSignupStatus
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /auth/signup/{sessionId}/status [get]
func (controller UserController) GetSignupStatus(ctx fiber.Ctx) error {
	sessionId := ctx.Params("sessionId")

	var validationErr *model.ValidationError

	response, err := controller.UserUsecase.GetSignupStatus(ctx, sessionId)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "UserController.GetSignupStatus")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// VerifyPassword godoc
// @Summary      Verify password and complete signup
// @Description.markdown verify_password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param 		 body body model.UserVerifyPasswordRequest true "Payload"
// @Success      200   {object}  model.TokenResponse
// @Failure      400   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /auth/signup/password [post]
func (controller UserController) VerifyPassword(ctx fiber.Ctx) error {
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
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "UserController.VerifyPassword")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

func (controller UserController) UpdateUsername(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	var payload model.UsernameUpdateRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendErrorResponse(ctx, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		})
	}

	var validationErr *model.ValidationError

	err = controller.UserUsecase.UpdateUsername(ctx, userId, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationErrorNotFound(ctx, controller.Log, validationErr, "UserController.UpdateUsername")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// UpdateFullname godoc
// @Summary      Update user fullname
// @Description.markdown update_fullname
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param 		 body body model.FullnameUpdateRequest true "Payload"
// @Success      200
// @Failure      400   {object}  model.ValidationError
// @Failure      404   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /users/fullname [put]
func (controller UserController) UpdateFullname(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	var payload model.FullnameUpdateRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendErrorResponse(ctx, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		})
	}

	var validationErr *model.ValidationError

	err = controller.UserUsecase.UpdateFullname(ctx, userId, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationErrorNotFound(ctx, controller.Log, validationErr, "UserController.UpdateFullname")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// UpdateBio godoc
// @Summary      Update user bio
// @Description.markdown update_bio
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param 		 body body model.BioUpdateRequest true "Payload"
// @Success      200
// @Failure      400   {object}  model.ValidationError
// @Failure      404   {object}  model.ValidationError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /users/bio [put]
func (controller UserController) UpdateBio(ctx fiber.Ctx) error {
	userId := ctx.Locals("userId").(uuid.UUID)

	var payload model.BioUpdateRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendErrorResponse(ctx, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		})
	}

	var validationErr *model.ValidationError

	err = controller.UserUsecase.UpdateBio(ctx, userId, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationErrorNotFound(ctx, controller.Log, validationErr, "UserController.UpdateBio")
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// RefreshToken godoc
// @Summary      Refresh access token
// @Description  Use refresh token to get new access token (with token rotation)
// @Description.markdown refresh_token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body model.RefreshTokenRefreshRequest true "Refresh token"
// @Success      200   {object}  model.TokenResponse
// @Failure      400   {object}  model.ValidationError
// @Failure      401   {object}  model.UnauthorizedError
// @Failure 	 500   {object}  model.ValidationError
// @Router       /auth/refresh [post]
func (controller UserController) RefreshToken(ctx fiber.Ctx) error {
	var payload model.RefreshTokenRefreshRequest
	err := util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendErrorResponse(ctx, &model.ValidationError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		})
	}

	var validationErr *model.ValidationError
	var unauthorizedErr *model.UnauthorizedError

	response, err := controller.UserUsecase.RefreshToken(ctx, payload)
	if err != nil {
		if errors.As(err, &validationErr) {
			return util.RecordAndSendValidationError(ctx, controller.Log, validationErr, "UserController.RefreshToken")
		}
		if errors.As(err, &unauthorizedErr) {
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": unauthorizedErr,
			})
		}

		return util.SendErrorResponseInternalServer(ctx, controller.Log, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

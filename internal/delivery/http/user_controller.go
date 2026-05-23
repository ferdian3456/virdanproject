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

// StartSignup godoc
// @Summary      Start signup process
// @Description.markdown start_signup
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body model.UserSignupStartRequest true "Payload"
// @Success      200   {object}  model.UserSignupStartResponse
// @Failure      400   {object}  model.BadRequestError
// @Failure      409   {object}  model.ConflictError
// @Failure      500   {object}  model.BadRequestError
// @Router       /auth/signup/start [post]
func (controller UserController) StartSignup(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.StartSignup")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var payload model.UserSignupStartRequest
	err = util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	var response model.UserSignupStartResponse
	response, err = controller.UserUsecase.StartSignup(ctx, payload)
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
// @Param        body body model.UserVerifyOTPRequest true "Payload"
// @Success      200
// @Failure      400   {object}  model.BadRequestError
// @Failure      500   {object}  model.BadRequestError
// @Router       /auth/signup/otp [post]
func (controller UserController) VerifyOtp(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.VerifyOtp")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var payload model.UserVerifyOTPRequest
	err = util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	err = controller.UserUsecase.VerifyOtp(ctx, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// ResendOtp godoc
// @Summary      Resend OTP code
// @Description.markdown resend_otp
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body model.UserResendOTPRequest true "Payload"
// @Success      200   {object}  model.UserSignupStartResponse
// @Failure      400   {object}  model.BadRequestError
// @Failure      500   {object}  model.BadRequestError
// @Router       /auth/signup/resend-otp [post]
func (controller UserController) ResendOtp(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.ResendOtp")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var payload model.UserResendOTPRequest
	err = util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	var response model.UserSignupStartResponse
	response, err = controller.UserUsecase.ResendOtp(ctx, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// VerifyPassword godoc
// @Summary      Verify password and complete signup
// @Description.markdown verify_password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body model.UserVerifyPasswordRequest true "Payload"
// @Success      200   {object}  model.TokenResponse
// @Failure      400   {object}  model.BadRequestError
// @Failure      409   {object}  model.ConflictError
// @Failure      500   {object}  model.BadRequestError
// @Router       /auth/signup/password [post]
func (controller UserController) VerifyPassword(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.VerifyPassword")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var payload model.UserVerifyPasswordRequest
	err = util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	var response model.TokenResponse
	response, err = controller.UserUsecase.VerifyPassword(ctx, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// GetSignupStatus godoc
// @Summary      Get signup session status
// @Description.markdown get_signup_status
// @Tags         auth
// @Produce      json
// @Param        sessionId path string true "Signup session UUID"
// @Success      200   {object}  model.UserSignupStatus
// @Failure      400   {object}  model.BadRequestError
// @Failure      404   {object}  model.NotFoundError
// @Failure      500   {object}  model.BadRequestError
// @Router       /auth/signup/{sessionId}/status [get]
func (controller UserController) GetSignupStatus(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetSignupStatus")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	sessionId := ctx.Params("sessionId")

	var response model.UserSignupStatus
	response, err = controller.UserUsecase.GetSignupStatus(ctx, sessionId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// Login godoc
// @Summary      Login user
// @Description.markdown login
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body model.UserLoginRequest true "Payload"
// @Success      200   {object}  model.TokenResponse
// @Failure      400   {object}  model.BadRequestError
// @Failure      500   {object}  model.BadRequestError
// @Router       /auth/login [post]
func (controller UserController) Login(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.Login")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var payload model.UserLoginRequest
	err = util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	var response model.TokenResponse
	response, err = controller.UserUsecase.Login(ctx, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

// RefreshToken godoc
// @Summary      Refresh access token
// @Description.markdown refresh_token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body model.RefreshTokenRefreshRequest true "Refresh token"
// @Success      200   {object}  model.TokenResponse
// @Failure      400   {object}  model.BadRequestError
// @Failure      401   {object}  model.UnauthorizedError
// @Failure      500   {object}  model.BadRequestError
// @Router       /auth/refresh [post]
func (controller UserController) RefreshToken(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.RefreshToken")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var payload model.RefreshTokenRefreshRequest
	err = util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	var response model.TokenResponse
	response, err = controller.UserUsecase.RefreshToken(ctx, payload)
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
// @Failure      401   {object}  model.UnauthorizedError
// @Failure      404   {object}  model.NotFoundError
// @Failure      500   {object}  model.BadRequestError
// @Router       /users/me [get]
func (controller UserController) GetUserInfo(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetUserInfo")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	var response model.UserResponse
	response, err = controller.UserUsecase.GetUserInfo(ctx, userId)
	if err != nil {
		return util.SendError(ctx, err)
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
// @Failure      401   {object}  model.UnauthorizedError
// @Failure      500   {object}  model.BadRequestError
// @Router       /users/logout [post]
func (controller UserController) Logout(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.Logout")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	err = controller.UserUsecase.Logout(ctx, userId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// DeleteAccount godoc
// @Summary      Delete current user account
// @Description.markdown delete_account
// @Tags         users
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Success      200
// @Failure      401   {object}  model.UnauthorizedError
// @Failure      404   {object}  model.NotFoundError
// @Failure      500   {object}  model.BadRequestError
// @Router       /users/me [delete]
func (controller UserController) DeleteAccount(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.DeleteAccount")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	err = controller.UserUsecase.DeleteAccount(ctx, userId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseNoData(ctx)
}

// VerifyCurrentPassword godoc
// @Summary      Verify current password (change-password step 1)
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        body body model.UserVerifyCurrentPasswordRequest true "Payload"
// @Success      200
// @Failure      400 {object} model.BadRequestError
// @Failure      401 {object} model.UnauthorizedError
// @Router       /users/password/verify [post]
func (controller UserController) VerifyCurrentPassword(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.VerifyCurrentPassword")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	var payload model.UserVerifyCurrentPasswordRequest
	err = util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	err = controller.UserUsecase.VerifyCurrentPassword(ctx, userId, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}
	return util.SendSuccessResponseNoData(ctx)
}

// ChangePassword godoc
// @Summary      Change password
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        body body model.UserChangePasswordRequest true "Payload"
// @Success      200
// @Failure      400 {object} model.BadRequestError
// @Failure      401 {object} model.UnauthorizedError
// @Router       /users/password [put]
func (controller UserController) ChangePassword(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.ChangePassword")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	var payload model.UserChangePasswordRequest
	err = util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	err = controller.UserUsecase.ChangePassword(ctx, userId, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}
	return util.SendSuccessResponseNoData(ctx)
}

// RequestEmailChange godoc
// @Summary      Request email change — sends OTP to current email
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        body body model.UserChangeEmailRequestRequest true "Payload"
// @Success      200 {object} model.UserChangeEmailRequestResponse
// @Failure      400 {object} model.BadRequestError
// @Failure      409 {object} model.ConflictError
// @Router       /users/email/change/request [post]
func (controller UserController) RequestEmailChange(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.RequestEmailChange")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	var payload model.UserChangeEmailRequestRequest
	err = util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	var response model.UserChangeEmailRequestResponse
	response, err = controller.UserUsecase.RequestEmailChange(ctx, userId, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}
	return util.SendSuccessResponseWithData(ctx, response)
}

// ConfirmEmailChange godoc
// @Summary      Confirm email change via OTP
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        body body model.UserChangeEmailConfirmRequest true "Payload"
// @Success      200
// @Failure      400 {object} model.BadRequestError
// @Failure      409 {object} model.ConflictError
// @Router       /users/email/change/confirm [post]
func (controller UserController) ConfirmEmailChange(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.ConfirmEmailChange")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	var payload model.UserChangeEmailConfirmRequest
	err = util.ReadRequestBody(ctx, &payload)
	if err != nil {
		return util.SendError(ctx, err)
	}

	err = controller.UserUsecase.ConfirmEmailChange(ctx, userId, payload)
	if err != nil {
		return util.SendError(ctx, err)
	}
	return util.SendSuccessResponseNoData(ctx)
}

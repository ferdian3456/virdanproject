package auth

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

// StartSignup godoc
// @Summary Start user signup by sending an OTP to the given email
// @description.markdown start_signup
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.UserSignupStartRequest true "Signup email"
// @Success 200 {object} auth.UserSignupStartResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 409 {object} shared.ConflictError
// @Router /auth/signup/start [post]
func (controller Controller) StartSignup(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.StartSignup")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var payload UserSignupStartRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	var response UserSignupStartResponse
	response, err = controller.Service.StartSignup(ctx, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// VerifyOtp godoc
// @Summary Verify signup OTP code
// @description.markdown verify_otp
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.UserVerifyOTPRequest true "OTP verification payload"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Router /auth/signup/otp [post]
func (controller Controller) VerifyOtp(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.VerifyOtp")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var payload UserVerifyOTPRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	err = controller.Service.VerifyOtp(ctx, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseNoData(ctx)
}

// ResendOtp godoc
// @Summary Resend signup OTP code
// @description.markdown resend_otp
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.UserResendOTPRequest true "Resend OTP payload"
// @Success 200 {object} auth.UserSignupStartResponse
// @Failure 400 {object} shared.BadRequestError
// @Router /auth/signup/resend-otp [post]
func (controller Controller) ResendOtp(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.ResendOtp")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var payload UserResendOTPRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	var response UserSignupStartResponse
	response, err = controller.Service.ResendOtp(ctx, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// VerifyPassword godoc
// @Summary Set password to complete signup
// @description.markdown set_password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.UserVerifyPasswordRequest true "Set password payload"
// @Success 200 {object} shared.TokenResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 409 {object} shared.ConflictError
// @Router /auth/signup/password [post]
func (controller Controller) VerifyPassword(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.VerifyPassword")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var payload UserVerifyPasswordRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	var response shared.TokenResponse
	response, err = controller.Service.VerifyPassword(ctx, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// GetSignupStatus godoc
// @Summary Get current signup session status
// @description.markdown get_signup_status
// @Tags auth
// @Produce json
// @Param sessionId path string true "Signup Session ID"
// @Success 200 {object} auth.UserSignupStatus
// @Failure 400 {object} shared.BadRequestError
// @Failure 404 {object} shared.NotFoundError
// @Router /auth/signup/{sessionId}/status [get]
func (controller Controller) GetSignupStatus(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetSignupStatus")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	sessionId := ctx.Params("sessionId")

	var response UserSignupStatus
	response, err = controller.Service.GetSignupStatus(ctx, sessionId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// Login godoc
// @Summary Login with email and password
// @description.markdown login
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.UserLoginRequest true "Login credentials"
// @Success 200 {object} shared.TokenResponse
// @Failure 400 {object} shared.BadRequestError
// @Router /auth/login [post]
func (controller Controller) Login(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.Login")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var payload UserLoginRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	var response shared.TokenResponse
	response, err = controller.Service.Login(ctx, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// RefreshToken godoc
// @Summary Refresh access token using a refresh token
// @description.markdown refresh_token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.RefreshTokenRefreshRequest true "Refresh token payload"
// @Success 200 {object} shared.TokenResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 404 {object} shared.NotFoundError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /auth/refresh [post]
func (controller Controller) RefreshToken(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.RefreshToken")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	var payload RefreshTokenRefreshRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	var response shared.TokenResponse
	response, err = controller.Service.RefreshToken(ctx, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// Logout godoc
// @Summary Logout current user and revoke tokens
// @description.markdown logout
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 401 {object} shared.UnauthorizedError
// @Router /auth/logout [post]
func (controller Controller) Logout(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.Logout")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	err = controller.Service.Logout(ctx, userId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseNoData(ctx)
}

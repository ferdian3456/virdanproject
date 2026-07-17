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

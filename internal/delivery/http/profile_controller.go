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

type ProfileController struct {
	ProfileUsecase *usecase.ProfileUsecase
	Log            *zap.Logger
	Config         *koanf.Koanf
}

func NewProfileController(profileUsecase *usecase.ProfileUsecase, log *zap.Logger, config *koanf.Koanf) *ProfileController {
	return &ProfileController{
		ProfileUsecase: profileUsecase,
		Log:            log,
		Config:         config,
	}
}

func (controller *ProfileController) GetProfileHistory(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.GetProfileHistory")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	var response model.GetProfileHistoryResponse
	response, err = controller.ProfileUsecase.GetProfileHistory(ctx, userId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

func (controller *ProfileController) GetServerProfileMe(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.GetServerProfileMe")
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

	var response model.ServerMemberProfileResponse
	response, err = controller.ProfileUsecase.GetServerProfileMe(ctx, userId, serverId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

func (controller *ProfileController) UpdateServerProfile(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.UpdateServerProfile")
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

	var response model.ServerProfileUpdateResponse
	response, err = controller.ProfileUsecase.UpdateServerProfile(ctx, userId, serverId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

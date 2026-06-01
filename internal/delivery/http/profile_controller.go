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

// GetProfileHistory godoc
// @Summary      Get my per-server profile history
// @Description  Returns the authenticated user's snapshot of past per-server profiles (nickname, username, bio, avatar) across every server they have joined or previously joined. Useful for the "pick a profile from another server" picker on YourProfilePage.
// @Tags         profiles
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Success      200 {object} model.GetProfileHistoryResponse
// @Failure      401 {object} model.UnauthorizedError
// @Failure      500 {object} model.BadRequestError
// @Router       /profiles/history [get]
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

// GetServerProfileMe godoc
// @Summary      Get my profile in a specific server
// @Description  Returns the authenticated user's per-server profile row for the given serverId: nickname, username, bio, avatar URL, plus metadata. Caller must be a member of the server.
// @Tags         profiles
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        serverId path string true "Server UUID"
// @Success      200 {object} model.ServerMemberProfileResponse
// @Failure      400 {object} model.BadRequestError
// @Failure      401 {object} model.UnauthorizedError
// @Failure      403 {object} model.ForbiddenError
// @Failure      404 {object} model.NotFoundError
// @Failure      500 {object} model.BadRequestError
// @Router       /servers/{serverId}/profile/me [get]
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

// UpdateServerProfile godoc
// @Summary      Update my per-server profile
// @Description  Multipart update for the authenticated user's per-server profile. Updates nickname, username, bio plus an optional profileAvatar file XOR avatarImageId (existing profile_avatar_images row). Username must match ^[a-zA-Z0-9_.]+$ and is unique per server. Runs in a single transaction.
// @Tags         profiles
// @Accept       multipart/form-data
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        serverId path string true "Server UUID"
// @Param        nickname formData string true "Display name (3-50 chars)"
// @Param        username formData string true "Per-server handle (3-22 chars, [a-zA-Z0-9_.])"
// @Param        bio formData string false "Optional bio (max 500 chars)"
// @Param        profileAvatar formData file false "New avatar image (JPEG/PNG/WebP). Mutually exclusive with avatarImageId."
// @Param        avatarImageId formData string false "Existing profile_avatar_images UUID to reuse. Mutually exclusive with profileAvatar."
// @Success      200 {object} model.ServerProfileUpdateResponse
// @Failure      400 {object} model.BadRequestError
// @Failure      401 {object} model.UnauthorizedError
// @Failure      403 {object} model.ForbiddenError
// @Failure      404 {object} model.NotFoundError
// @Failure      409 {object} model.ConflictError
// @Failure      500 {object} model.BadRequestError
// @Router       /servers/{serverId}/profile [put]
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

// GetServerProfileByUserId godoc
// @Summary      Get another member's profile in a server
// @Description.markdown get_server_member_profile
// @Tags         profiles
// @Produce      json
// @Param        Authorization header string true "Bearer access token"
// @Param        serverId path string true "Server UUID"
// @Param        userId path string true "Target user UUID"
// @Success      200 {object} model.ServerMemberProfileResponse
// @Failure      400 {object} model.BadRequestError
// @Failure      401 {object} model.UnauthorizedError
// @Failure      403 {object} model.ForbiddenError
// @Failure      404 {object} model.NotFoundError
// @Failure      500 {object} model.BadRequestError
// @Router       /servers/{serverId}/members/{userId}/profile [get]
func (controller *ProfileController) GetServerProfileByUserId(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName + "-controller").Start(ctxContext, "controller.GetServerProfileByUserId")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			util.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	requesterUserId := ctx.Locals("userId").(string)
	serverId := ctx.Params("serverId")
	targetUserId := ctx.Params("userId")

	var response model.ServerMemberProfileResponse
	response, err = controller.ProfileUsecase.GetServerMemberProfileByUserId(ctx, requesterUserId, serverId, targetUserId)
	if err != nil {
		return util.SendError(ctx, err)
	}

	return util.SendSuccessResponseWithData(ctx, response)
}

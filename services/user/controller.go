package user

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

// GetUserInfo godoc
// @Summary Get the logged-in user's account info
// @description.markdown get_user_info
// @Tags users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} user.UserResponse
// @Failure 401 {object} shared.UnauthorizedError
// @Router /users/me [get]
func (controller Controller) GetUserInfo(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.GetUserInfo")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	var response UserResponse
	response, err = controller.Service.GetUserInfo(ctx, userId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseWithData(ctx, response)
}

// DeleteAccount godoc
// @Summary Permanently delete the logged-in user's account
// @description.markdown delete_account
// @Tags users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 404 {object} shared.NotFoundError
// @Failure 409 {object} shared.ConflictError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /users/me [delete]
func (controller Controller) DeleteAccount(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.DeleteAccount")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	err = controller.Service.DeleteAccount(ctx, userId)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseNoData(ctx)
}

// VerifyCurrentPassword godoc
// @Summary Verify the logged-in user's current password
// @description.markdown verify_current_password
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body user.UserVerifyCurrentPasswordRequest true "Current password"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /users/password/verify [post]
func (controller Controller) VerifyCurrentPassword(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.VerifyCurrentPassword")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	var payload UserVerifyCurrentPasswordRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	err = controller.Service.VerifyCurrentPassword(ctx, userId, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}
	return shared.SendSuccessResponseNoData(ctx)
}

// ChangePassword godoc
// @Summary Change the logged-in user's password
// @description.markdown change_password
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body user.UserChangePasswordRequest true "Current + new password"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /users/password [put]
func (controller Controller) ChangePassword(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.ChangePassword")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	var payload UserChangePasswordRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	err = controller.Service.ChangePassword(ctx, userId, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}
	return shared.SendSuccessResponseNoData(ctx)
}

// RequestEmailChange godoc
// @Summary Request an email change and send OTP to the new address
// @description.markdown request_email_change
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body user.UserChangeEmailRequestRequest true "New email"
// @Success 200 {object} user.UserChangeEmailRequestResponse
// @Failure 400 {object} shared.BadRequestError
// @Failure 409 {object} shared.ConflictError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /users/email/change/request [post]
func (controller Controller) RequestEmailChange(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.RequestEmailChange")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	var payload UserChangeEmailRequestRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	var response UserChangeEmailRequestResponse
	response, err = controller.Service.RequestEmailChange(ctx, userId, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}
	return shared.SendSuccessResponseWithData(ctx, response)
}

// ConfirmEmailChange godoc
// @Summary Confirm the pending email change with an OTP
// @description.markdown confirm_email_change
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body user.UserChangeEmailConfirmRequest true "OTP code"
// @Success 200 {object} map[string]string
// @Failure 400 {object} shared.BadRequestError
// @Failure 401 {object} shared.UnauthorizedError
// @Router /users/email/change/confirm [post]
func (controller Controller) ConfirmEmailChange(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.ConfirmEmailChange")
	ctx.SetContext(ctxContext)
	var err error

	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	var payload UserChangeEmailConfirmRequest
	err = shared.ReadRequestBody(ctx, &payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}

	err = controller.Service.ConfirmEmailChange(ctx, userId, payload)
	if err != nil {
		return shared.SendError(ctx, err)
	}
	return shared.SendSuccessResponseNoData(ctx)
}

// UpdateNotificationPreferences godoc
// @Summary Update the logged-in user's push notification preferences
// @description.markdown update_notification_preferences
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body user.UpdateNotificationPreferencesRequest true "Notification preference flags"
// @Success 200 {object} map[string]string
// @Failure 401 {object} shared.UnauthorizedError
// @Router /users/me/notification-preferences [put]
func (controller Controller) UpdateNotificationPreferences(ctx fiber.Ctx) error {
	ctxContext := ctx.Context()
	serviceName := controller.Config.String("OTEL_SERVICE_NAME")
	ctxContext, span := otel.Tracer(serviceName+"-controller").Start(ctxContext, "controller.UpdateNotificationPreferences")
	ctx.SetContext(ctxContext)
	var err error
	defer func() {
		if err != nil {
			shared.RecordErrorTelemetry(ctxContext, span, err)
		}
		span.End()
	}()

	userId := ctx.Locals("userId").(string)

	var request UpdateNotificationPreferencesRequest
	if err = shared.ReadRequestBody(ctx, &request); err != nil {
		return shared.SendError(ctx, err)
	}

	if err = controller.Service.UpdateNotificationPreferences(ctx, userId, request); err != nil {
		return shared.SendError(ctx, err)
	}

	return shared.SendSuccessResponseNoData(ctx)
}

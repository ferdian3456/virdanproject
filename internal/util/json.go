package util

import (
	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/gofiber/fiber/v3"
)

func ReadRequestBody(ctx fiber.Ctx, result interface{}) error {
	err := ctx.Bind().Body(result)
	if err != nil {
		return &model.BadRequestError{
			Code:    constant.ERR_BAD_REQUEST_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		}
	}
	return nil
}

func SendSuccessResponseNoData(ctx fiber.Ctx) error {
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"status": "OK",
	})
}

func SendSuccessResponseWithData(ctx fiber.Ctx, data interface{}) error {
	return ctx.Status(fiber.StatusOK).JSON(data)
}

// SendError writes the error response. Telemetry recording is delegated to
// ObservabilityMiddleware via ctx.Locals("handler_error", err).
func SendError(ctx fiber.Ctx, err error) error {
	statusCode := fiber.StatusInternalServerError
	errCode := constant.ERR_INTERNAL_SERVER_ERROR_CODE
	message := err.Error()
	param := ""

	if apiErr, ok := err.(model.ApiError); ok {
		statusCode = apiErr.StatusCode()
		errCode = apiErr.GetCode()
		param = apiErr.GetParam()
	} else { // internal server error, failed to query into postgres
		statusCode = fiber.StatusInternalServerError
		errCode = constant.ERR_INTERNAL_SERVER_ERROR_CODE
		message = constant.ERR_INTENRAL_SERVER_ERROR_MESSAGE
		param = ""
	}

	ctx.Locals("handler_error", err)

	return ctx.Status(statusCode).JSON(fiber.Map{
		"error": fiber.Map{
			"code":    errCode,
			"message": message,
			"param":   param,
		},
	})
}

func ReadMultipartBody(ctx fiber.Ctx) error {
	ct := ctx.Get("Content-Type")
	if len(ct) < 19 || ct[:19] != "multipart/form-data" {
		return &model.BadRequestError{
			Code:    constant.ERR_BAD_REQUEST_CODE,
			Message: constant.ERR_INVALID_CONTENT_TYPE_MESSAGE,
			Param:   "Content-Type",
		}
	}
	return nil
}

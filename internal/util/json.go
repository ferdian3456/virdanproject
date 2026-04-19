package util

import (
	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/gofiber/fiber/v3"
)

func ReadRequestBody(ctx fiber.Ctx, result interface{}) error {
	err := ctx.Bind().Body(result)
	if err != nil {
		return err
	}
	return nil
}

func SendSuccessResponseNoData(ctx fiber.Ctx) error {
	err := ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"status": "OK",
	})
	if err != nil {
		return err
	}
	return nil
}

func SendSuccessResponseWithData(ctx fiber.Ctx, data interface{}) error {
	err := ctx.Status(fiber.StatusOK).JSON(data)
	if err != nil {
		return err
	}

	return nil
}

func SendError(ctx fiber.Ctx, err error) error {
	statusCode := fiber.StatusInternalServerError
	errCode := constant.ERR_INTERNAL_SERVER_ERROR_CODE
	param := ""

	if apiErr, ok := err.(model.ApiError); ok {
		statusCode = apiErr.StatusCode()
		errCode = apiErr.GetCode()
		param = apiErr.GetParam()
	}

	if statusCode >= 400 && statusCode < 500 {
		RecordErrorTelemetry(ctx.Context(), nil, err)
	}

	return ctx.Status(statusCode).JSON(fiber.Map{
		"error": fiber.Map{
			"code":    errCode,
			"message": err.Error(),
			"param":   param,
		},
	})
}

func ReadMultipartBody(ctx fiber.Ctx, payload any) error {
	if len(ctx.Get("Content-Type")) < 19 || ctx.Get("Content-Type")[:19] != "multipart/form-data" {
		return &model.BadRequestError{
			Code:    constant.ERR_BAD_REQUEST_CODE,
			Message: constant.ERR_INVALID_CONTENT_TYPE_MESSAGE,
			Param:   "Content-Type",
		}
	}

	if err := ctx.Bind().Body(payload); err != nil {
		return &model.BadRequestError{
			Code:    constant.ERR_INVALID_REQUEST_BODY_ERROR_CODE,
			Message: constant.ERR_INVALID_REQUEST_BODY_MESSAGE,
		}
	}
	return nil
}

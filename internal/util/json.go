package util

import (
	"github.com/ferdian3456/virdanproject/internal/constant"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func ReadRequestBody(ctx *fiber.Ctx, result interface{}) error {
	err := ctx.BodyParser(&result)
	if err != nil {
		return err
	}
	return nil
}

func SendSuccessResponseNoData(ctx *fiber.Ctx) error {
	err := ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"status": "OK",
	})
	if err != nil {
		return err
	}
	return nil
}

func SendSuccessResponseWithData(ctx *fiber.Ctx, data interface{}) error {
	err := ctx.Status(fiber.StatusOK).JSON(data)
	if err != nil {
		return err
	}

	return nil
}

func SendErrorResponse(ctx *fiber.Ctx, error error) error {
	err := ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"error": error,
	})
	if err != nil {
		return err
	}

	return nil
}

func SendErrorResponseNotFound(ctx *fiber.Ctx, error error) error {
	err := ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"error": error,
	})
	if err != nil {
		return err
	}

	return nil
}

func SendErrorResponseInternalServer(ctx *fiber.Ctx, log *zap.Logger, error error) error {
	log.Error("internal server error occured", zap.Error(error))
	err := ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": fiber.Map{
			"code":    constant.ERR_INTERNAL_SERVER_ERROR_CODE,
			"message": constant.ERR_INTENRAL_SERVER_ERROR_MESSAGE,
		},
	})

	if err != nil {
		return err
	}

	return err
}

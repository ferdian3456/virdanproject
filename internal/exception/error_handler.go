package middleware

import (
	"fmt"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func Recovery(log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				// Handle nil panic
				if r == nil {
					return
				}

				// Extract error message
				var errMsg string
				switch v := r.(type) {
				case error:
					errMsg = v.Error()
				case string:
					errMsg = v
				default:
					errMsg = fmt.Sprintf("%v", v)
				}

				// Log panic with full context
				log.Error("panic occurred and recovered", zap.String("error", errMsg))

				// Send standardized error response
				_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fiber.Map{
						"code":    constant.ERR_INTERNAL_SERVER_ERROR_CODE,
						"message": constant.ERR_INTENRAL_SERVER_ERROR_MESSAGE,
					},
				})
			}
		}()

		return c.Next()
	}
}

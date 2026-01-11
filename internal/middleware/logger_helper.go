package middleware

import (
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// GetLoggerFromContext retrieves the trace-aware logger from Fiber context
// Returns the logger with trace_id and span_id injected by middleware
func GetLoggerFromContext(c *fiber.Ctx) *zap.Logger {
	// Get trace-aware logger from middleware
	loggerIf := c.Locals("logger")
	if loggerIf != nil {
		if logger, ok := loggerIf.(*zap.Logger); ok {
			return logger
		}
	}

	// Fallback: return a basic logger (should not happen in normal flow)
	return zap.NewNop()
}

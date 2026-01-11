package middleware

import (
	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// TraceLoggerMiddleware injects trace ID and span ID into logger for trace-log correlation
// This enables logs to be linked with traces in ClickStack/HyperDX
func TraceLoggerMiddleware(logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get span from OpenTelemetry context
		span := trace.SpanFromContext(c.UserContext())
		spanContext := span.SpanContext()

		// Create logger with trace correlation fields
		traceLogger := logger.With(
			zap.String("trace_id", spanContext.TraceID().String()),
			zap.String("span_id", spanContext.SpanID().String()),
		)

		// Store trace-aware logger in context for use in handlers
		c.Locals("logger", traceLogger)

		return c.Next()
	}
}

package util

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/gofiber/fiber/v3"
)

// GetLoggerWithTraceContext returns logger with trace context fields for OTel correlation
func GetLoggerWithTraceContext(ctx context.Context, logger *zap.Logger) *zap.Logger {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return logger.With(
			zap.String("trace_id", span.SpanContext().TraceID().String()),
			zap.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return logger
}

// RecordValidationError logs a validation error and records it on the span
func RecordValidationError(ctx context.Context, logger *zap.Logger, span trace.Span, err *model.ValidationError, action string) {
	if span == nil {
		span = trace.SpanFromContext(ctx)
	}

	GetLoggerWithTraceContext(ctx, logger).Warn(action+" validation failed",
		zap.String("code", err.Code),
		zap.String("param", err.Param),
		zap.String("message", err.Message),
	)

	if span.SpanContext().IsValid() {
		span.RecordError(err, trace.WithAttributes(
			attribute.String("error.code", err.Code),
			attribute.String("error.param", err.Param),
		))
		span.SetStatus(codes.Error, err.Message)
	}
}

// RecordAndSendValidationError records a validation error and sends it as a 400 response
func RecordAndSendValidationError(ctx fiber.Ctx, logger *zap.Logger, err *model.ValidationError, action string) error {
	RecordValidationError(ctx.Context(), logger, nil, err, action)
	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"error": err,
	})
}

// RecordAndSendValidationErrorNotFound records a validation error and sends it as a 404 response
func RecordAndSendValidationErrorNotFound(ctx fiber.Ctx, logger *zap.Logger, err *model.ValidationError, action string) error {
	RecordValidationError(ctx.Context(), logger, nil, err, action)
	return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"error": err,
	})
}

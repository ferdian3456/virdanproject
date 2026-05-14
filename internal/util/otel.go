package util

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/ferdian3456/virdanproject/internal/model"
)

// GetLoggerWithTraceContext returns logger with trace context fields for OTel correlation.
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

// RecordErrorTelemetry sets span status to Error and records the error.
// If error implements model.ApiError, also adds error.code and error.param attributes.
func RecordErrorTelemetry(ctx context.Context, span trace.Span, err error) {
	if span == nil {
		span = trace.SpanFromContext(ctx)
	}
	if span == nil || !span.SpanContext().IsValid() {
		return
	}

	opts := []trace.EventOption{}
	if apiErr, ok := err.(model.ApiError); ok {
		opts = append(opts, trace.WithAttributes(
			attribute.String("error.code", apiErr.GetCode()),
			attribute.String("error.param", apiErr.GetParam()),
		))
	}

	span.RecordError(err, opts...)
	span.SetStatus(codes.Error, err.Error())
}

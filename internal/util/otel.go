package util

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/ferdian3456/virdanproject/internal/model"
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

// RecordErrorTelemetry memproses dan menstempel spesifik Atribut error ke dalam Span yang sudah direkam
func RecordErrorTelemetry(ctx context.Context, span trace.Span, err error) {
	if span == nil {
		span = trace.SpanFromContext(ctx)
	}

	var errCode, errParam string

	if apiErr, ok := err.(model.ApiError); ok {
		errCode = apiErr.GetCode()
		errParam = apiErr.GetParam()
	}
	if span != nil && span.SpanContext().IsValid() {
		span.RecordError(err, trace.WithAttributes(
			attribute.String("error.code", errCode),
			attribute.String("error.param", errParam),
		))
	}
}

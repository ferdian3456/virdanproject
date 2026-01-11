package observability

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

type TraceContext struct {
	TraceID string
	SpanID  string
}

func ExtractTrace(ctx context.Context) *TraceContext {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return nil
	}

	sc := span.SpanContext()

	return &TraceContext{
		TraceID: sc.TraceID().String(),
		SpanID:  sc.SpanID().String(),
	}
}

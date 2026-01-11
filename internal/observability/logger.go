package observability

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type contextAwareCore struct {
	zapcore.Core
}

func (c *contextAwareCore) With(fields []zapcore.Field) zapcore.Core {
	return &contextAwareCore{
		Core: c.Core.With(fields),
	}
}

func (c *contextAwareCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	return c.Core.Write(entry, fields)
}

// Public helper
func WithContext(ctx context.Context, logger *zap.Logger) *zap.Logger {
	tc := ExtractTrace(ctx)
	if tc == nil {
		return logger
	}

	return logger.With(
		zap.String("trace_id", tc.TraceID),
		zap.String("span_id", tc.SpanID),
	)
}

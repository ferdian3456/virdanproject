package config

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewZap() *zap.Logger {
	levelStr := strings.ToLower("DEBUG")
	var level zapcore.Level
	switch levelStr {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	case "panic":
		level = zapcore.PanicLevel
	case "fatal":
		level = zapcore.FatalLevel
	default:
		level = zapcore.InfoLevel // default
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(level)
	cfg.DisableStacktrace = true
	cfg.DisableCaller = true
	cfg.EncoderConfig.StacktraceKey = ""
	cfg.EncoderConfig.TimeKey = "timestamp"

	log, _ := cfg.Build()

	return log
}

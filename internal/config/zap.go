package config

import (
	"os"
	"strings"

	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/contrib/bridges/otelzap"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewBootstrapZap() *zap.Logger {
	cfg := zap.NewProductionConfig()
	cfg.DisableStacktrace = true
	cfg.DisableCaller = false
	cfg.EncoderConfig.StacktraceKey = ""
	cfg.EncoderConfig.TimeKey = "timestamp"

	log, _ := cfg.Build()

	return log
}

func NewZap(koanf *koanf.Koanf, loggerProvider *sdklog.LoggerProvider) *zap.Logger {
	serviceName := koanf.String("OTEL_SERVICE_NAME")
	serviceVersion := koanf.String("OTEL_SERVICE_VERSION")
	deploymentEnvironment := koanf.String("OTEL_DEPLOYMENT_ENVIRONMENT")
	logLevel := koanf.String("LOG_LEVEL")

	otelCore := otelzap.NewCore(serviceName, otelzap.WithLoggerProvider(loggerProvider))

	levelStr := strings.ToLower(logLevel)
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

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	encoder := zapcore.NewJSONEncoder(encoderCfg)
	consoleCore := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	combinedCore := zapcore.NewTee(consoleCore, otelCore)

	logger := zap.New(combinedCore,
		zap.AddCaller(),
		zap.Fields(
			zap.String("service", serviceName),
			zap.String("version", serviceVersion),
			zap.String("environment", deploymentEnvironment),
		),
	)

	logger.Info("Zap logger initialized",
		zap.String("service", serviceName),
		zap.String("version", serviceVersion),
		zap.String("environment", deploymentEnvironment),
		zap.String("log_level", logLevel),
	)

	return logger
}

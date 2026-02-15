package config

import (
	"context"
	"os"
	"strings"

	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
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

func NewZap(ctx context.Context, koanf *koanf.Koanf, bootstrapZap *zap.Logger) *zap.Logger {
	serviceName := koanf.String("OTEL_SERVICE_NAME")
	serviceVersion := koanf.String("OTEL_SERVICE_VERSION")
	deploymentEnvironment := koanf.String("OTEL_DEPLOYMENT_ENVIRONMENT")
	otelExporterOTLPEndpoint := koanf.String("OTEL_EXPORTER_OTLP_ENDPOINT")
	batchExportTimeout := koanf.Duration("OTEL_LOG_BATCH_EXPORT_TIMEOUT")
	batchTimeout := koanf.Duration("OTEL_LOG_BATCH_TIMEOUT")
	batchMaxQueueSize := koanf.Int("OTEL_LOG_BATCH_MAX_QUEUE_SIZE")
	logLevel := koanf.String("LOG_LEVEL")

	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(serviceVersion),
		semconv.DeploymentEnvironment(deploymentEnvironment),
	),
		resource.WithHost(),
	)

	if err != nil {
		bootstrapZap.Fatal("failed to create resource", zap.Error(err))
	}

	exporter, err := otlploggrpc.New(ctx,
		otlploggrpc.WithEndpoint(otelExporterOTLPEndpoint),
		otlploggrpc.WithInsecure(),
	)
	if err != nil {
		bootstrapZap.Fatal("failed to create exporter", zap.Error(err))
	}

	processor := sdklog.NewBatchProcessor(
		exporter,
		sdklog.WithExportTimeout(batchExportTimeout),
		sdklog.WithExportInterval(batchTimeout),
		sdklog.WithMaxQueueSize(batchMaxQueueSize),
	)

	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(processor),
	)

	global.SetLoggerProvider(loggerProvider)

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

	logger.Info("OpenTelemetry logger initialized",
		zap.String("otlp_endpoint", otelExporterOTLPEndpoint),
		zap.String("service", serviceName),
		zap.String("version", serviceVersion),
		zap.String("environment", deploymentEnvironment),
		zap.String("batch_export_timeout", batchExportTimeout.String()),
		zap.String("batch_timeout", batchTimeout.String()),
		zap.Int("batch_max_queue_size", batchMaxQueueSize),
		zap.String("log_level", logLevel),
	)

	return logger
}

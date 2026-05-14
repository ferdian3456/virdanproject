package config

import (
	"context"

	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/exemplar"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.uber.org/zap"
)

func NewOtelLoggerProvider(ctx context.Context, koanf *koanf.Koanf, log *zap.Logger) *sdklog.LoggerProvider {
	serviceName := koanf.String("OTEL_SERVICE_NAME")
	serviceVersion := koanf.String("OTEL_SERVICE_VERSION")
	deploymentEnvironment := koanf.String("OTEL_DEPLOYMENT_ENVIRONMENT")
	otelExporterOTLPEndpoint := koanf.String("OTEL_EXPORTER_OTLP_ENDPOINT")
	batchExportTimeout := koanf.Duration("OTEL_LOG_BATCH_EXPORT_TIMEOUT")
	batchTimeout := koanf.Duration("OTEL_LOG_BATCH_TIMEOUT")
	batchMaxQueueSize := koanf.Int("OTEL_LOG_BATCH_MAX_QUEUE_SIZE")

	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(serviceVersion),
		semconv.DeploymentEnvironment(deploymentEnvironment),
	),
		resource.WithHost(),
	)
	if err != nil {
		log.Fatal("failed to create resource", zap.Error(err))
	}

	exporter, err := otlploggrpc.New(ctx,
		otlploggrpc.WithEndpoint(otelExporterOTLPEndpoint),
		otlploggrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatal("failed to create log exporter", zap.Error(err))
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

	log.Info("OpenTelemetry logger initialized",
		zap.String("otlp_endpoint", otelExporterOTLPEndpoint),
		zap.String("service", serviceName),
		zap.String("version", serviceVersion),
		zap.String("environment", deploymentEnvironment),
		zap.String("batch_export_timeout", batchExportTimeout.String()),
		zap.String("batch_timeout", batchTimeout.String()),
		zap.Int("batch_max_queue_size", batchMaxQueueSize),
	)

	return loggerProvider
}

func NewOtelMetricProvider(ctx context.Context, koanf *koanf.Koanf, log *zap.Logger) *sdkmetric.MeterProvider {
	serviceName := koanf.String("OTEL_SERVICE_NAME")
	serviceVersion := koanf.String("OTEL_SERVICE_VERSION")
	deploymentEnvironment := koanf.String("OTEL_DEPLOYMENT_ENVIRONMENT")
	otelExporterOTLPEndpoint := koanf.String("OTEL_EXPORTER_OTLP_ENDPOINT")
	intervalMetrics := koanf.Duration("OTEL_METRIC_INTERVAL")

	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(serviceVersion),
		semconv.DeploymentEnvironment(deploymentEnvironment),
	),
		resource.WithHost(),
	)
	if err != nil {
		log.Fatal("Failed to create resource", zap.Error(err))
	}

	exporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(otelExporterOTLPEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatal("Failed to create metrics exporter", zap.Error(err))
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(intervalMetrics))),
		sdkmetric.WithExemplarFilter(exemplar.TraceBasedFilter),
	)

	otel.SetMeterProvider(meterProvider)

	if err := runtime.Start(); err != nil {
		log.Fatal("Failed to start runtime metrics", zap.Error(err))
	}

	log.Info("OpenTelemetry metrics initialized",
		zap.String("otlp_endpoint", otelExporterOTLPEndpoint),
		zap.String("service", serviceName),
		zap.String("version", serviceVersion),
		zap.String("environment", deploymentEnvironment),
		zap.Duration("interval", intervalMetrics),
	)

	return meterProvider
}

func NewOtelTracerProvider(ctx context.Context, koanf *koanf.Koanf, log *zap.Logger) *sdktrace.TracerProvider {
	traceServiceName := koanf.String("OTEL_SERVICE_NAME")
	traceServiceVersion := koanf.String("OTEL_SERVICE_VERSION")
	traceDeploymentEnvironment := koanf.String("OTEL_DEPLOYMENT_ENVIRONMENT")
	otelTraceExporterOTLPEndpoint := koanf.String("OTEL_EXPORTER_OTLP_ENDPOINT")
	batchTimeout := koanf.Duration("OTEL_TRACE_BATCH_TIMEOUT")
	batchExportTimeout := koanf.Duration("OTEL_TRACE_BATCH_EXPORT_TIMEOUT")
	maxQueueSize := koanf.Int("OTEL_TRACE_MAX_QUEUE_SIZE")
	sampleRate := koanf.Float64("OTEL_TRACE_SAMPLE_RATE")

	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(traceServiceName),
		semconv.ServiceVersion(traceServiceVersion),
		semconv.DeploymentEnvironment(traceDeploymentEnvironment),
	),
		resource.WithHost(),
	)
	if err != nil {
		log.Fatal("Failed to create resource", zap.Error(err))
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otelTraceExporterOTLPEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatal("Failed to create trace exporter", zap.Error(err))
	}

	batchProcessor := sdktrace.NewBatchSpanProcessor(
		exporter,
		sdktrace.WithMaxQueueSize(maxQueueSize),
		sdktrace.WithBatchTimeout(batchTimeout),
		sdktrace.WithExportTimeout(batchExportTimeout),
	)

	// ParentBased sampler: respects parent decision, uses ratio for root spans
	sampler := sdktrace.ParentBased(
		sdktrace.TraceIDRatioBased(sampleRate),
	)

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(batchProcessor),
		sdktrace.WithSampler(sampler),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Info("OpenTelemetry tracer initialized",
		zap.String("otlp_endpoint", otelTraceExporterOTLPEndpoint),
		zap.String("service", traceServiceName),
		zap.String("version", traceServiceVersion),
		zap.String("environment", traceDeploymentEnvironment),
		zap.String("batch_timeout", batchTimeout.String()),
		zap.String("batch_export_timeout", batchExportTimeout.String()),
		zap.Int("batch_max_queue_size", maxQueueSize),
		zap.Float64("sample_rate", sampleRate),
	)

	return tracerProvider
}

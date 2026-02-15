package config

import (
	"context"

	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.uber.org/zap"
)

func NewTracer(ctx context.Context, koanf *koanf.Koanf, log *zap.Logger) *sdktrace.TracerProvider {
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
		log.Fatal("failed to create resource", zap.Error(err))
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otelTraceExporterOTLPEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatal("failed to create trace exporter", zap.Error(err))
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

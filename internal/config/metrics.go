package config

import (
	"context"

	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/exemplar"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.uber.org/zap"
)

func NewMetrics(ctx context.Context, koanf *koanf.Koanf, log *zap.Logger) *sdkmetric.MeterProvider {
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
		log.Fatal("failed to create resource", zap.Error(err))
	}

	exporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(otelExporterOTLPEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatal("failed to create metrics exporter", zap.Error(err))
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(intervalMetrics))),
		sdkmetric.WithExemplarFilter(exemplar.TraceBasedFilter),
	)

	otel.SetMeterProvider(meterProvider)

	if err := runtime.Start(); err != nil {
		log.Fatal("failed to start runtime metrics", zap.Error(err))
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

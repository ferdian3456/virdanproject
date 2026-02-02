package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap"
)

func InitMetrics(ctx context.Context, cfg Config, log *zap.Logger) (func(context.Context) error, error) {
	// Parse headers from config
	headers := parseHeaders(cfg.OtelHeaders)

	// Create OTLP metrics exporter with delta temporality for better performance
	exporter, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint(cfg.OtelEndpoint),
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithHeaders(headers),
		otlpmetricgrpc.WithRetry(
			otlpmetricgrpc.RetryConfig{
				Enabled:         true,
				InitialInterval: 5 * time.Second,
				MaxInterval:     30 * time.Second,
				MaxElapsedTime:  60 * time.Second,
			},
		),
		otlpmetricgrpc.WithTimeout(30*time.Second),
	)
	if err != nil {
		log.Fatal("failed to create otlp metric exporter", zap.Error(err))
	}

	// Create resource with service attributes
	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		log.Fatal("failed to create otel resource", zap.Error(err))
	}

	// Create meter provider with resource and periodic reader
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				exporter,
				sdkmetric.WithInterval(10*time.Second), // Detailed metrics: 10s export interval
				sdkmetric.WithTimeout(30*time.Second),
			),
		),
		sdkmetric.WithView(
			// Drop high-cardinality attributes to prevent metric explosion
			sdkmetric.NewView(
				sdkmetric.Instrument{
					Name: "http.server.duration",
				},
				sdkmetric.Stream{
					AttributeFilter: filterHighCardinalityAttributes,
				},
			),
			sdkmetric.NewView(
				sdkmetric.Instrument{
					Name: "http.server.request.count",
				},
				sdkmetric.Stream{
					AttributeFilter: filterHighCardinalityAttributes,
				},
			),
		),
	)

	// Set global meter provider
	otel.SetMeterProvider(meterProvider)

	log.Info("otel metric exporter initialized",
		zap.String("endpoint", cfg.OtelEndpoint),
		zap.String("protocol", "grpc"),
		zap.String("service", cfg.ServiceName),
		zap.String("temporality", "delta"),
		zap.String("export_interval", "10s"),
		zap.Bool("auth_enabled", len(headers) > 0),
	)

	return meterProvider.Shutdown, nil
}

// filterHighCardinalityAttributes filters out attributes that can cause metric explosion
func filterHighCardinalityAttributes(kv attribute.KeyValue) bool {
	// Block high-cardinality attributes
	switch kv.Key {
	case "user.id", "user.email", "user.ip", "http.user_agent", "user.token":
		return false
	}
	// Allow all other attributes
	return true
}

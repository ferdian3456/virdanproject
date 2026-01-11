package observability

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap"
)

func parseHeaders(headersStr string) map[string]string {
	headers := make(map[string]string)
	if headersStr == "" {
		return headers
	}

	// Parse format: "key1=value1,key2=value2"
	parts := strings.Split(headersStr, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}

	return headers
}

func Init(ctx context.Context, cfg Config, log *zap.Logger) (func(context.Context) error, error) {
	// Parse headers from config
	headers := parseHeaders(cfg.OtelHeaders)

	// Create OTLP HTTP exporter (more stable than gRPC for ClickStack/HyperDX)
	exporter, err := otlptracehttp.New(
		ctx,
		otlptracehttp.WithEndpoint(cfg.OtelEndpoint),
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithHeaders(headers),
		otlptracehttp.WithRetry(
			otlptracehttp.RetryConfig{
				Enabled:         true,
				InitialInterval: 5 * time.Second,
				MaxInterval:     30 * time.Second,
				MaxElapsedTime:  60 * time.Second,
			},
		),
		otlptracehttp.WithTimeout(30*time.Second),
	)
	if err != nil {
		log.Fatal("failed to create otlp trace exporter", zap.Error(err))
	}

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

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(10*time.Second),
			sdktrace.WithExportTimeout(30*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
		),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	log.Info("otel trace exporter initialized",
		zap.String("endpoint", cfg.OtelEndpoint),
		zap.String("protocol", "http/protobuf"),
		zap.String("service", cfg.ServiceName),
		zap.Bool("auth_enabled", len(headers) > 0),
	)

	return tp.Shutdown, nil
}

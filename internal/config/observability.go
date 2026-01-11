package config

import (
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
)

type ObservabilityConfig struct {
	OtelEndpoint string
	ServiceName  string
	Environment  string
	OtelHeaders  string
}

func LoadObservabilityConfig(config *koanf.Koanf, log *zap.Logger) ObservabilityConfig {
	observabilityConfig := ObservabilityConfig{
		OtelEndpoint: config.String("OTEL_EXPORTER_OTLP_ENDPOINT"),
		ServiceName:  config.String("OTEL_SERVICE_NAME"),
		Environment:  config.String("ENVIRONMENT"),
		OtelHeaders:  config.String("OTEL_EXPORTER_OTLP_HEADERS"),
	}

	if observabilityConfig.ServiceName == "" {
		log.Fatal("failed to get observability config")
	}

	return observabilityConfig
}

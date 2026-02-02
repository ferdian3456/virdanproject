package observability

import "context"

type Config struct {
	ServiceName     string
	Environment     string
	OtelEndpoint    string
	OtelHeaders     string
	TraceShutdown   func(context.Context) error
	MetricsShutdown func(context.Context) error
}

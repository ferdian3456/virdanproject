package observability

type Config struct {
	ServiceName  string
	Environment  string
	OtelEndpoint string
	OtelHeaders  string
}

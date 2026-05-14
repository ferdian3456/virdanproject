package middleware

import (
	"time"

	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func ObservabilityMiddleware(koanfInstance *koanf.Koanf, meterProvider metric.MeterProvider, log *zap.Logger) fiber.Handler {
	serviceName := koanfInstance.String("OTEL_SERVICE_NAME")
	tracer := otel.Tracer(serviceName + "-http")
	propagator := otel.GetTextMapPropagator()

	meter := meterProvider.Meter(serviceName + "-http")
	requestCount, _ := meter.Int64Counter("http.requests_total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("{request}"),
	)
	requestDuration, _ := meter.Float64Histogram("http.request.duration_ms",
		metric.WithDescription("HTTP request duration in milliseconds"),
		metric.WithUnit("ms"),
	)
	requestSize, _ := meter.Int64Histogram("http.request.size_bytes",
		metric.WithDescription("HTTP request body size in bytes"),
		metric.WithUnit("By"),
	)
	responseSize, _ := meter.Int64Histogram("http.response.size_bytes",
		metric.WithDescription("HTTP response body size in bytes"),
		metric.WithUnit("By"),
	)

	return func(c fiber.Ctx) error {
		start := time.Now()

		requestID := uuid.New().String()
		c.Set(fiber.HeaderXRequestID, requestID)

		ctx := propagator.Extract(c.Context(), propagation.HeaderCarrier(c.GetReqHeaders()))

		spanName := c.Method() + " " + c.Path()
		if route := c.Route(); route != nil && route.Path != "" {
			spanName = c.Method() + " " + route.Path
		}

		ctx, span := tracer.Start(ctx, spanName,
			trace.WithAttributes(
				attribute.String("http.method", c.Method()),
				attribute.String("http.path", c.Path()),
				attribute.String("http.url", c.OriginalURL()),
				attribute.String("http.request_id", requestID),
				attribute.String("http.client_ip", c.IP()),
				attribute.String("http.user_agent", c.Get("User-Agent")),
			),
			trace.WithSpanKind(trace.SpanKindServer),
		)
		c.SetContext(ctx)

		logger := util.GetLoggerWithTraceContext(ctx, log)
		logger.Info("Incoming request",
			zap.String("http.method", c.Method()),
			zap.String("http.path", c.Path()),
			zap.String("http.request_id", requestID),
			zap.String("http.user_agent", c.Get("User-Agent")),
			zap.String("http.client_ip", c.IP()),
		)

		err := c.Next()

		duration := time.Since(start)
		statusCode := c.Response().StatusCode()

		if route := c.Route(); route != nil && route.Path != "" {
			span.SetName(c.Method() + " " + route.Path)
			span.SetAttributes(attribute.String("http.route", route.Path))
		}
		span.SetAttributes(attribute.Int("http.status_code", statusCode))

		finalErr := err
		if finalErr == nil {
			if handlerErr, ok := c.Locals("handler_error").(error); ok {
				finalErr = handlerErr
			}
		}
		if finalErr != nil {
			util.RecordErrorTelemetry(ctx, span, finalErr)
		}

		routePath := ""
		if route := c.Route(); route != nil {
			routePath = route.Path
		}

		metricAttrs := metric.WithAttributes(
			attribute.String("http.method", c.Method()),
			attribute.String("http.route", routePath),
			attribute.Int("http.status_code", statusCode),
		)
		requestCount.Add(ctx, 1, metricAttrs)
		requestDuration.Record(ctx, float64(duration.Milliseconds()), metricAttrs)

		sizeAttrs := metric.WithAttributes(
			attribute.String("http.method", c.Method()),
			attribute.String("http.route", routePath),
		)
		requestSize.Record(ctx, int64(len(c.Request().Body())), sizeAttrs)
		responseSize.Record(ctx, int64(len(c.Response().Body())), sizeAttrs)

		logger.Info("Request completed",
			zap.String("http.method", c.Method()),
			zap.String("http.route", routePath),
			zap.String("http.path", c.Path()),
			zap.String("http.request_id", requestID),
			zap.Int("http.status_code", statusCode),
			zap.Int64("http.request.duration_ms", duration.Milliseconds()),
		)

		span.End()
		propagator.Inject(ctx, propagation.HeaderCarrier(c.GetRespHeaders()))

		return err
	}
}

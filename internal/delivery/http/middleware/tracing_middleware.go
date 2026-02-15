package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TracingMiddleware(koanf *koanf.Koanf) fiber.Handler {
	serviceName := koanf.String("OTEL_SERVICE_NAME")
	tracer := otel.Tracer(serviceName + "-tracing")
	propagator := otel.GetTextMapPropagator()

	return func(c fiber.Ctx) error {
		ctx := propagator.Extract(c, propagation.HeaderCarrier(c.GetReqHeaders()))

		spanName := c.Method() + " " + c.Path()
		if route := c.Route(); route != nil {
			spanName = c.Method() + " " + route.Path
		}

		requestID := string(c.Response().Header.Peek(fiber.HeaderXRequestID))

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
		defer span.End()

		// Update context with span
		// Fiber v3 handles context differently, we can use c.SetContext
		c.SetContext(ctx)

		// Process request
		err := c.Next()

		// Update span name and attributes after route matching
		if route := c.Route(); route != nil && route.Path != "" {
			span.SetName(c.Method() + " " + route.Path)
			span.SetAttributes(attribute.String("http.route", route.Path))
		}

		// Recording response attributes
		statusCode := c.Response().StatusCode()
		span.SetAttributes(attribute.Int("http.status_code", statusCode))

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		// Inject trace context into response headers
		propagator.Inject(ctx, propagation.HeaderCarrier(c.GetRespHeaders()))

		return err
	}
}

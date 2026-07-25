package shared

import (
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/google/uuid"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type AccessTokenChecker interface {
	GetAccessToken(ctx fiber.Ctx, userId string, accessToken string) error
}

type AuthMiddleware struct {
	Config  *koanf.Koanf
	Log     *zap.Logger
	Checker AccessTokenChecker
}

func NewAuthMiddleware(config *koanf.Koanf, log *zap.Logger, checker AccessTokenChecker) *AuthMiddleware {
	return &AuthMiddleware{
		Config:  config,
		Log:     log,
		Checker: checker,
	}
}

func (middleware *AuthMiddleware) WsProtectedRoute() fiber.Handler {
	return func(ctx fiber.Ctx) error {
		ctxContext := ctx.Context()
		serviceName := middleware.Config.String("OTEL_SERVICE_NAME")
		ctxContext, span := otel.Tracer(serviceName+"-middleware").Start(ctxContext, "middleware.WsProtectedRoute")
		ctx.SetContext(ctxContext)
		var err error

		defer func() {
			if err != nil {
				RecordErrorTelemetry(ctxContext, span, err)
			}
			span.End()
		}()

		tokenString := ctx.Query("token")
		if tokenString == "" {
			err = &UnauthorizedError{
				Code:    ERR_UNAUTHORIZED_ERROR,
				Message: "Missing token query parameter",
				Param:   "token",
			}
			return SendError(ctx, err)
		}

		var userId string
		tokenString, userId, err = ValidateAccessToken(tokenString, middleware.Config.String("JWT_SECRET_KEY"))
		if err != nil {
			return SendError(ctx, err)
		}

		err = middleware.Checker.GetAccessToken(ctx, userId, tokenString)
		if err != nil {
			return SendError(ctx, err)
		}

		ctx.Locals("userId", userId)

		middleware.Log.Debug("ws auth success", zap.String("userId", userId))

		err = ctx.Next()
		return err
	}
}

func (middleware *AuthMiddleware) ProtectedRoute() fiber.Handler {
	return func(ctx fiber.Ctx) error {
		ctxContext := ctx.Context()
		serviceName := middleware.Config.String("OTEL_SERVICE_NAME")
		ctxContext, span := otel.Tracer(serviceName+"-middleware").Start(ctxContext, "middleware.ProtectedRoute")
		ctx.SetContext(ctxContext)
		var err error

		defer func() {
			if err != nil {
				RecordErrorTelemetry(ctxContext, span, err)
			}
			span.End()
		}()

		authHeader := ctx.Get("Authorization")
		if authHeader == "" {
			err = &UnauthorizedError{
				Code:    ERR_UNAUTHORIZED_ERROR,
				Message: "Authorization header is missing",
				Param:   "authorization",
			}
			return SendError(ctx, err)
		}

		if !strings.HasPrefix(authHeader, BearerPrefix) {
			err = &UnauthorizedError{
				Code:    ERR_UNAUTHORIZED_ERROR,
				Message: "Invalid authorization scheme",
				Param:   "authorization",
			}
			return SendError(ctx, err)
		}

		tokenString := strings.TrimPrefix(authHeader, BearerPrefix)

		var userId string
		tokenString, userId, err = ValidateAccessToken(tokenString, middleware.Config.String("JWT_SECRET_KEY"))
		if err != nil {
			return SendError(ctx, err)
		}

		err = middleware.Checker.GetAccessToken(ctx, userId, tokenString)
		if err != nil {
			return SendError(ctx, err)
		}

		ctx.Locals("userId", userId)

		middleware.Log.Debug("auth success", zap.String("userId", userId))

		err = ctx.Next()
		return err
	}
}

func CORSMiddleware() fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"X-Requested-With", "Content-Type", "Authorization", "ngrok-skip-browser-warning"},
		AllowCredentials: false,
		ExposeHeaders:    []string{"Content-Length"},
		MaxAge:           86400,
	})
}

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

		logger := GetLoggerWithTraceContext(ctx, log)
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
			RecordErrorTelemetry(ctx, span, finalErr)
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

		// ponytail: binary bodies (e.g. multipart file uploads) aren't valid UTF-8 and
		// crash the OTLP gRPC log exporter's protobuf string marshaling, so skip them.
		reqBody := "<binary omitted>"
		if isTextContentType(c.Get("Content-Type")) {
			reqBody = string(c.Request().Body())
		}
		respBody := "<binary omitted>"
		if isTextContentType(string(c.Response().Header.ContentType())) {
			respBody = string(c.Response().Body())
		}

		logger.Debug("Request/response body",
			zap.String("http.request_id", requestID),
			zap.String("http.request.body", reqBody),
			zap.String("http.response.body", respBody),
		)

		span.End()
		propagator.Inject(ctx, propagation.HeaderCarrier(c.GetRespHeaders()))

		return err
	}
}

func isTextContentType(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.HasPrefix(ct, "text/") ||
		strings.Contains(ct, "json") ||
		strings.Contains(ct, "xml") ||
		strings.HasPrefix(ct, "application/x-www-form-urlencoded")
}

func WebSocketUpgradeOnly(ctx fiber.Ctx) error {
	if websocket.IsWebSocketUpgrade(ctx) {
		return ctx.Next()
	}
	return fiber.ErrUpgradeRequired
}

func Recovery(log *zap.Logger) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				var errMsg string
				switch v := r.(type) {
				case error:
					errMsg = v.Error()
				case string:
					errMsg = v
				default:
					errMsg = fmt.Sprintf("%v", v)
				}

				stack := debug.Stack()
				panicSource := parsePanicSource(stack)

				log.WithOptions(zap.WithCaller(false)).Error("panic occurred and recovered",
					zap.String("caller", panicSource),
					zap.String("error", errMsg),
				)

				_ = ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fiber.Map{
						"code":    ERR_INTERNAL_SERVER_ERROR_CODE,
						"message": ERR_INTENRAL_SERVER_ERROR_MESSAGE,
						"param":   "",
					},
				})
			}
		}()

		return ctx.Next()
	}
}

func parsePanicSource(stack []byte) string {
	lines := strings.Split(string(stack), "\n")

	panicIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "panic(") {
			panicIdx = i
			break
		}
	}

	if panicIdx == -1 {
		return "unknown"
	}

	for i := panicIdx + 1; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "\t") {
			continue
		}
		fileLine := strings.TrimSpace(lines[i])
		if idx := strings.LastIndex(fileLine, " +0x"); idx != -1 {
			fileLine = fileLine[:idx]
		}
		short := shortPath(fileLine)
		if strings.HasPrefix(short, "runtime/") {
			continue
		}
		return short
	}

	return "unknown"
}

func shortPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return path
}

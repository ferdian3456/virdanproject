package middleware

import (
	"strings"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/usecase"
	"github.com/ferdian3456/virdanproject/internal/util"
	"github.com/gofiber/fiber/v3"
	"github.com/knadh/koanf/v2"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

type AuthMiddleware struct {
	Config      *koanf.Koanf
	Log         *zap.Logger
	UserUsecase *usecase.UserUsecase
}

func NewAuthMiddleware(config *koanf.Koanf, log *zap.Logger, userUsecase *usecase.UserUsecase) *AuthMiddleware {
	return &AuthMiddleware{
		Config:      config,
		Log:         log,
		UserUsecase: userUsecase,
	}
}

// WsProtectedRoute reads the token from the ?token= query parameter instead
// of the Authorization header. Proxies (nginx/CF) often strip hop-by-hop
// headers during WebSocket upgrade, making header-based auth unreliable for WS.
func (middleware *AuthMiddleware) WsProtectedRoute() fiber.Handler {
	return func(ctx fiber.Ctx) error {
		ctxContext := ctx.Context()
		serviceName := middleware.Config.String("OTEL_SERVICE_NAME")
		ctxContext, span := otel.Tracer(serviceName+"-middleware").Start(ctxContext, "middleware.WsProtectedRoute")
		ctx.SetContext(ctxContext)
		var err error

		defer func() {
			if err != nil {
				util.RecordErrorTelemetry(ctxContext, span, err)
			}
			span.End()
		}()

		tokenString := ctx.Query("token")
		if tokenString == "" {
			err = &model.UnauthorizedError{
				Code:    constant.ERR_UNAUTHORIZED_ERROR,
				Message: "Missing token query parameter",
				Param:   "token",
			}
			return util.SendError(ctx, err)
		}

		var userId string
		tokenString, userId, err = util.ValidateAccessToken(tokenString, middleware.Config.String("JWT_SECRET_KEY"))
		if err != nil {
			return util.SendError(ctx, err)
		}

		err = middleware.UserUsecase.GetAccessToken(ctx, userId, tokenString)
		if err != nil {
			return util.SendError(ctx, err)
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
				util.RecordErrorTelemetry(ctxContext, span, err)
			}
			span.End()
		}()

		authHeader := ctx.Get("Authorization")
		if authHeader == "" {
			err = &model.UnauthorizedError{
				Code:    constant.ERR_UNAUTHORIZED_ERROR,
				Message: "Authorization header is missing",
				Param:   "authorization",
			}
			return util.SendError(ctx, err)
		}

		if !strings.HasPrefix(authHeader, util.BearerPrefix) {
			err = &model.UnauthorizedError{
				Code:    constant.ERR_UNAUTHORIZED_ERROR,
				Message: "Invalid authorization scheme",
				Param:   "authorization",
			}
			return util.SendError(ctx, err)
		}

		tokenString := strings.TrimPrefix(authHeader, util.BearerPrefix)

		var userId string
		tokenString, userId, err = util.ValidateAccessToken(tokenString, middleware.Config.String("JWT_SECRET_KEY"))
		if err != nil {
			return util.SendError(ctx, err)
		}

		err = middleware.UserUsecase.GetAccessToken(ctx, userId, tokenString)
		if err != nil {
			return util.SendError(ctx, err)
		}

		ctx.Locals("userId", userId)

		middleware.Log.Debug("auth success", zap.String("userId", userId))

		err = ctx.Next()
		return err
	}
}

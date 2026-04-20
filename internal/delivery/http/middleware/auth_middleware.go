package middleware

import (
	"github.com/ferdian3456/virdanproject/internal/usecase"
	"github.com/ferdian3456/virdanproject/internal/util"

	"github.com/gofiber/fiber/v3"
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
)

type AuthMiddleware struct {
	App         *fiber.App
	Log         *zap.Logger
	Config      *koanf.Koanf
	UserUsecase *usecase.UserUsecase
}

func NewAuthMiddleware(app *fiber.App, zap *zap.Logger, koanf *koanf.Koanf, userUsecase *usecase.UserUsecase) *AuthMiddleware {
	return &AuthMiddleware{
		App:         app,
		Log:         zap,
		Config:      koanf,
		UserUsecase: userUsecase,
	}
}

func (middleware *AuthMiddleware) ProtectedRoute() fiber.Handler {
	return func(ctx fiber.Ctx) error {

		accessToken := ctx.Get("Authorization")
		tokenString, userId, err := util.ValidateAccessToken(accessToken, middleware.Log, middleware.Config.String("JWT_SECRET_KEY"))
		if err != nil {
			return util.SendError(ctx, err)
		}

		err = middleware.UserUsecase.GetAccessToken(ctx, userId, tokenString)
		if err != nil {
			return util.SendError(ctx, err)
		}

		ctx.Locals("userId", userId)

		middleware.Log.Debug("middleware here", zap.String("userId", userId.String()))

		return ctx.Next()
	}
}

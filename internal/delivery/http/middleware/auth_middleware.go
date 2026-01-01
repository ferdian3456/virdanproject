package middleware

import (
	"errors"

	"github.com/ferdian3456/virdanproject/internal/model"
	"github.com/ferdian3456/virdanproject/internal/usecase"
	"github.com/ferdian3456/virdanproject/internal/util"

	"github.com/gofiber/fiber/v2"
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
	return func(ctx *fiber.Ctx) error {
		var validationErr *model.ValidationError

		accessToken := ctx.Get("Authorization")
		tokenString, userId, err := util.ValidateAccessToken(accessToken, middleware.Log, middleware.Config.String("JWT_SECRET_KEY"))
		if err != nil {
			if errors.As(err, &validationErr) {
				return util.SendErrorResponseNotFound(ctx, err)
			}

			return util.SendErrorResponseInternalServer(ctx, middleware.Log, err)
		}

		err = middleware.UserUsecase.GetAccessToken(ctx, userId, tokenString)
		if err != nil {
			if errors.As(err, &validationErr) {
				return util.SendErrorResponseNotFound(ctx, err)
			}

			return util.SendErrorResponseInternalServer(ctx, middleware.Log, err)
		}

		ctx.Locals("userId", userId)

		middleware.Log.Debug("middleware here", zap.String("userId", userId.String()))

		return ctx.Next()
	}
}

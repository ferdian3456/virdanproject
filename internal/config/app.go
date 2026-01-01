package config

import (
	http "github.com/ferdian3456/virdanproject/internal/delivery/http"
	"github.com/ferdian3456/virdanproject/internal/delivery/http/middleware"
	"github.com/ferdian3456/virdanproject/internal/delivery/http/route"
	"github.com/ferdian3456/virdanproject/internal/repository"
	"github.com/ferdian3456/virdanproject/internal/usecase"
	"github.com/minio/minio-go/v7"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type ServerConfig struct {
	Router  *fiber.App
	DB      *pgxpool.Pool
	DBCache *redis.Client
	Log     *zap.Logger
	Config  *koanf.Koanf
	MinIO   *minio.Client
}

func Server(config *ServerConfig) {
	serverRepository := repository.NewServerRepository(config.Log, config.DB, config.DBCache)
	serverUsecase := usecase.NewServerUsecase(serverRepository, config.DB, config.Log, config.Config)
	serverController := http.NewServerController(serverUsecase, config.Log, config.Config)

	userRepository := repository.NewUserRepository(config.Log, config.DB, config.DBCache, config.MinIO)
	userUsecase := usecase.NewUserUsecase(userRepository, serverRepository, config.DB, config.Log, config.Config)
	userController := http.NewUserController(userUsecase, config.Log, config.Config)

	authMiddleware := middleware.NewAuthMiddleware(config.Router, config.Log, config.Config, userUsecase)

	routeConfig := route.RouteConfig{
		App:              config.Router,
		UserController:   userController,
		ServerController: serverController,
		AuthMiddleware:   authMiddleware,
	}

	routeConfig.SetupRoute()
}

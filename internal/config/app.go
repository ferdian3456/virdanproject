package config

import (
	"firebase.google.com/go/v4/messaging"
	http "github.com/ferdian3456/virdanproject/internal/delivery/http"
	"github.com/ferdian3456/virdanproject/internal/delivery/http/middleware"
	"github.com/ferdian3456/virdanproject/internal/delivery/http/route"
	"github.com/ferdian3456/virdanproject/internal/repository"
	"github.com/ferdian3456/virdanproject/internal/usecase"
	"github.com/ferdian3456/virdanproject/internal/ws"
	xenditpkg "github.com/ferdian3456/virdanproject/internal/xendit"
	"github.com/minio/minio-go/v7"

	"github.com/gofiber/fiber/v3"
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
	FCM     *messaging.Client
}

func Server(config *ServerConfig) {
	// Hub has no dependencies — created first so Login/Logout can close WS sessions.
	hub := ws.NewHub()

	serverRepository := repository.NewServerRepository(config.Log, config.Config, config.DB, config.DBCache, config.MinIO)
	profileRepository := repository.NewProfileRepository(config.Log, config.Config, config.DB, config.MinIO)
	serverUsecase := usecase.NewServerUsecase(serverRepository, profileRepository, config.DB, config.Log, config.Config)
	serverController := http.NewServerController(serverUsecase, config.Log, config.Config)

	userRepository := repository.NewUserRepository(config.Log, config.Config, config.DB, config.DBCache, config.MinIO)
	userUsecase := usecase.NewUserUsecase(userRepository, serverRepository, config.DB, config.Log, config.Config, hub)
	userController := http.NewUserController(userUsecase, config.Log, config.Config)

	// NotificationUsecase is built before PostUsecase because PostUsecase injects it (notif triggers).
	notificationRepository := repository.NewNotificationRepository(config.Log, config.Config, config.DB)
	notificationUsecase := usecase.NewNotificationUsecase(notificationRepository, serverRepository, config.FCM, config.DB, config.Log, config.Config)
	notificationController := http.NewNotificationController(notificationUsecase, config.Log, config.Config)

	serverPlusRepository := repository.NewServerPlusRepository(config.Log, config.Config, config.DB)
	xenditClient := xenditpkg.NewClient(config.Config, config.Log)
	serverPlusUsecase := usecase.NewServerPlusUsecase(serverPlusRepository, serverRepository, xenditClient, config.DB, config.Log, config.Config)
	serverPlusController := http.NewServerPlusController(serverPlusUsecase, config.Log, config.Config)

	postRepository := repository.NewPostRepository(config.Log, config.Config, config.DB, config.DBCache, config.MinIO)
	postUsecase := usecase.NewPostUsecase(postRepository, serverRepository, profileRepository, serverPlusRepository, notificationUsecase, config.DB, config.Log, config.Config)
	postController := http.NewPostController(postUsecase, config.Log, config.Config)

	profileUsecase := usecase.NewProfileUsecase(profileRepository, serverRepository, config.DB, config.Log, config.Config)
	profileController := http.NewProfileController(profileUsecase, config.Log, config.Config)

	chatRepository := repository.NewChatRepository(config.Log, config.Config, config.DB)
	broker := ws.NewInProcessBroker(hub)
	chatUsecase := usecase.NewChatUsecase(config.Log, config.Config, config.DB, chatRepository, serverRepository, notificationUsecase, broker, hub)
	chatController := http.NewChatController(config.Log, config.Config, chatUsecase, hub)

	authMiddleware := middleware.NewAuthMiddleware(config.Config, config.Log, userUsecase)

	routeConfig := route.RouteConfig{
		App:                    config.Router,
		UserController:         userController,
		ServerController:       serverController,
		PostController:         postController,
		ProfileController:      profileController,
		NotificationController: notificationController,
		ChatController:         chatController,
		ServerPlusController:   serverPlusController,
		AuthMiddleware:         authMiddleware,
		DB:                     config.DB,
		DBCache:                config.DBCache,
		MinIO:                  config.MinIO,
	}

	routeConfig.SetupRoute()
}

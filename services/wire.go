package services

import (
	firebaseMessaging "firebase.google.com/go/v4/messaging"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/ferdian3456/virdanproject/services/auth"
	"github.com/ferdian3456/virdanproject/services/chat"
	"github.com/ferdian3456/virdanproject/services/notification"
	"github.com/ferdian3456/virdanproject/services/payment"
	"github.com/ferdian3456/virdanproject/services/post"
	"github.com/ferdian3456/virdanproject/services/server"
	"github.com/ferdian3456/virdanproject/services/user"
	"github.com/ferdian3456/virdanproject/shared"
)

type Deps struct {
	DB     *pgxpool.Pool
	Redis  *redis.Client
	MinIO  *minio.Client
	FCM    *firebaseMessaging.Client
	Config *koanf.Koanf
	Log    *zap.Logger
}

type Registry struct {
	Auth         *auth.Controller
	User         *user.Controller
	Server       *server.Controller
	Post         *post.Controller
	Notification *notification.Controller
	Chat         *chat.Controller
	Payment      *payment.Controller

	AuthMiddleware *shared.AuthMiddleware
}

func Wire(d Deps) *Registry {
	hub := shared.NewWsHub()

	serverRepo := server.NewRepository(d.Log, d.Config, d.DB, d.Redis, d.MinIO)
	serverSvc := server.NewService(serverRepo, d.DB, d.Log, d.Config)
	serverCtrl := server.NewController(serverSvc, d.Log, d.Config)

	postRepo := post.NewRepository(d.Log, d.Config, d.DB, d.Redis, d.MinIO)

	userRepo := user.NewRepository(d.Log, d.Config, d.DB, d.Redis, d.MinIO)
	userSvc := user.NewService(userRepo, serverRepo, postRepo, d.DB, d.Log, d.Config, hub)
	userCtrl := user.NewController(userSvc, d.Log, d.Config)

	authRepo := auth.NewRepository(d.Log, d.Config, d.DB, d.Redis)
	authSvc := auth.NewService(authRepo, d.DB, d.Log, d.Config, hub)
	authCtrl := auth.NewController(authSvc, d.Log, d.Config)

	authMiddleware := shared.NewAuthMiddleware(d.Config, d.Log, authSvc)

	notificationRepo := notification.NewRepository(d.Log, d.Config, d.DB)
	notificationSvc := notification.NewService(notificationRepo, serverRepo, d.FCM, d.DB, d.Log, d.Config)
	notificationCtrl := notification.NewController(notificationSvc, d.Log, d.Config)

	xenditClient := shared.NewXenditClient(d.Config, d.Log)
	paymentRepo := payment.NewRepository(d.Log, d.Config, d.DB)
	paymentSvc := payment.NewService(paymentRepo, serverRepo, xenditClient, d.DB, d.Log, d.Config)
	paymentCtrl := payment.NewController(paymentSvc, d.Log, d.Config)

	postSvc := post.NewService(postRepo, serverRepo, paymentRepo, notificationSvc, d.DB, d.Log, d.Config)
	postCtrl := post.NewController(postSvc, d.Log, d.Config)

	chatRepo := chat.NewRepository(d.Log, d.Config, d.DB)
	broker := shared.NewInProcessWsBroker(hub)
	chatSvc := chat.NewService(d.Log, d.Config, d.DB, chatRepo, serverRepo, notificationSvc, broker, hub)
	chatCtrl := chat.NewController(d.Log, d.Config, chatSvc, hub)

	return &Registry{
		Auth:           authCtrl,
		User:           userCtrl,
		Server:         serverCtrl,
		Post:           postCtrl,
		Notification:   notificationCtrl,
		Chat:           chatCtrl,
		Payment:        paymentCtrl,
		AuthMiddleware: authMiddleware,
	}
}

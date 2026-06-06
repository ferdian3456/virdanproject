package route

import (
	"github.com/ferdian3456/virdanproject/internal/delivery/http"
	"github.com/ferdian3456/virdanproject/internal/delivery/http/middleware"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
)

type RouteConfig struct {
	App                    *fiber.App
	AuthMiddleware         *middleware.AuthMiddleware
	UserController         *http.UserController
	ServerController       *http.ServerController
	PostController         *http.PostController
	ProfileController      *http.ProfileController
	NotificationController *http.NotificationController
	ChatController         *http.ChatController
	DB                     *pgxpool.Pool
	DBCache                *redis.Client
	MinIO                  *minio.Client
}

func (c *RouteConfig) SetupRoute() {
	api := c.App.Group("/api")

	api.Get("/health", func(fiberCtx fiber.Ctx) error {
		ctx := fiberCtx.Context()

		health := fiber.Map{
			"status": "ok",
			"checks": fiber.Map{},
		}

		// Postgres Check
		if err := c.DB.Ping(ctx); err != nil {
			health["status"] = "error"
			health["checks"].(fiber.Map)["postgres"] = "down: " + err.Error()
		} else {
			health["checks"].(fiber.Map)["postgres"] = "up"
		}

		// Redis Check
		if err := c.DBCache.Ping(ctx).Err(); err != nil {
			health["status"] = "error"
			health["checks"].(fiber.Map)["redis"] = "down: " + err.Error()
		} else {
			health["checks"].(fiber.Map)["redis"] = "up"
		}

		// MinIO Check
		if _, err := c.MinIO.ListBuckets(ctx); err != nil {
			health["status"] = "error"
			health["checks"].(fiber.Map)["minio"] = "down: " + err.Error()
		} else {
			health["checks"].(fiber.Map)["minio"] = "up"
		}

		if health["status"] == "error" {
			return fiberCtx.Status(fiber.StatusServiceUnavailable).JSON(health)
		}

		return fiberCtx.JSON(health)
	})

	authGroup := api.Group("/auth")
	authGroup.Post("/signup/start", c.UserController.StartSignup)
	authGroup.Post("/signup/otp", c.UserController.VerifyOtp)
	authGroup.Post("/signup/password", c.UserController.VerifyPassword)
	authGroup.Get("/signup/:sessionId/status", c.UserController.GetSignupStatus)
	authGroup.Post("/signup/resend-otp", c.UserController.ResendOtp)
	//authGroup.Post("/register", c.UserController.Register)
	authGroup.Post("/login", c.UserController.Login)
	authGroup.Post("/refresh", c.UserController.RefreshToken)
	//authGroup.Post("/forgot-password", c.UserController.ForgotPassword)
	//authGroup.Post("/reset-password", c.UserController.ResetPassword)

	userGroup := api.Group("/users", c.AuthMiddleware.ProtectedRoute())
	userGroup.Get("/me", c.UserController.GetUserInfo)
	userGroup.Delete("/me", c.UserController.DeleteAccount)
	userGroup.Post("/logout", c.UserController.Logout)
	userGroup.Post("/password/verify", c.UserController.VerifyCurrentPassword)
	userGroup.Put("/password", c.UserController.ChangePassword)
	userGroup.Post("/email/change/request", c.UserController.RequestEmailChange)
	userGroup.Post("/email/change/confirm", c.UserController.ConfirmEmailChange)
	userGroup.Put("/me/notification-preferences", c.UserController.UpdateNotificationPreferences)

	// Public server routes (NO AUTH) - must be defined BEFORE protected routes
	// to ensure Fiber matches these routes first
	serverPublicGroup := api.Group("/servers")
	serverPublicGroup.Get("/invites/:inviteCode", c.ServerController.GetServerInfoForInvite)

	// Protected server routes (require auth)
	serverGroup := api.Group("/servers", c.AuthMiddleware.ProtectedRoute())

	// Post routes (must be FIRST to avoid conflicts with /:id routes)
	serverGroup.Post("/:serverId/posts", c.PostController.CreatePost)
	serverGroup.Put("/:serverId/posts/:postId", c.PostController.UpdatePost)
	serverGroup.Delete("/:serverId/posts/:postId", c.PostController.DeletePost)
	serverGroup.Get("/:serverId/posts", c.PostController.GetServerPosts)
	serverGroup.Get("/:serverId/posts/search", c.PostController.SearchServerPosts)

	// Server Categories
	serverGroup.Get("/categories", c.ServerController.GetCategoryServer)

	// Invite and join routes
	serverGroup.Post("/:serverId/invites", c.ServerController.CreateInviteLink)
	serverGroup.Post("/join", c.ServerController.JoinServerFromInvite)
	serverGroup.Post("/create", c.ServerController.CreateServer)
	serverGroup.Get("/", c.ServerController.GetDiscoveryServer)
	serverGroup.Post("/:serverId/join", c.ServerController.JoinServer)
	serverGroup.Get("/me", c.ServerController.GetUserServer)

	// Server management routes with /:id parameter
	serverGroup.Get("/:id", c.ServerController.GetServerById)
	serverGroup.Put("/:id/name", c.ServerController.UpdateServerName)
	serverGroup.Put("/:id/shortName", c.ServerController.UpdateServerShortName)
	serverGroup.Put("/:id/category", c.ServerController.UpdateServerCategory)
	serverGroup.Put("/:id/avatar", c.ServerController.UpdateServerAvatar)
	serverGroup.Put("/:id/banner", c.ServerController.UpdateServerBanner)
	serverGroup.Put("/:id/description", c.ServerController.UpdateServerDescription)
	serverGroup.Put("/:id/settings", c.ServerController.UpdateServerSettings)
	serverGroup.Delete("/:id", c.ServerController.DeleteServer)
	serverGroup.Delete("/:serverId/membership", c.ServerController.LeaveServer)
	serverGroup.Get("/:serverId/posts/me", c.PostController.GetServerPostForMe)
	serverGroup.Get("/:serverId/posts/saved", c.PostController.GetSavedPosts)
	serverGroup.Get("/:serverId/members/:userId/posts", c.PostController.GetServerPostsByUserId)

	profileGroup := api.Group("", c.AuthMiddleware.ProtectedRoute())
	profileGroup.Get("/profiles/history", c.ProfileController.GetProfileHistory)
	profileGroup.Get("/servers/:serverId/profile/me", c.ProfileController.GetServerProfileMe)
	profileGroup.Get("/servers/:serverId/members/:userId/profile", c.ProfileController.GetServerProfileByUserId)
	profileGroup.Put("/servers/:serverId/profile", c.ProfileController.UpdateServerProfile)

	postGroup := api.Group("/posts", c.AuthMiddleware.ProtectedRoute())
	postGroup.Get("/:postId", c.PostController.GetPost)
	// postGroup.Delete("/:postId", c.PostController.DeletePost)
	postGroup.Post("/:postId/likes", c.PostController.LikePost)
	postGroup.Delete("/:postId/likes", c.PostController.UnlikePost)
	postGroup.Post("/:postId/saves", c.PostController.SavePost)
	postGroup.Delete("/:postId/saves", c.PostController.UnsavePost)
	postGroup.Post("/:postId/comments", c.PostController.CreateComment)
	postGroup.Get("/:postId/comments", c.PostController.GetComments)
	postGroup.Delete("/:postId/comments/:commentId", c.PostController.DeleteComment)

	deviceGroup := api.Group("/devices", c.AuthMiddleware.ProtectedRoute())
	deviceGroup.Post("/", c.NotificationController.RegisterDevice)
	deviceGroup.Delete("/", c.NotificationController.UnregisterDevice)

	notifGroup := api.Group("/notifications", c.AuthMiddleware.ProtectedRoute())
	notifGroup.Post("/test-send", c.NotificationController.TestSend)

	// Per-server notification feed/badge/read — nested under the server, member-guarded.
	// unread-count BEFORE the feed route so Fiber matches the literal segment first.
	serverGroup.Get("/:serverId/notifications/unread-count", c.NotificationController.GetUnreadCount)
	serverGroup.Get("/:serverId/notifications", c.NotificationController.GetFeed)
	serverGroup.Post("/:serverId/notifications/:id/read", c.NotificationController.MarkRead)

	serverGroup.Get("/:serverId/members", c.ChatController.ListMembers)
	serverGroup.Get("/:serverId/conversations", c.ChatController.ListConversations)
	serverGroup.Post("/:serverId/conversations", c.ChatController.GetOrCreateConversation)

	convGroup := api.Group("/conversations", c.AuthMiddleware.ProtectedRoute())
	convGroup.Post("/:conversationId/messages", c.ChatController.SendMessage)
	convGroup.Get("/:conversationId/messages", c.ChatController.ListMessages)
	convGroup.Post("/:conversationId/read", c.ChatController.MarkRead)

	wsGroup := api.Group("/ws", c.AuthMiddleware.WsProtectedRoute())
	wsGroup.Use(middleware.WebSocketUpgradeOnly)
	wsGroup.Get("/", websocket.New(c.ChatController.HandleWS))
}

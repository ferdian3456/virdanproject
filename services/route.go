package services

import (
	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"

	"github.com/ferdian3456/virdanproject/shared"
)

func RegisterRoutes(app fiber.Router, r *Registry, d Deps) {
	api := app.Group("/api")

	api.Get("/health", func(fiberCtx fiber.Ctx) error {
		ctx := fiberCtx.Context()

		health := fiber.Map{
			"status": "ok",
			"checks": fiber.Map{},
		}

		if err := d.DB.Ping(ctx); err != nil {
			health["status"] = "error"
			health["checks"].(fiber.Map)["postgres"] = "down: " + err.Error()
		} else {
			health["checks"].(fiber.Map)["postgres"] = "up"
		}

		if err := d.Redis.Ping(ctx).Err(); err != nil {
			health["status"] = "error"
			health["checks"].(fiber.Map)["redis"] = "down: " + err.Error()
		} else {
			health["checks"].(fiber.Map)["redis"] = "up"
		}

		if _, err := d.MinIO.ListBuckets(ctx); err != nil {
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
	authGroup.Post("/signup/start", r.Auth.StartSignup)
	authGroup.Post("/signup/otp", r.Auth.VerifyOtp)
	authGroup.Post("/signup/password", r.Auth.VerifyPassword)
	authGroup.Get("/signup/:sessionId/status", r.Auth.GetSignupStatus)
	authGroup.Post("/signup/resend-otp", r.Auth.ResendOtp)
	authGroup.Post("/login", r.Auth.Login)
	authGroup.Post("/refresh", r.Auth.RefreshToken)
	authGroup.Post("/logout", r.AuthMiddleware.ProtectedRoute(), r.Auth.Logout)

	userGroup := api.Group("/users", r.AuthMiddleware.ProtectedRoute())
	userGroup.Get("/me", r.User.GetUserInfo)
	userGroup.Delete("/me", r.User.DeleteAccount)
	userGroup.Post("/password/verify", r.User.VerifyCurrentPassword)
	userGroup.Put("/password", r.User.ChangePassword)
	userGroup.Post("/email/change/request", r.User.RequestEmailChange)
	userGroup.Post("/email/change/confirm", r.User.ConfirmEmailChange)
	userGroup.Put("/me/notification-preferences", r.User.UpdateNotificationPreferences)

	serverPublicGroup := api.Group("/servers")
	serverPublicGroup.Get("/invites/:inviteCode", r.Server.GetServerInfoForInvite)

	serverGroup := api.Group("/servers", r.AuthMiddleware.ProtectedRoute())

	serverGroup.Post("/:serverId/posts", r.Post.CreatePost)
	serverGroup.Put("/:serverId/posts/:postId", r.Post.UpdatePost)
	serverGroup.Delete("/:serverId/posts/:postId", r.Post.DeletePost)
	serverGroup.Get("/:serverId/posts", r.Post.GetServerPosts)
	serverGroup.Get("/:serverId/posts/search", r.Post.SearchServerPosts)

	serverGroup.Get("/categories", r.Server.GetCategoryServer)

	serverGroup.Post("/:serverId/invites", r.Server.CreateInviteLink)
	serverGroup.Post("/join", r.Server.JoinServerFromInvite)
	serverGroup.Post("/create", r.Server.CreateServer)
	serverGroup.Get("/", r.Server.GetDiscoveryServer)
	serverGroup.Post("/:serverId/join", r.Server.JoinServer)
	serverGroup.Get("/me", r.Server.GetUserServer)

	serverGroup.Get("/:id", r.Server.GetServerById)
	serverGroup.Put("/:id/name", r.Server.UpdateServerName)
	serverGroup.Put("/:id/shortName", r.Server.UpdateServerShortName)
	serverGroup.Put("/:id/category", r.Server.UpdateServerCategory)
	serverGroup.Put("/:id/avatar", r.Server.UpdateServerAvatar)
	serverGroup.Put("/:id/banner", r.Server.UpdateServerBanner)
	serverGroup.Put("/:id/description", r.Server.UpdateServerDescription)
	serverGroup.Put("/:id/settings", r.Server.UpdateServerSettings)
	serverGroup.Delete("/:id", r.Server.DeleteServer)
	serverGroup.Delete("/:serverId/membership", r.Server.LeaveServer)
	serverGroup.Get("/:serverId/posts/me", r.Post.GetServerPostForMe)
	serverGroup.Get("/:serverId/posts/saved", r.Post.GetSavedPosts)
	serverGroup.Get("/:serverId/members/:userId/posts", r.Post.GetServerPostsByUserId)

	profilesGroup := api.Group("/profiles", r.AuthMiddleware.ProtectedRoute())
	profilesGroup.Get("/history", r.Server.GetProfileHistory)

	serverGroup.Get("/:serverId/profile/me", r.Server.GetServerProfileMe)
	serverGroup.Get("/:serverId/members/:userId/profile", r.Server.GetServerProfileByUserId)
	serverGroup.Put("/:serverId/profile", r.Server.UpdateServerProfile)

	postGroup := api.Group("/posts", r.AuthMiddleware.ProtectedRoute())
	postGroup.Get("/:postId", r.Post.GetPost)
	postGroup.Post("/:postId/likes", r.Post.LikePost)
	postGroup.Delete("/:postId/likes", r.Post.UnlikePost)
	postGroup.Post("/:postId/saves", r.Post.SavePost)
	postGroup.Delete("/:postId/saves", r.Post.UnsavePost)
	postGroup.Post("/:postId/comments", r.Post.CreateComment)
	postGroup.Get("/:postId/comments", r.Post.GetComments)
	postGroup.Delete("/:postId/comments/:commentId", r.Post.DeleteComment)

	deviceGroup := api.Group("/devices", r.AuthMiddleware.ProtectedRoute())
	deviceGroup.Post("/", r.Notification.RegisterDevice)
	deviceGroup.Delete("/", r.Notification.UnregisterDevice)

	notifGroup := api.Group("/notifications", r.AuthMiddleware.ProtectedRoute())
	notifGroup.Post("/test-send", r.Notification.TestSend)

	serverGroup.Get("/:serverId/notifications/unread-count", r.Notification.GetUnreadCount)
	serverGroup.Get("/:serverId/notifications", r.Notification.GetFeed)
	serverGroup.Post("/:serverId/notifications/:id/read", r.Notification.MarkRead)

	serverGroup.Get("/:serverId/plus", r.Payment.GetPlusStatus)
	serverGroup.Post("/:serverId/plus/checkout", r.Payment.Checkout)

	meGroup := api.Group("/me", r.AuthMiddleware.ProtectedRoute())
	meGroup.Get("/plus-orders", r.Payment.ListMyOrders)

	webhookGroup := api.Group("/webhooks")
	webhookGroup.Post("/xendit", r.Payment.HandleWebhook)

	serverGroup.Get("/:serverId/members", r.Server.GetServerMembers)
	serverGroup.Get("/:serverId/members/me", r.Server.GetMyRoleInServer)
	serverGroup.Delete("/:serverId/members/:userId", r.Server.KickMember)
	serverGroup.Put("/:serverId/members/:userId/role", r.Server.AssignMemberRole)
	serverGroup.Put("/:serverId/ownership", r.Server.TransferOwnership)

	serverGroup.Get("/:serverId/members/dm", r.Chat.ListMembers)
	serverGroup.Get("/:serverId/conversations", r.Chat.ListConversations)
	serverGroup.Post("/:serverId/conversations", r.Chat.GetOrCreateConversation)

	convGroup := api.Group("/conversations", r.AuthMiddleware.ProtectedRoute())
	convGroup.Post("/:conversationId/messages", r.Chat.SendMessage)
	convGroup.Get("/:conversationId/messages", r.Chat.ListMessages)
	convGroup.Post("/:conversationId/read", r.Chat.MarkRead)

	wsGroup := api.Group("/ws", r.AuthMiddleware.WsProtectedRoute())
	wsGroup.Use(shared.WebSocketUpgradeOnly)
	wsGroup.Get("/", websocket.New(r.Chat.HandleWS))
}

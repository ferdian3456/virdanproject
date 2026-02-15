package route

import (
	"github.com/ferdian3456/virdanproject/internal/delivery/http"
	"github.com/ferdian3456/virdanproject/internal/delivery/http/middleware"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
)

type RouteConfig struct {
	App              *fiber.App
	AuthMiddleware   *middleware.AuthMiddleware
	UserController   *http.UserController
	ServerController *http.ServerController
	PostController   *http.PostController
	DB               *pgxpool.Pool
	DBCache          *redis.Client
	MinIO            *minio.Client
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
	authGroup.Post("/signup/username", c.UserController.VerifyUsername)
	authGroup.Post("/signup/password", c.UserController.VerifyPassword)
	authGroup.Get("/signup/:sessionId/status", c.UserController.GetSignupStatus)
	//authGroup.Post("/register", c.UserController.Register)
	authGroup.Post("/login", c.UserController.Login)
	//authGroup.Post("/refresh", c.UserController.Refresh)
	//authGroup.Post("/forgot-password", c.UserController.ForgotPassword)
	//authGroup.Post("/reset-password", c.UserController.ResetPassword)

	userGroup := api.Group("/users", c.AuthMiddleware.ProtectedRoute())
	userGroup.Get("/me", c.UserController.GetUserInfo)
	userGroup.Post("/logout", c.UserController.Logout)
	// userGroup.Put("/username", c.UserController.UpdateUsername)
	userGroup.Put("/fullname", c.UserController.UpdateFullname)
	userGroup.Put("/bio", c.UserController.UpdateBio)
	userGroup.Put("/avatar", c.UserController.UpdateAvatar)
	//userGroup.Patch("/password", c.UserController.ChangePassword)
	//userGroup.Delete("/account", c.UserController.DeleteAccount)

	serverGroup := api.Group("/servers", c.AuthMiddleware.ProtectedRoute())

	// Post routes (must be FIRST to avoid conflicts with /:id routes)
	serverGroup.Post("/:serverId/posts", c.PostController.CreatePost)
	serverGroup.Put("/:serverId/posts/:postId", c.PostController.UpdatePost)
	serverGroup.Delete("/:serverId/posts/:postId", c.PostController.DeletePost)
	serverGroup.Get("/:serverId/posts", c.PostController.GetServerPosts)

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

	postGroup := api.Group("/posts", c.AuthMiddleware.ProtectedRoute())
	postGroup.Get("/:postId", c.PostController.GetPost)
	// postGroup.Delete("/:postId", c.PostController.DeletePost)
	postGroup.Post("/:postId/likes", c.PostController.LikePost)
	postGroup.Delete("/:postId/likes", c.PostController.UnlikePost)
	postGroup.Post("/:postId/comments", c.PostController.CreateComment)
	postGroup.Get("/:postId/comments", c.PostController.GetComments)
	postGroup.Delete("/:postId/comments/:commentId", c.PostController.DeleteComment)

	serverPublicGroup := api.Group("/servers")
	serverPublicGroup.Get("/invites/:inviteCode", c.ServerController.GetServerInfoForInvite)
}

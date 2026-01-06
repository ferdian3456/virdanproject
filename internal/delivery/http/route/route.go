package route

import (
	"github.com/ferdian3456/virdanproject/internal/delivery/http"
	"github.com/ferdian3456/virdanproject/internal/delivery/http/middleware"

	"github.com/gofiber/fiber/v2"
)

type RouteConfig struct {
	App              *fiber.App
	AuthMiddleware   *middleware.AuthMiddleware
	UserController   *http.UserController
	ServerController *http.ServerController
	PostController   *http.PostController
}

func (c *RouteConfig) SetupRoute() {
	api := c.App.Group("/api")

	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
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
	// userGroup.Put("/fullname", c.UserController.UpdateFullName)
	// userGroup.Put("/bio", c.UserController.UpdateBio)
	//userGroup.Put("/avatar", c.UserController.UpdateAvatar)
	//userGroup.Patch("/password", c.UserController.ChangePassword)
	//userGroup.Delete("/account", c.UserController.DeleteAccount)

	serverGroup := api.Group("/servers", c.AuthMiddleware.ProtectedRoute())
	serverGroup.Post("/:serverId/invites", c.ServerController.CreateInviteLink)
	serverGroup.Post("/join", c.ServerController.JoinServerFromInvite)
	serverGroup.Post("/create", c.ServerController.CreateServer)
	serverGroup.Get("/", c.ServerController.GetDiscoveryServer)
	serverGroup.Post("/:serverId/join", c.ServerController.JoinServer)
	serverGroup.Get("/me", c.ServerController.GetUserServer)
	// serverGroup.Get("/:id", c.ServerController.GetServerById)
	serverGroup.Put("/:id/name", c.ServerController.UpdateServerName)
	serverGroup.Put("/:id/shortName", c.ServerController.UpdateServerShortName)
	serverGroup.Put("/:id/category", c.ServerController.UpdateServerCategory)
	serverGroup.Put("/:id/avatar", c.ServerController.UpdateServerAvatar)
	serverGroup.Put("/:id/banner", c.ServerController.UpdateServerBanner)
	serverGroup.Put("/:id/description", c.ServerController.UpdateServerDescription)
	serverGroup.Put("/:id/settings", c.ServerController.UpdateServerSettings)
	serverGroup.Delete("/:id", c.ServerController.DeleteServer)

	serverPostGroup := serverGroup.Group("/:serverId/posts")
	serverPostGroup.Post("/", c.PostController.CreatePost)
	serverPostGroup.Put("/:postId", c.PostController.UpdatePost)
	serverPostGroup.Delete("/:postId", c.PostController.DeletePost)
	serverPostGroup.Get("/", c.PostController.GetServerPosts)

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

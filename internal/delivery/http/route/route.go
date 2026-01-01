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
	//userGroup.Put("/avatar", c.UserController.UpdateAvatar)
	//userGroup.Patch("/password", c.UserController.ChangePassword)
	//userGroup.Delete("/account", c.UserController.DeleteAccount)

	//serverGroup := api.Group("/servers", c.AuthMiddleware.ProtectedRoute())
	//serverGroup.Get("/me", c.ServerController.GetServer)
	//serverGroup.Post("", c.ServerController.CreateServer())
	//serverGroup.Get("/:id", c.ServerController.GetServerById())
	//serverGroup.Patch("/:id", c.ServerController.UpdateServer())
	//serverGroup.Delete("/:id", c.ServerController.DeleteServer())

}

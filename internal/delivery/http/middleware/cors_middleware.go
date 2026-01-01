package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// SetupCORS configures CORS middleware for the application
func SetupCORS() fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000, http://localhost:8080",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowCredentials: true,
		ExposeHeaders:    "Content-Length",
		MaxAge:           86400, // Pre-flight request can be cached for 1 day
	})
}
package middleware

import (
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3"
)

//CORSMiddleware configures CORS middleware for the application
func CORSMiddleware() fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:8100"},
		AllowMethods:     []string{"GET,POST,PUT,PATCH,DELETE,OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		ExposeHeaders:    []string{"Content-Length"},
		MaxAge:           86400, // Pre-flight request can be cached for 1 day
	})
}

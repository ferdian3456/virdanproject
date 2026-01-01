package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"go.uber.org/zap"
)

// SetupRateLimiter configures rate limiting middleware for the application
func SetupRateLimiter(logger *zap.Logger) fiber.Handler {
	return limiter.New(limiter.Config{
		Next: func(c *fiber.Ctx) bool {
			// Skip rate limiting for health check endpoint
			return c.Path() == "/api/health"
		},
		Max:        100, // Max requests per window
		Expiration: 60,  // Window duration in seconds
		KeyGenerator: func(c *fiber.Ctx) string {
			// Use IP address as the key for rate limiting
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			logger.Warn("Rate limit exceeded", zap.String("ip", c.IP()))
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Rate limit exceeded, please try again later",
			})
		},
	})
}

// SetupAuthRateLimiter configures a stricter rate limiting for authentication endpoints
func SetupAuthRateLimiter(logger *zap.Logger) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        5,   // Max requests per window
		Expiration: 300, // Window duration in seconds (5 minutes)
		KeyGenerator: func(c *fiber.Ctx) string {
			// Use IP address as the key for rate limiting
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			logger.Warn("Auth rate limit exceeded", zap.String("ip", c.IP()))
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many authentication attempts, please try again later",
			})
		},
	})
}
package middleware

import (
<<<<<<< HEAD
	"log"

=======
>>>>>>> refs/remotes/origin/main
	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
)

func WebSocketUpgradeOnly(ctx fiber.Ctx) error {
<<<<<<< HEAD
	log.Printf("[WS-DEBUG] method=%s path=%s Connection=%q Upgrade=%q",
		ctx.Method(), ctx.Path(),
		ctx.Get("Connection"),
		ctx.Get("Upgrade"),
	)
=======
>>>>>>> refs/remotes/origin/main
	if websocket.IsWebSocketUpgrade(ctx) {
		return ctx.Next()
	}
	return fiber.ErrUpgradeRequired
}

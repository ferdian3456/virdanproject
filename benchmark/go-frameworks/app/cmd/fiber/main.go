package main

import (
	"flag"
	"log"

	"github.com/ferdian3456/virdanproject/benchmark/go-frameworks/app/internal/device"
	"github.com/gofiber/fiber/v3"
)

func sendJSON(c fiber.Ctx, status int, b []byte) error {
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.Status(status).Send(b)
}

func main() {
	addr := flag.String("addr", ":8085", "listen address")
	flag.Parse()

	app := fiber.New()
	app.Get("/api/devices", func(c fiber.Ctx) error {
		b, err := device.List()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
		}
		return sendJSON(c, fiber.StatusOK, b)
	})
	app.Post("/api/devices", func(c fiber.Ctx) error {
		b, err := device.Create(c.Body())
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString(err.Error())
		}
		return sendJSON(c, fiber.StatusCreated, b)
	})
	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.SendString("OK")
	})

	log.Printf("fiber listening on %s", *addr)
	log.Fatal(app.Listen(*addr, fiber.ListenConfig{DisableStartupMessage: true}))
}

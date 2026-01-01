package config

import (
	"time"

	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v2"
)

func NewFiber() *fiber.App {
	app := fiber.New(fiber.Config{
		//Prefork:               true,
		Prefork:               false,
		AppName:               "",
		BodyLimit:             4 * 1024 * 1024, // 4MB
		ReadBufferSize:        4096,
		WriteBufferSize:       4096,
		Concurrency:           256 * 1024,
		IdleTimeout:           30 * time.Second,
		ReadTimeout:           10 * time.Second,
		WriteTimeout:          10 * time.Second,
		DisableKeepalive:      false,
		DisableStartupMessage: true,
		ReduceMemoryUsage:     true,
		JSONEncoder:           sonic.Marshal,
		JSONDecoder:           sonic.Unmarshal,
	})

	return app
}

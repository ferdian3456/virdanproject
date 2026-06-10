package config

import (
	"time"

	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v3"
)

func NewFiber() *fiber.App {
	app := fiber.New(fiber.Config{
		//Prefork:               true,
		AppName:           "",
		BodyLimit:         105 * 1024 * 1024, // 105MB (100MB video + form overhead)
		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
		Concurrency:       256 * 1024,
		IdleTimeout:       30 * time.Second,
		ReadTimeout:       120 * time.Second,
		WriteTimeout:      60 * time.Second,
		DisableKeepalive:  false,
		ReduceMemoryUsage: true,
		JSONEncoder:       sonic.Marshal,
		JSONDecoder:       sonic.Unmarshal,
	})

	return app
}

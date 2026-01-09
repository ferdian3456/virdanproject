package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ferdian3456/virdanproject/internal/config"
	middleware "github.com/ferdian3456/virdanproject/internal/exception"
	"github.com/gofiber/fiber/v2/middleware/compress"
	zapLog "go.uber.org/zap"
)

func main() {
	time.Local = time.UTC
	// Flush zap buffered log first then cancel the context for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fiber := config.NewFiber()
	zap := config.NewZap()
	koanf := config.NewKoanf(zap)
	rds := config.NewRedisClient(koanf, zap)
	postgresql := config.NewPostgresqlPool(koanf, zap)
	minio := config.NewMinIO(koanf, zap)

	// Custom recovery middleware to handle panics with JSON response
	fiber.Use(middleware.Recovery(zap))

	// 5. Compression middleware (should be before logging)
	fiber.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))

	config.Server(&config.ServerConfig{
		Router:  fiber,
		DB:      postgresql,
		DBCache: rds,
		Log:     zap,
		Config:  koanf,
		MinIO:   minio,
	})

	GO_SERVER_PORT := koanf.String("GO_SERVER")

	zap.Info("Server is running on: " + GO_SERVER_PORT)

	var err error
	go func() {
		err = fiber.Listen(GO_SERVER_PORT)
		if err != nil {
			zap.Fatal("error starting server", zapLog.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	zap.Info("got one of stop signals")

	err = fiber.ShutdownWithContext(ctx)
	if err != nil {
		zap.Warn("timeout, forced kill!", zapLog.Error(err))
		_ = zap.Sync()
		os.Exit(1)
	}

	zap.Info("server has shut down gracefully")
	_ = zap.Sync()
}

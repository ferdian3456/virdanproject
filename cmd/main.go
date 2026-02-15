package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ferdian3456/virdanproject/internal/config"
	"github.com/ferdian3456/virdanproject/internal/delivery/http/middleware"
	"github.com/ferdian3456/virdanproject/internal/exception"
	gofiber "github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	zapLog "go.uber.org/zap"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "run healthcheck")
	versionFlag := flag.Bool("version", false, "print version")
	flag.Parse()

	if *healthcheck {
		os.Exit(0) // Minimal healthcheck for now
	}

	if *versionFlag {
		fmt.Printf("Version: %s\nCommit: %s\nBuildTime: %s\n", Version, Commit, BuildTime)
		os.Exit(0)
	}

	time.Local = time.UTC

	// Init context — for bootstrapping resources (DB, Redis, MinIO, OTel)
	initCtx, initCancel := context.WithTimeout(context.Background(), 30*time.Second)

	fiber := config.NewFiber()
	bootstrapZap := config.NewBootstrapZap()
	koanf := config.NewKoanf(bootstrapZap)
	zap := config.NewZap(initCtx, koanf, bootstrapZap)
	_ = bootstrapZap.Sync()

	// OTel providers — returned for graceful shutdown
	meterProvider := config.NewMetrics(initCtx, koanf, zap)
	tracerProvider := config.NewTracer(initCtx, koanf, zap)

	rds := config.NewRedisClient(initCtx, koanf, zap)
	postgresql := config.NewPostgresqlPool(initCtx, koanf, zap)
	minio := config.NewMinIO(initCtx, koanf, zap)

	initCancel() // init done, release init context

	// Custom recovery middleware to handle panics with JSON response
	fiber.Use(exception.Recovery(zap))

	// Compression middleware (should be before logging)
	fiber.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))

	// RequestID middleware
	fiber.Use(requestid.New())

	fiber.Use(middleware.TracingMiddleware(koanf))

	fiber.Use(middleware.LoggingMiddleware(koanf, meterProvider, zap))

	config.Server(&config.ServerConfig{
		Router:  fiber,
		DB:      postgresql,
		DBCache: rds,
		Log:     zap,
		Config:  koanf,
		MinIO:   minio,
	})

	GO_SERVER_PORT := koanf.String("GO_SERVER")

	zap.Info("server is running on: " + GO_SERVER_PORT)

	var err error
	go func() {
		err = fiber.Listen(GO_SERVER_PORT, gofiber.ListenConfig{
			DisableStartupMessage: true,
			EnablePrefork:         false,
		})
		if err != nil {
			zap.Fatal("error starting server", zapLog.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	zap.Info("got one of stop signals")

	// Shutdown context — for graceful shutdown only
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Shutdown OTel providers first — flush all telemetry data before app exits
	if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
		zap.Error("failed to shutdown tracer provider", zapLog.Error(err))
	}
	if err := meterProvider.Shutdown(shutdownCtx); err != nil {
		zap.Error("failed to shutdown meter provider", zapLog.Error(err))
	}

	err = fiber.ShutdownWithContext(shutdownCtx)
	if err != nil {
		zap.Warn("timeout, forced kill!", zapLog.Error(err))
		_ = zap.Sync()
		os.Exit(1)
	}

	zap.Info("server has shut down gracefully")
	_ = zap.Sync()
}

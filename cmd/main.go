package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ferdian3456/virdanproject/services"
	"github.com/ferdian3456/virdanproject/shared"
	gofiber "github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	zapLog "go.uber.org/zap"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "run healthcheck")
	flag.Parse()

	if *healthcheck {
		os.Exit(0)
	}

	time.Local = time.UTC

	initCtx, initCancel := context.WithTimeout(context.Background(), 30*time.Second)

	fiber := shared.NewFiber()
	fiber.Use(shared.CORSMiddleware())
	bootstrapZap := shared.NewBootstrapZap()
	koanf := shared.NewKoanf(bootstrapZap)

	loggerProvider := shared.NewOtelLoggerProvider(initCtx, koanf, bootstrapZap)
	zap := shared.NewZap(koanf, loggerProvider)
	_ = bootstrapZap.Sync()

	meterProvider := shared.NewOtelMetricProvider(initCtx, koanf, zap)
	tracerProvider := shared.NewOtelTracerProvider(initCtx, koanf, zap)

	rds := shared.NewRedisClient(initCtx, koanf, zap)
	postgresql := shared.NewPostgresqlPool(initCtx, koanf, zap)
	minio := shared.NewMinIO(initCtx, koanf, zap)
	fcm := shared.NewFCMClient(initCtx, koanf, zap)

	initCancel()

	fiber.Use(shared.Recovery(zap))

	fiber.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))

	fiber.Use(shared.ObservabilityMiddleware(koanf, meterProvider, zap))

	deps := services.Deps{
		DB:     postgresql,
		Redis:  rds,
		MinIO:  minio,
		FCM:    fcm,
		Config: koanf,
		Log:    zap,
	}
	registry := services.Wire(deps)
	services.RegisterRoutes(fiber, registry, deps)

	GO_SERVER_PORT := koanf.String("GO_SERVER")

	zap.Info("Server is running on: " + GO_SERVER_PORT)

	var err error
	go func() {
		err = fiber.Listen(GO_SERVER_PORT, gofiber.ListenConfig{
			DisableStartupMessage: true,
			EnablePrefork:         false,
		})
		if err != nil {
			zap.Fatal("Error starting server", zapLog.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	zap.Info("Got one of stop signals")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
		zap.Error("Failed to shutdown tracer provider", zapLog.Error(err))
	}
	if err := meterProvider.Shutdown(shutdownCtx); err != nil {
		zap.Error("Failed to shutdown meter provider", zapLog.Error(err))
	}
	if err := loggerProvider.Shutdown(shutdownCtx); err != nil {
		zap.Error("Failed to shutdown logger provider", zapLog.Error(err))
	}

	err = fiber.ShutdownWithContext(shutdownCtx)
	if err != nil {
		zap.Warn("Timeout, forced kill!", zapLog.Error(err))
		_ = zap.Sync()
		os.Exit(1)
	}

	zap.Info("Server has shut down gracefully")
	_ = zap.Sync()
}

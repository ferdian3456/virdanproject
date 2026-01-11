package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	goFiber "github.com/gofiber/fiber/v2"

	"github.com/ferdian3456/virdanproject/internal/config"
	middlewarepkg "github.com/ferdian3456/virdanproject/internal/middleware"
	exception "github.com/ferdian3456/virdanproject/internal/exception"
	"github.com/ferdian3456/virdanproject/internal/observability"
	"github.com/gofiber/contrib/otelfiber"
	"github.com/gofiber/fiber/v2/middleware/compress"
	zapLog "go.uber.org/zap"
)

// Build information (set via ldflags)
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	// Parse flags
	versionFlag := flag.Bool("version", false, "Show version information")
	healthcheckFlag := flag.Bool("healthcheck", false, "Run healthcheck and exit")
	flag.Parse()

	// Handle --version flag
	if *versionFlag {
		fmt.Printf("Virdan API\n")
		fmt.Printf("Version: %s\n", Version)
		fmt.Printf("Commit: %s\n", Commit)
		fmt.Printf("Build Time: %s\n", BuildTime)
		os.Exit(0)
	}

	// Handle --healthcheck flag
	if *healthcheckFlag {
		// Simple healthcheck - can be expanded
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get("http://localhost:8081/api/health")
		if err != nil {
			fmt.Println("Healthcheck failed:", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			fmt.Println("Healthcheck OK")
			os.Exit(0)
		}
		fmt.Printf("Healthcheck failed with status: %d\n", resp.StatusCode)
		os.Exit(1)
	}

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
	obsAppCfg := config.LoadObservabilityConfig(koanf, zap)

	// Initialize trace exporter
	obsCfg := observability.Config{
		ServiceName:  obsAppCfg.ServiceName,
		Environment:  obsAppCfg.Environment,
		OtelEndpoint: obsAppCfg.OtelEndpoint,
		OtelHeaders:  obsAppCfg.OtelHeaders,
	}

	shutdownTracer, err := observability.Init(ctx, obsCfg, zap)
	if err != nil {
		zap.Fatal("failed to initialize otel trace exporter", zapLog.Error(err))
	}
	defer shutdownTracer(ctx)

	fiber.Use(func(c *goFiber.Ctx) error {
		ctx := c.UserContext()
		c.SetUserContext(ctx)
		return c.Next()
	})

	fiber.Use(otelfiber.Middleware(
		otelfiber.WithServerName("virdan-api"),
	))

	// Trace logger middleware - injects trace_id and span_id into logger
	// Must come AFTER otelfiber middleware so trace context is available
	fiber.Use(middlewarepkg.TraceLoggerMiddleware(zap))

	// Custom recovery middleware to handle panics with JSON response
	fiber.Use(exception.Recovery(zap))

	// Compression middleware (should be before logging)
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


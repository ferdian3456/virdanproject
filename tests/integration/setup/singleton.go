package setup

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Global singleton infrastructure - shared across all tests
var (
	globalInfra   *TestInfra
	globalInfraMu sync.RWMutex
	globalCtx     context.Context
	globalCancel  context.CancelFunc

	// Track how many "TestMain" calls are active
	// When this reaches 0, we can safely shutdown
	testMainCount int
	testMainMu    sync.Mutex
)

// RunTestsWithSingleton is a helper to use in TestMain of each test package
// It initializes the singleton infrastructure, runs tests, and cleans up
//
// Usage in each test package:
//
//	func TestMain(m *testing.M) {
//	    setup.RunTestsWithSingleton(m)
//	}
func RunTestsWithSingleton(m *testing.M) {
	testMainMu.Lock()
	testMainCount++
	testMainMu.Unlock()

	// Initialize singleton if not already done
	if err := EnsureSingletonInitialized(); err != nil {
		fmt.Printf("FATAL: Failed to start test infrastructure: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	exitCode := m.Run()

	// Decrement counter and cleanup if this is the last TestMain
	testMainMu.Lock()
	testMainCount--
	isLast := testMainCount == 0
	testMainMu.Unlock()

	if isLast {
		fmt.Println("\n=== All test packages completed, shutting down infrastructure ===")
		if err := ShutdownSingleton(); err != nil {
			fmt.Printf("Warning: Error during shutdown: %v\n", err)
		}
	}

	os.Exit(exitCode)
}

// TestMain sets up singleton infrastructure for all tests
// This is called once before any test runs and cleans up after all tests complete
func TestMain(m *testing.M) {
	RunTestsWithSingleton(m)
}

// startSingletonInfra starts all containers once for all tests
func startSingletonInfra(ctx context.Context) (*TestInfra, error) {
	// 1. Start Postgres
	fmt.Println("Starting PostgreSQL container...")
	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("virdan_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start postgres: %w", err)
	}

	pgURL, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("failed to get postgres connection string: %w", err)
	}
	fmt.Printf("PostgreSQL started at: %s\n", pgURL)

	// 2. Start Redis
	fmt.Println("Starting Redis container...")
	redisContainer, err := redis.Run(ctx,
		"redis:7-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start redis: %w", err)
	}

	redisHost, err := redisContainer.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get redis host: %w", err)
	}

	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	if err != nil {
		return nil, fmt.Errorf("failed to get redis port: %w", err)
	}

	redisURL := fmt.Sprintf("%s:%s", redisHost, redisPort.Port())
	fmt.Printf("Redis started at: %s\n", redisURL)

	// 3. Start MinIO
	fmt.Println("Starting MinIO container...")
	minioContainer, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image: "minio/minio:latest",
				Cmd:   []string{"server", "/data", "--console-address", ":9001"},
				Env: map[string]string{
					"MINIO_ROOT_USER":     "minioadmin",
					"MINIO_ROOT_PASSWORD": "minioadmin",
				},
				ExposedPorts: []string{"9000/tcp", "9001/tcp"},
				WaitingFor:   wait.ForListeningPort("9000/tcp"),
			},
			Started: true,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start minio: %w", err)
	}

	minioHost, err := minioContainer.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get minio host: %w", err)
	}

	minioPort, err := minioContainer.MappedPort(ctx, "9000")
	if err != nil {
		return nil, fmt.Errorf("failed to get minio port: %w", err)
	}

	minioURL := fmt.Sprintf("%s:%s", minioHost, minioPort.Port())
	fmt.Printf("MinIO started at: %s\n", minioURL)

	// 4. Start MailHog
	fmt.Println("Starting MailHog container...")
	mailhogContainer, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        "mailhog/mailhog:latest",
				ExposedPorts: []string{"1025/tcp", "8025/tcp"},
				WaitingFor:   wait.ForListeningPort("1025/tcp"),
			},
			Started: true,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start mailhog: %w", err)
	}

	mailhogHost, err := mailhogContainer.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get mailhog host: %w", err)
	}

	mailhogAPIPort, err := mailhogContainer.MappedPort(ctx, "8025")
	if err != nil {
		return nil, fmt.Errorf("failed to get mailhog API port: %w", err)
	}

	mailhogSMTPPort, err := mailhogContainer.MappedPort(ctx, "1025")
	if err != nil {
		return nil, fmt.Errorf("failed to get mailhog SMTP port: %w", err)
	}

	mailhogURL := fmt.Sprintf("http://%s:%s", mailhogHost, mailhogAPIPort.Port())
	mailhogSMTP := fmt.Sprintf("%s:%s", mailhogHost, mailhogSMTPPort.Port())
	fmt.Printf("MailHog started at: %s (API), %s (SMTP)\n", mailhogURL, mailhogSMTP)

	return &TestInfra{
		Postgres:    pgContainer,
		Redis:       redisContainer,
		MinIO:       minioContainer,
		MailHog:     mailhogContainer,
		PgURL:       pgURL,
		RedisURL:    redisURL,
		MinioURL:    minioURL,
		MailhogURL:  mailhogURL,
		MailhogSMTP: mailhogSMTP,
	}, nil
}

// GetGlobalInfra returns the global singleton infrastructure
// This is safe to call from parallel tests
func GetGlobalInfra() *TestInfra {
	globalInfraMu.RLock()
	defer globalInfraMu.RUnlock()
	return globalInfra
}

// GetGlobalContext returns the global context
func GetGlobalContext() context.Context {
	return globalCtx
}

// EnsureSingletonInitialized ensures the singleton infrastructure is initialized
// This can be called from any test package to initialize the singleton
// Must be paired with ShutdownSingleton() when tests complete
func EnsureSingletonInitialized() error {
	globalInfraMu.Lock()
	defer globalInfraMu.Unlock()

	if globalInfra != nil {
		return nil // Already initialized
	}

	// Setup context with cancellation
	globalCtx, globalCancel = context.WithCancel(context.Background())

	// Start singleton infrastructure (only once)
	fmt.Println("=== Starting Singleton Test Infrastructure ===")

	// Start containers
	infra, err := startSingletonInfra(globalCtx)
	if err != nil {
		return fmt.Errorf("failed to start test infrastructure: %w", err)
	}

	globalInfra = infra
	fmt.Println("=== Singleton Test Infrastructure Ready ===")
	return nil
}

// ShutdownSingleton shuts down the singleton infrastructure
// This should be called once after all tests complete
func ShutdownSingleton() error {
	globalInfraMu.Lock()
	defer globalInfraMu.Unlock()

	if globalInfra == nil {
		return nil // Already shut down
	}

	fmt.Println("=== Shutting Down Singleton Test Infrastructure ===")
	ctx := context.Background()
	if err := globalInfra.Terminate(ctx, &testing.T{}); err != nil {
		return err
	}
	if globalCancel != nil {
		globalCancel()
	}
	globalInfra = nil
	fmt.Println("=== Test Infrastructure Shutdown Complete ===")
	return nil
}

package setup

import (
	"context"
	"fmt"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

type TestInfra struct {
	Postgres *postgres.PostgresContainer
	Redis    *redis.RedisContainer
	MinIO    testcontainers.Container
	MailHog  testcontainers.Container

	PgURL       string
	RedisURL    string
	MinioURL    string
	MailhogURL  string
	MailhogSMTP string
}

func StartInfra(ctx context.Context, t *testing.T) (*TestInfra, error) {
	t.Log("Starting test infrastructure...")

	// 1. Start Postgres
	t.Log("Starting PostgreSQL container...")
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
	t.Logf("PostgreSQL started at: %s", pgURL)

	// 2. Start Redis
	t.Log("Starting Redis container...")
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
	t.Logf("Redis started at: %s", redisURL)

	// 3. Start MinIO
	t.Log("Starting MinIO container...")
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
	t.Logf("MinIO started at: %s", minioURL)

	// 4. Start MailHog
	t.Log("Starting MailHog container...")
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
	t.Logf("MailHog started at: %s (API), %s (SMTP)", mailhogURL, mailhogSMTP)

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

func (infra *TestInfra) Terminate(ctx context.Context, t *testing.T) error {
	t.Log("Terminating test infrastructure...")

	if infra.Postgres != nil {
		if err := infra.Postgres.Terminate(ctx); err != nil {
			return fmt.Errorf("failed to terminate postgres: %w", err)
		}
	}
	if infra.Redis != nil {
		if err := infra.Redis.Terminate(ctx); err != nil {
			return fmt.Errorf("failed to terminate redis: %w", err)
		}
	}
	if infra.MinIO != nil {
		if err := infra.MinIO.Terminate(ctx); err != nil {
			return fmt.Errorf("failed to terminate minio: %w", err)
		}
	}
	if infra.MailHog != nil {
		if err := infra.MailHog.Terminate(ctx); err != nil {
			return fmt.Errorf("failed to terminate mailhog: %w", err)
		}
	}

	t.Log("Test infrastructure terminated successfully")
	return nil
}

package setup

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/ferdian3456/virdanproject/services"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
)

func SetupTestApp(t *testing.T, pgURL, redisURL, minioURL, mailhogSMTP string) (*fiber.App, *pgxpool.Pool, *redis.Client, *minio.Client) {
	t.Log("Setting up test application...")

	ctx := context.Background()

	testConfig := koanf.New(".")
	_ = testConfig.Set("postgres_url", pgURL)
	_ = testConfig.Set("redis_addr", redisURL)
	_ = testConfig.Set("minio_url", minioURL)
	_ = testConfig.Set("minio_http", "http://")
	_ = testConfig.Set("minio_bucket_name", "virdan-test")
	_ = testConfig.Set("minio_access_key", "minioadmin")
	_ = testConfig.Set("minio_secret_key", "minioadmin")
	_ = testConfig.Set("jwt_secret_key", "test-secret-key-for-jwt-token-generation")

	_ = testConfig.Set("JWT_SECRET_KEY", "test-secret-key-for-jwt-token-generation")
	_ = testConfig.Set("MINIO_BUCKET_NAME", "virdan-test")
	_ = testConfig.Set("MINIO_ACCESS_KEY", "minioadmin")
	_ = testConfig.Set("MINIO_SECRET_KEY", "minioadmin")

	smtpParts := strings.Split(mailhogSMTP, ":")
	smtpHost := smtpParts[0]
	smtpPort, _ := strconv.Atoi(smtpParts[1])

	_ = testConfig.Set("smtp_host", smtpHost)
	_ = testConfig.Set("smtp_port", smtpPort)
	_ = testConfig.Set("sender_name", "Virdan Test")
	_ = testConfig.Set("sender_email", "noreply@virdan.test")
	_ = testConfig.Set("sender_password", "")

	_ = testConfig.Set("SMTP_HOST", smtpHost)
	_ = testConfig.Set("SMTP_PORT", smtpPort)
	_ = testConfig.Set("SENDER_NAME", "Virdan Test <noreply@virdan.test>")
	_ = testConfig.Set("SENDER_EMAIL", "noreply@virdan.test")
	_ = testConfig.Set("SENDER_PASSWORD", "")

	_ = testConfig.Set("XENDIT_SECRET_KEY", "xnd_development_test_key")
	_ = testConfig.Set("XENDIT_WEBHOOK_TOKEN", "test_webhook_token_12345")
	_ = testConfig.Set("XENDIT_API_BASE_URL", "https://api.xendit.co")
	_ = testConfig.Set("XENDIT_SUCCESS_URL", "https://virdan.cloud/payment/success")
	_ = testConfig.Set("XENDIT_CANCEL_URL", "https://virdan.cloud/payment/cancel")

	t.Log("Connecting to test PostgreSQL...")
	dbPool, err := pgxpool.New(ctx, pgURL)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	t.Log("Connecting to test Redis...")
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisURL,
		DB:   0,
	})

	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Fatalf("failed to connect to test redis: %v", err)
	}

	t.Log("Connecting to test MinIO...")
	minioClient, err := minio.New(minioURL, &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("failed to connect to minio: %v", err)
	}

	bucketName := "virdan-test"
	exists, err := minioClient.BucketExists(ctx, bucketName)
	if err != nil {
		t.Fatalf("failed to check minio bucket: %v", err)
	}

	if !exists {
		t.Logf("Creating MinIO bucket: %s", bucketName)
		err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			errResp := minio.ToErrorResponse(err)
			if errResp.Code != "BucketAlreadyOwnedByYou" && errResp.Code != "BucketAlreadyExists" {
				t.Fatalf("failed to create minio bucket: %v", err)
			}
			t.Logf("MinIO bucket already exists (race-safe): %s", bucketName)
		}
	} else {
		t.Logf("MinIO bucket already exists: %s", bucketName)
	}

	zapLogger := zap.NewExample()
	defer func() {
		_ = zapLogger.Sync()
	}()

	deps := services.Deps{
		DB:     dbPool,
		Redis:  redisClient,
		MinIO:  minioClient,
		FCM:    nil,
		Config: testConfig,
		Log:    zapLogger,
	}
	registry := services.Wire(deps)

	fiberApp := fiber.New(fiber.Config{
		AppName:          "Virdan Test",
		DisableKeepalive: true,
		ErrorHandler: func(c fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	services.RegisterRoutes(fiberApp, registry, deps)

	t.Log("Test application setup completed successfully")

	return fiberApp, dbPool, redisClient, minioClient
}

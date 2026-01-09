package setup

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/ferdian3456/virdanproject/internal/delivery/http"
	"github.com/ferdian3456/virdanproject/internal/delivery/http/middleware"
	"github.com/ferdian3456/virdanproject/internal/delivery/http/route"
	"github.com/ferdian3456/virdanproject/internal/repository"
	"github.com/ferdian3456/virdanproject/internal/usecase"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
)

func SetupTestApp(t *testing.T, pgURL, redisURL, minioURL, mailhogSMTP string) (*fiber.App, *pgxpool.Pool, *redis.Client, *minio.Client) {
	t.Log("Setting up test application...")

	ctx := context.Background()

	// 1. Create test config dengan test infrastructure values
	testConfig := koanf.New(".")
	_ = testConfig.Set("postgres_url", pgURL)
	_ = testConfig.Set("redis_addr", redisURL)
	_ = testConfig.Set("minio_url", minioURL)
	_ = testConfig.Set("minio_http", "http://")
	_ = testConfig.Set("minio_bucket_name", "virdan-test")
	_ = testConfig.Set("minio_access_key", "minioadmin")
	_ = testConfig.Set("minio_secret_key", "minioadmin")
	_ = testConfig.Set("jwt_secret_key", "test-secret-key-for-jwt-token-generation")

	// Set uppercase keys for compatibility with existing code
	_ = testConfig.Set("JWT_SECRET_KEY", "test-secret-key-for-jwt-token-generation")
	_ = testConfig.Set("MINIO_BUCKET_NAME", "virdan-test")
	_ = testConfig.Set("MINIO_ACCESS_KEY", "minioadmin")
	_ = testConfig.Set("MINIO_SECRET_KEY", "minioadmin")

	// Use MailHog for SMTP
	// mailhogSMTP format: host:port (e.g., localhost:32768)
	smtpParts := strings.Split(mailhogSMTP, ":")
	smtpHost := smtpParts[0]
	smtpPort, _ := strconv.Atoi(smtpParts[1])

	_ = testConfig.Set("smtp_host", smtpHost)
	_ = testConfig.Set("smtp_port", smtpPort)
	_ = testConfig.Set("sender_name", "Virdan Test")
	_ = testConfig.Set("sender_email", "noreply@virdan.test")
	_ = testConfig.Set("sender_password", "")

	// Set uppercase keys for compatibility with existing code
	_ = testConfig.Set("SMTP_HOST", smtpHost)
	_ = testConfig.Set("SMTP_PORT", smtpPort)
	_ = testConfig.Set("SENDER_NAME", "Virdan Test <noreply@virdan.test>") // Include email address
	_ = testConfig.Set("SENDER_EMAIL", "noreply@virdan.test")
	_ = testConfig.Set("SENDER_PASSWORD", "")

	// 3. Connect to PostgreSQL
	t.Log("Connecting to test PostgreSQL...")
	dbPool, err := pgxpool.New(ctx, pgURL)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	// 4. Connect to Redis
	t.Log("Connecting to test Redis...")
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisURL,
		DB:   0, // Use default DB for testing
	})

	// Test redis connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Fatalf("failed to connect to test redis: %v", err)
	}

	// 5. Connect to MinIO
	t.Log("Connecting to test MinIO...")
	minioClient, err := minio.New(minioURL, &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("failed to connect to minio: %v", err)
	}

	// Create bucket kalau belum ada
	bucketName := "virdan-test"
	exists, err := minioClient.BucketExists(ctx, bucketName)
	if err != nil {
		t.Fatalf("failed to check minio bucket: %v", err)
	}

	if !exists {
		t.Logf("Creating MinIO bucket: %s", bucketName)
		err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			t.Fatalf("failed to create minio bucket: %v", err)
		}
	} else {
		t.Logf("MinIO bucket already exists: %s", bucketName)
	}

	// 6. Setup logger (use development config for test)
	zapLogger := zap.NewExample()
	defer func() {
		_ = zapLogger.Sync()
	}()

	// 7. Setup repositories
	serverRepository := repository.NewServerRepository(zapLogger, dbPool, redisClient, minioClient)
	userRepository := repository.NewUserRepository(zapLogger, dbPool, redisClient, minioClient)
	postRepository := repository.NewPostRepository(zapLogger, dbPool, redisClient, minioClient)

	// 8. Setup usecases
	serverUsecase := usecase.NewServerUsecase(serverRepository, dbPool, zapLogger, testConfig)
	userUsecase := usecase.NewUserUsecase(userRepository, serverRepository, dbPool, zapLogger, testConfig)
	postUsecase := usecase.NewPostUsecase(postRepository, dbPool, zapLogger, testConfig)

	// 9. Setup controllers
	serverController := http.NewServerController(serverUsecase, zapLogger, testConfig)
	userController := http.NewUserController(userUsecase, zapLogger, testConfig)
	postController := http.NewPostController(postUsecase, zapLogger, testConfig)

	// 10. Setup middleware
	authMiddleware := middleware.NewAuthMiddleware(nil, zapLogger, testConfig, userUsecase)

	// 11. Setup Fiber app
	fiberApp := fiber.New(fiber.Config{
		AppName:               "Virdan Test",
		DisableStartupMessage: true,
		DisableKeepalive:      true, // Important for tests
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	// 12. Setup routes
	routeConfig := route.RouteConfig{
		App:              fiberApp,
		UserController:   userController,
		ServerController: serverController,
		PostController:   postController,
		AuthMiddleware:   authMiddleware,
	}

	routeConfig.SetupRoute()

	t.Log("Test application setup completed successfully")

	return fiberApp, dbPool, redisClient, minioClient
}

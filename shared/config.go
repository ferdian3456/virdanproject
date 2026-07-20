package shared

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/bytedance/sonic"
	"github.com/exaring/otelpgx"
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/parsers/dotenv"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/exemplar"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/api/option"
)

func NewFiber() *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:           "",
		BodyLimit:         105 * 1024 * 1024,
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

func NewFCMClient(ctx context.Context, config *koanf.Koanf, log *zap.Logger) *messaging.Client {
	raw := config.String("FIREBASE_SERVICE_ACCOUNT_BASE64_JSON")
	if raw == "" {
		log.Fatal("Failed to get firebase service account from environment variable because it's empty")
	}

	creds, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		log.Fatal("Failed to decode base64 firebase service account because it's not valid base64-encoded", zap.Error(err))
	}

	app, err := firebase.NewApp(ctx, nil, option.WithAuthCredentialsJSON(option.ServiceAccount, creds))
	if err != nil {
		log.Fatal("Failed to initialize firebase app", zap.Error(err))
	}

	msgClient, err := app.Messaging(ctx)
	if err != nil {
		log.Fatal("Failed to get firebase messaging client", zap.Error(err))
	}

	return msgClient
}

func NewKoanf(log *zap.Logger) *koanf.Koanf {
	k := koanf.New(".")
	err := k.Load(file.Provider(".env"), dotenv.Parser())
	if err != nil {
		log.Fatal("Failed to load .env files", zap.Error(err))
	}

	return k
}

func NewMinIO(ctx context.Context, config *koanf.Koanf, log *zap.Logger) *minio.Client {
	endpoint := config.String("MINIO_INTERNAL_URL")
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.String("MINIO_USER"), config.String("MINIO_PASSWORD"), ""),
		Secure: false,
	})
	if err != nil {
		log.Fatal("Failed to initialize minio client", zap.Error(err))
	}

	bucketName := config.String("MINIO_BUCKET_NAME")
	location := config.String("MINIO_LOCATION")

	err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{
		Region: location,
	})

	if err != nil {
		exists, errBucketExists := minioClient.BucketExists(ctx, bucketName)
		if errBucketExists == nil && exists {
			log.Info("MinIO bucket already exists", zap.String("bucket", bucketName))
		} else {
			log.Fatal("Failed to create minio bucket", zap.Error(err), zap.String("bucket", bucketName))
		}
	} else {
		log.Info("Successfully created minio bucket", zap.String("bucket", bucketName))
	}

	log.Info("MinIO client initialized",
		zap.String("endpoint", endpoint),
		zap.String("bucket", bucketName),
		zap.String("location", location),
	)

	return minioClient
}

func NewOtelLoggerProvider(ctx context.Context, koanf *koanf.Koanf, log *zap.Logger) *sdklog.LoggerProvider {
	serviceName := koanf.String("OTEL_SERVICE_NAME")
	serviceVersion := koanf.String("OTEL_SERVICE_VERSION")
	deploymentEnvironment := koanf.String("OTEL_DEPLOYMENT_ENVIRONMENT")
	otelExporterOTLPEndpoint := koanf.String("OTEL_EXPORTER_OTLP_ENDPOINT")
	batchExportTimeout := koanf.Duration("OTEL_LOG_BATCH_EXPORT_TIMEOUT")
	batchTimeout := koanf.Duration("OTEL_LOG_BATCH_TIMEOUT")
	batchMaxQueueSize := koanf.Int("OTEL_LOG_BATCH_MAX_QUEUE_SIZE")

	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(serviceVersion),
		semconv.DeploymentEnvironment(deploymentEnvironment),
	),
		resource.WithHost(),
	)
	if err != nil {
		log.Fatal("failed to create resource", zap.Error(err))
	}

	exporter, err := otlploggrpc.New(ctx,
		otlploggrpc.WithEndpoint(otelExporterOTLPEndpoint),
		otlploggrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatal("failed to create log exporter", zap.Error(err))
	}

	processor := sdklog.NewBatchProcessor(
		exporter,
		sdklog.WithExportTimeout(batchExportTimeout),
		sdklog.WithExportInterval(batchTimeout),
		sdklog.WithMaxQueueSize(batchMaxQueueSize),
	)

	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(processor),
	)

	global.SetLoggerProvider(loggerProvider)

	log.Info("OpenTelemetry logger initialized",
		zap.String("otlp_endpoint", otelExporterOTLPEndpoint),
		zap.String("service", serviceName),
		zap.String("version", serviceVersion),
		zap.String("environment", deploymentEnvironment),
		zap.String("batch_export_timeout", batchExportTimeout.String()),
		zap.String("batch_timeout", batchTimeout.String()),
		zap.Int("batch_max_queue_size", batchMaxQueueSize),
	)

	return loggerProvider
}

func NewOtelMetricProvider(ctx context.Context, koanf *koanf.Koanf, log *zap.Logger) *sdkmetric.MeterProvider {
	serviceName := koanf.String("OTEL_SERVICE_NAME")
	serviceVersion := koanf.String("OTEL_SERVICE_VERSION")
	deploymentEnvironment := koanf.String("OTEL_DEPLOYMENT_ENVIRONMENT")
	otelExporterOTLPEndpoint := koanf.String("OTEL_EXPORTER_OTLP_ENDPOINT")
	intervalMetrics := koanf.Duration("OTEL_METRIC_INTERVAL")

	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(serviceVersion),
		semconv.DeploymentEnvironment(deploymentEnvironment),
	),
		resource.WithHost(),
	)
	if err != nil {
		log.Fatal("Failed to create resource", zap.Error(err))
	}

	exporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(otelExporterOTLPEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatal("Failed to create metrics exporter", zap.Error(err))
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(intervalMetrics))),
		sdkmetric.WithExemplarFilter(exemplar.TraceBasedFilter),
	)

	otel.SetMeterProvider(meterProvider)

	if err := runtime.Start(); err != nil {
		log.Fatal("Failed to start runtime metrics", zap.Error(err))
	}

	log.Info("OpenTelemetry metrics initialized",
		zap.String("otlp_endpoint", otelExporterOTLPEndpoint),
		zap.String("service", serviceName),
		zap.String("version", serviceVersion),
		zap.String("environment", deploymentEnvironment),
		zap.Duration("interval", intervalMetrics),
	)

	return meterProvider
}

func NewOtelTracerProvider(ctx context.Context, koanf *koanf.Koanf, log *zap.Logger) *sdktrace.TracerProvider {
	traceServiceName := koanf.String("OTEL_SERVICE_NAME")
	traceServiceVersion := koanf.String("OTEL_SERVICE_VERSION")
	traceDeploymentEnvironment := koanf.String("OTEL_DEPLOYMENT_ENVIRONMENT")
	otelTraceExporterOTLPEndpoint := koanf.String("OTEL_EXPORTER_OTLP_ENDPOINT")
	batchTimeout := koanf.Duration("OTEL_TRACE_BATCH_TIMEOUT")
	batchExportTimeout := koanf.Duration("OTEL_TRACE_BATCH_EXPORT_TIMEOUT")
	maxQueueSize := koanf.Int("OTEL_TRACE_MAX_QUEUE_SIZE")
	sampleRate := koanf.Float64("OTEL_TRACE_SAMPLE_RATE")

	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(traceServiceName),
		semconv.ServiceVersion(traceServiceVersion),
		semconv.DeploymentEnvironment(traceDeploymentEnvironment),
	),
		resource.WithHost(),
	)
	if err != nil {
		log.Fatal("Failed to create resource", zap.Error(err))
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otelTraceExporterOTLPEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatal("Failed to create trace exporter", zap.Error(err))
	}

	batchProcessor := sdktrace.NewBatchSpanProcessor(
		exporter,
		sdktrace.WithMaxQueueSize(maxQueueSize),
		sdktrace.WithBatchTimeout(batchTimeout),
		sdktrace.WithExportTimeout(batchExportTimeout),
	)

	sampler := sdktrace.ParentBased(
		sdktrace.TraceIDRatioBased(sampleRate),
	)

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(batchProcessor),
		sdktrace.WithSampler(sampler),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Info("OpenTelemetry tracer initialized",
		zap.String("otlp_endpoint", otelTraceExporterOTLPEndpoint),
		zap.String("service", traceServiceName),
		zap.String("version", traceServiceVersion),
		zap.String("environment", traceDeploymentEnvironment),
		zap.String("batch_timeout", batchTimeout.String()),
		zap.String("batch_export_timeout", batchExportTimeout.String()),
		zap.Int("batch_max_queue_size", maxQueueSize),
		zap.Float64("sample_rate", sampleRate),
	)

	return tracerProvider
}

func NewPostgresqlPool(ctx context.Context, config *koanf.Koanf, log *zap.Logger) *pgxpool.Pool {
	dsn := config.String("POSTGRES_URL")
	pgxConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatal("Failed to parse postgresl config", zap.Error(err))
	}

	pgxConfig.MaxConns = 20
	pgxConfig.MinConns = 5
	pgxConfig.MaxConnLifetime = 30 * time.Minute
	pgxConfig.MaxConnIdleTime = 5 * time.Minute
	pgxConfig.HealthCheckPeriod = 1 * time.Minute

	pgxConfig.ConnConfig.Tracer = otelpgx.NewTracer(
		otelpgx.WithIncludeQueryParameters(),
	)

	pool, err := pgxpool.NewWithConfig(ctx, pgxConfig)
	if err != nil {
		log.Fatal("Failed to create pgx pool", zap.Error(err))
	}

	err = pool.Ping(ctx)
	if err != nil {
		log.Fatal("Failed to ping postgresql database", zap.Error(err))
	}

	log.Info("PostgreSQL connection pool initialized",
		zap.String("host", pgxConfig.ConnConfig.Host),
		zap.Uint16("port", pgxConfig.ConnConfig.Port),
		zap.String("database", pgxConfig.ConnConfig.Database),
		zap.Int32("max_conns", pgxConfig.MaxConns),
		zap.Int32("min_conns", pgxConfig.MinConns),
		zap.Duration("max_conn_lifetime", pgxConfig.MaxConnLifetime),
		zap.Duration("max_conn_idle_time", pgxConfig.MaxConnIdleTime),
		zap.Duration("health_check_period", pgxConfig.HealthCheckPeriod),
	)

	return pool
}

func NewRedisClient(ctx context.Context, config *koanf.Koanf, log *zap.Logger) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:         config.String("REDIS_URL"),
		Password:     "",
		DB:           0,
		MinIdleConns: 10,
		PoolSize:     100,
		PoolTimeout:  30 * time.Second,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,

		MaxRetries:      3,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
	})

	if err := redisotel.InstrumentTracing(rdb); err != nil {
		log.Fatal("Failed to instrument Redis tracing", zap.Error(err))
	}
	if err := redisotel.InstrumentMetrics(rdb); err != nil {
		log.Fatal("Failed to instrument Redis metrics", zap.Error(err))
	}

	err := rdb.Ping(ctx).Err()
	if err != nil {
		log.Fatal("Failed to connect redis", zap.Error(err))
	}

	log.Info("Redis client initialized",
		zap.String("addr", config.String("REDIS_URL")),
		zap.Int("min_idle_conns", rdb.Options().MinIdleConns),
		zap.Int("pool_size", rdb.Options().PoolSize),
		zap.Duration("pool_timeout", rdb.Options().PoolTimeout),
		zap.Duration("dial_timeout", rdb.Options().DialTimeout),
		zap.Duration("read_timeout", rdb.Options().ReadTimeout),
		zap.Duration("write_timeout", rdb.Options().WriteTimeout),
		zap.Int("max_retries", rdb.Options().MaxRetries),
		zap.Duration("min_retry_backoff", rdb.Options().MinRetryBackoff),
		zap.Duration("max_retry_backoff", rdb.Options().MaxRetryBackoff),
	)

	return rdb
}

func NewBootstrapZap() *zap.Logger {
	cfg := zap.NewProductionConfig()
	cfg.DisableStacktrace = true
	cfg.DisableCaller = false
	cfg.EncoderConfig.StacktraceKey = ""
	cfg.EncoderConfig.TimeKey = "timestamp"

	log, _ := cfg.Build()

	return log
}

func NewZap(koanf *koanf.Koanf, loggerProvider *sdklog.LoggerProvider) *zap.Logger {
	serviceName := koanf.String("OTEL_SERVICE_NAME")
	serviceVersion := koanf.String("OTEL_SERVICE_VERSION")
	deploymentEnvironment := koanf.String("OTEL_DEPLOYMENT_ENVIRONMENT")
	logLevel := koanf.String("LOG_LEVEL")

	otelCore := otelzap.NewCore(serviceName, otelzap.WithLoggerProvider(loggerProvider))

	levelStr := strings.ToLower(logLevel)
	var level zapcore.Level
	switch levelStr {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	case "panic":
		level = zapcore.PanicLevel
	case "fatal":
		level = zapcore.FatalLevel
	default:
		level = zapcore.InfoLevel
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	encoder := zapcore.NewJSONEncoder(encoderCfg)
	consoleCore := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	combinedCore := zapcore.NewTee(consoleCore, otelCore)

	logger := zap.New(combinedCore,
		zap.AddCaller(),
		zap.Fields(
			zap.String("service", serviceName),
			zap.String("version", serviceVersion),
			zap.String("environment", deploymentEnvironment),
		),
	)

	logger.Info("Zap logger initialized",
		zap.String("service", serviceName),
		zap.String("version", serviceVersion),
		zap.String("environment", deploymentEnvironment),
		zap.String("log_level", logLevel),
	)

	return logger
}

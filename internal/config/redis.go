package config

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/knadh/koanf/v2"
	redisotel "github.com/redis/go-redis/extra/redisotel/v9"
	"go.uber.org/zap"
)

func NewRedisClient(ctx context.Context, config *koanf.Koanf, log *zap.Logger) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:         config.String("REDIS_URL"), // Redis server address
		Password:     "",                         // No password set
		DB:           0,                          // Use default DB
		MinIdleConns: 10,                         // Minimum number of idle connections
		PoolSize:     100,                        // Maximum number of connections
		PoolTimeout:  30 * time.Second,           // Timeout for getting a connection from the pool
		DialTimeout:  5 * time.Second,            // Timeout for establishing a new connection
		ReadTimeout:  3 * time.Second,            // Timeout for reading a response
		WriteTimeout: 3 * time.Second,            // Timeout for writing a request

		MaxRetries:      3,                      // Maximum number of retries before giving up
		MinRetryBackoff: 8 * time.Millisecond,   // Minimum backoff between retries
		MaxRetryBackoff: 512 * time.Millisecond, // Maximum backoff between retries
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

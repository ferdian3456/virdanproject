package config

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
)

func NewRedisClient(config *koanf.Koanf, log *zap.Logger) *redis.Client {
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

	err := rdb.Ping(context.Background()).Err()
	if err != nil {
		log.Fatal("failed to connect redis", zap.Error(err))
	}

	return rdb
}

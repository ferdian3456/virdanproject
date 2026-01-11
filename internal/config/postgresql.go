package config

import (
	"context"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
)

func NewPostgresqlPool(config *koanf.Koanf, log *zap.Logger) *pgxpool.Pool {
	dsn := config.String("POSTGRES_URL")
	pgxConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatal("failed to parse postgresl config", zap.Error(err))
	}

	pgxConfig.MaxConns = 20
	pgxConfig.MinConns = 5
	pgxConfig.MaxConnLifetime = 30 * time.Minute
	pgxConfig.MaxConnIdleTime = 5 * time.Minute
	pgxConfig.HealthCheckPeriod = 1 * time.Minute
	pgxConfig.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(context.Background(), pgxConfig)
	if err != nil {
		log.Fatal("failed to create pgx pool", zap.Error(err))
	}

	err = pool.Ping(context.Background())
	if err != nil {
		log.Fatal("failed to ping postgresql database", zap.Error(err))
	}

	return pool
}

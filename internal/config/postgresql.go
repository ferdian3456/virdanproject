package config

import (
	"context"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
)

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

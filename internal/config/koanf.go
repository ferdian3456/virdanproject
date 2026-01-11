package config

import (
	"github.com/knadh/koanf/parsers/dotenv"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
)

func NewKoanf(log *zap.Logger) *koanf.Koanf {
	k := koanf.New(".")

	// Load from .env file if available (for local development)
	err := k.Load(file.Provider(".env"), dotenv.Parser())
	if err != nil {
		// .env file not found is OK in Docker (env vars from docker-compose)
		// Only log for debugging, don't fail
		log.Debug(".env file not found, using environment variables", zap.Error(err))
	}

	// Load from environment variables (for Docker)
	// This will override .env file values if both exist
	err = k.Load(env.Provider("", ".", nil), nil)
	if err != nil {
		log.Fatal("failed to load environment variables", zap.Error(err))
	}

	return k
}

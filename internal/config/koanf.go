package config

import (
	"github.com/knadh/koanf/parsers/dotenv"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
)

func NewKoanf(log *zap.Logger) *koanf.Koanf {
	k := koanf.New(".")
	err := k.Load(file.Provider(".env"), dotenv.Parser())
	if err != nil {
		log.Fatal("Failed to load .env files", zap.Error(err))
	}

	return k
}

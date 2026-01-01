package usecase

import (
	"github.com/ferdian3456/virdanproject/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
)

type ServerUsecase struct {
	ServerRepository *repository.ServerRepository
	DB               *pgxpool.Pool
	Log              *zap.Logger
	Config           *koanf.Koanf
}

func NewServerUsecase(serverRepository *repository.ServerRepository, db *pgxpool.Pool, zap *zap.Logger, koanf *koanf.Koanf) *ServerUsecase {
	return &ServerUsecase{
		ServerRepository: serverRepository,
		DB:               db,
		Log:              zap,
		Config:           koanf,
	}
}

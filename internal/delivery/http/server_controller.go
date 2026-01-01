package http

import (
	"github.com/ferdian3456/virdanproject/internal/usecase"
	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
)

type ServerController struct {
	ServerUsecase *usecase.ServerUsecase
	Log           *zap.Logger
	Config        *koanf.Koanf
}

func NewServerController(serverUsecase *usecase.ServerUsecase, zap *zap.Logger, koanf *koanf.Koanf) *ServerController {
	return &ServerController{
		ServerUsecase: serverUsecase,
		Log:           zap,
		Config:        koanf,
	}
}

//func (controller *ServerController) GetServer(ctx *fiber.Ctx) error {
//
//}

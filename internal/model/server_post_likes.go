package model

import (
	"time"

	"github.com/google/uuid"
)

type ServerPostLikes struct {
	PostId         uuid.UUID
	UserId         uuid.UUID
	CreateDatetime time.Time
	UpdateDatetime time.Time
	CreateUserId   uuid.UUID
	UpdateUserId   uuid.UUID
}

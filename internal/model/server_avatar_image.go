package model

import (
	"time"

	"github.com/google/uuid"
)

type ServerAvatarImage struct {
	Id             uuid.UUID
	ServerId       uuid.UUID
	Bucket         string
	ObjectKey      string
	MimeType       string
	CreateDatetime time.Time
	UpdateDatetime time.Time
	CreateUserId   uuid.UUID
	UpdateUserId   uuid.UUID
}

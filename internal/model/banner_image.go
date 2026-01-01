package model

import (
	"time"

	"github.com/google/uuid"
)

type BannerImage struct {
	Id             uuid.UUID
	ServerId       uuid.UUID
	Bucket         string
	ObjectKey      string
	MimeType       string
	Size           int64
	CreateDatetime time.Time
	UpdateDatetime time.Time
	CreateUserId   uuid.UUID
	UpdateUserId   uuid.UUID
}

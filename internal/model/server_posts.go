package model

import (
	"time"

	"github.com/google/uuid"
)

type ServerPosts struct {
	Id             uuid.UUID
	ServerId       uuid.UUID
	AuthorId       uuid.UUID
	PostImageId    uuid.UUID
	Caption        string
	CreateDatetime time.Time
	UpdateDatetime time.Time
	CreateUserId   uuid.UUID
	UpdateUserId   uuid.UUID
}

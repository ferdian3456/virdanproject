package model

import (
	"time"

	"github.com/google/uuid"
)

type ServerPostComments struct {
	Id             uuid.UUID
	PostId         uuid.UUID
	AuthorId       uuid.UUID
	ParentId       *uuid.UUID
	Content        string
	CreateDatetime time.Time
	UpdateDatetime time.Time
	CreateUserId   uuid.UUID
	UpdateUserId   uuid.UUID
}

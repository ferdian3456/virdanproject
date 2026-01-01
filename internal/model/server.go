package model

import (
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
)

type Server struct {
	Id             uuid.UUID
	OwnerId        uuid.UUID
	Name           string
	ShortName      string
	CategoryId     *int
	AvatarImageId  *uuid.UUID
	BannerImageId  *uuid.UUID
	Description    *string
	Settings       sonic.NoCopyRawMessage
	CreateDatetime time.Time
	UpdateDatetime time.Time
	CreateUserId   uuid.UUID
	UpdateUserId   uuid.UUID
}

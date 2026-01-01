package model

import (
	"time"

	"github.com/google/uuid"
)

type ServerMemberProfile struct {
	Id             uuid.UUID
	ServerMemberId uuid.UUID
	ServerId       uuid.UUID
	UserId         uuid.UUID
	Username       string
	Fullname       string
	Bio            *string
	AvatarImageId  *uuid.UUID
	CreateDatetime time.Time
	UpdateDatetime time.Time
	CreateUserId   uuid.UUID
	UpdateUserId   uuid.UUID
}

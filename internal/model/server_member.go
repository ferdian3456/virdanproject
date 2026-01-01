package model

import (
	"time"

	"github.com/google/uuid"
)

type Status int16

const (
	MemberStatusActive Status = 1
	MemberStatusLeft   Status = 2
	MemberStatusBanned Status = 3
)

type ServerMember struct {
	Id             uuid.UUID
	ServerId       uuid.UUID
	UserId         uuid.UUID
	ServerRoleId   uuid.UUID
	Status         Status
	JoinedAt       time.Time
	LeftAt         *time.Time
	CreateDatetime time.Time
	UpdateDatetime time.Time
	CreateUserId   uuid.UUID
	UpdateUserId   uuid.UUID
}

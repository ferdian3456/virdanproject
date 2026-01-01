package model

import (
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
)

const OwnerRole = "OWNER"

type ServerRole struct {
	Id             uuid.UUID
	ServerId       uuid.UUID
	Name           string
	Permissions    sonic.NoCopyRawMessage
	CreateDatetime time.Time
	UpdateDatetime time.Time
	CreateUserId   uuid.UUID
	UpdateUserId   uuid.UUID
}

package model

import (
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
)

const OwnerRole = "Owner"
const MemberRole = "Member"

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

package model

import (
	"time"
)

type ServerMember struct {
	Id           string
	ServerId     string
	UserId       string
	ServerRoleId string
	JoinedAt     time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CreatedBy    string
	UpdatedBy    string
}

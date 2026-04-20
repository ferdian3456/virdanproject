package model

import (
	"time"
)

type Status int16

const (
	MemberStatusActive Status = 1
	MemberStatusLeft   Status = 2
	MemberStatusBanned Status = 3
)

type ServerMember struct {
	Id           string
	ServerId     string
	UserId       string
	ServerRoleId string
	Status       Status
	JoinedAt     time.Time
	LeftAt       *time.Time
	Audit
}

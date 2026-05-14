package model

import (
	"time"
)

const OwnerRole = "Owner"
const MemberRole = "Member"

type ServerRole struct {
	Id          string
	ServerId    string
	Name        string
	Permissions []byte
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedBy   string
	UpdatedBy   string
}

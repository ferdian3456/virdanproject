package model

import "time"

type ServerInvite struct {
	Id        string
	ServerId  string
	Code      string
	MaxUses   int
	UsedCount int
	ExpiresAt *time.Time
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
}

package model

import (
	"time"
)

type RefreshToken struct {
	Id          string
	UserId      string
	TokenHash   string
	TokenFamily string
	ExpiresAt   time.Time
	RevokedAt   *time.Time
	Audit
}

type RefreshTokenRefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

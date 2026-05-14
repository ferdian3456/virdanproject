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
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedBy   string
	UpdatedBy   string
}

type RefreshTokenRefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

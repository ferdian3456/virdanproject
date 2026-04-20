package model

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	Id          uuid.UUID
	UserId      uuid.UUID
	TokenHash   string
	TokenFamily string
	ExpiresAt   time.Time
	RevokedAt   *time.Time
	Audit
}

type RefreshTokenRefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

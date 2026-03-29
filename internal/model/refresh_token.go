package model

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	Id         uuid.UUID
	UserId     uuid.UUID
	TokenHash  string
	TokenFamily string
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
	CreatedBy  uuid.UUID
	UpdatedBy  uuid.UUID
}

type RefreshTokenCreate struct {
	Id          uuid.UUID
	UserId      uuid.UUID
	TokenHash   string
	TokenFamily string
	ExpiresAt   time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedBy   uuid.UUID
}

type RefreshTokenRefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

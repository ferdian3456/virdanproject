package model

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserId uuid.UUID `json:"userId"`
	jwt.RegisteredClaims
}

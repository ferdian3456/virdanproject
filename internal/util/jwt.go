package util

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/ferdian3456/virdanproject/internal/constant"
	"github.com/ferdian3456/virdanproject/internal/model"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	BearerPrefix = "Bearer "
	// #nosec G101 -- TokenIssuer is a public identifier, not a credential
	TokenIssuer             = "github.com/ferdian3456/virdanproject"
	AccessTokenDuration     = 15 * time.Minute
	RefreshTokenDuration    = 7 * 24 * time.Hour
	ErrInvalidSigningMethod = errors.New("invalid token signing method")
)

// HashToken hashes a token using SHA256 for secure storage.
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// GenerateAccessToken signs a JWT access token for the given user id.
func GenerateAccessToken(userId string, jwtSecretKey string) (string, error) {
	if jwtSecretKey == "" {
		return "", errors.New("jwt secret key is not configured")
	}

	now := time.Now().UTC()
	claims := &model.Claims{
		UserId: userId,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenDuration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    TokenIssuer,
			Subject:   fmt.Sprintf("user:%s", userId),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(jwtSecretKey))
	if err != nil {
		return "", err
	}

	return signedToken, nil
}

// GenerateTokenPair creates both access and refresh tokens for a user.
func GenerateTokenPair(userId string, jwtSecretKey string) (model.TokenResponse, error) {
	accessToken, err := GenerateAccessToken(userId, jwtSecretKey)
	if err != nil {
		return model.TokenResponse{}, err
	}

	refreshToken := uuid.New().String()

	return model.TokenResponse{
		AccessToken:           accessToken,
		AccessTokenExpiresIn:  int(AccessTokenDuration.Seconds()),
		RefreshToken:          refreshToken,
		RefreshTokenExpiresIn: int(RefreshTokenDuration.Seconds()),
		TokenType:             "Bearer",
	}, nil
}

// ValidateAccessToken validates a JWT access token (without "Bearer " prefix)
// and returns the token string + user id. Caller MUST strip "Bearer " prefix first.
func ValidateAccessToken(tokenString string, jwtSecretKey string) (string, string, error) {
	if jwtSecretKey == "" {
		return "", "", errors.New("jwt secret key is not configured")
	}

	if tokenString == "" {
		return "", "", &model.UnauthorizedError{
			Code:    constant.ERR_UNAUTHORIZED_ERROR,
			Message: "Authentication token is empty",
			Param:   "accessToken",
		}
	}

	token, err := jwt.ParseWithClaims(tokenString, &model.Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidSigningMethod
		}
		return []byte(jwtSecretKey), nil
	})

	if err != nil {
		return "", "", handleAuthParseError(err)
	}

	claims, ok := token.Claims.(*model.Claims)
	if !ok || !token.Valid {
		return "", "", &model.UnauthorizedError{
			Code:    constant.ERR_UNAUTHORIZED_ERROR,
			Message: "Authentication token is invalid",
			Param:   "accessToken",
		}
	}

	return tokenString, claims.UserId, nil
}

// handleAuthParseError converts JWT parsing errors to UnauthorizedError.
func handleAuthParseError(err error) error {
	switch {
	case errors.Is(err, jwt.ErrTokenMalformed):
		return &model.UnauthorizedError{
			Code:    constant.ERR_UNAUTHORIZED_ERROR,
			Message: "Authentication token is malformed",
			Param:   "accessToken",
		}
	case errors.Is(err, jwt.ErrTokenExpired):
		return &model.UnauthorizedError{
			Code:    constant.ERR_UNAUTHORIZED_ERROR,
			Message: "Authentication token is expired",
			Param:   "accessToken",
		}
	case errors.Is(err, jwt.ErrTokenNotValidYet):
		return &model.UnauthorizedError{
			Code:    constant.ERR_UNAUTHORIZED_ERROR,
			Message: "Authentication token is not valid yet",
			Param:   "accessToken",
		}
	case errors.Is(err, ErrInvalidSigningMethod):
		return &model.UnauthorizedError{
			Code:    constant.ERR_UNAUTHORIZED_ERROR,
			Message: "Authentication token has invalid signing method",
			Param:   "accessToken",
		}
	default:
		return &model.UnauthorizedError{
			Code:    constant.ERR_UNAUTHORIZED_ERROR,
			Message: "Authentication token is invalid",
			Param:   "accessToken",
		}
	}
}

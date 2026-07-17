package shared

import (
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type ApiError interface {
	error
	StatusCode() int
	GetCode() string
	GetParam() string
}

type BadRequestError struct {
	Code    string
	Message string
	Param   string
}

func (e *BadRequestError) Error() string    { return e.Message }
func (e *BadRequestError) StatusCode() int  { return http.StatusBadRequest }
func (e *BadRequestError) GetCode() string  { return e.Code }
func (e *BadRequestError) GetParam() string { return e.Param }

type UnauthorizedError struct {
	Code    string
	Message string
	Param   string
}

func (e *UnauthorizedError) Error() string    { return e.Message }
func (e *UnauthorizedError) StatusCode() int  { return http.StatusUnauthorized }
func (e *UnauthorizedError) GetCode() string  { return e.Code }
func (e *UnauthorizedError) GetParam() string { return e.Param }

type ForbiddenError struct {
	Code    string
	Message string
	Param   string
}

func (e *ForbiddenError) Error() string    { return e.Message }
func (e *ForbiddenError) StatusCode() int  { return http.StatusForbidden }
func (e *ForbiddenError) GetCode() string  { return e.Code }
func (e *ForbiddenError) GetParam() string { return e.Param }

type NotFoundError struct {
	Code    string
	Message string
	Param   string
}

func (e *NotFoundError) Error() string    { return e.Message }
func (e *NotFoundError) StatusCode() int  { return http.StatusNotFound }
func (e *NotFoundError) GetCode() string  { return e.Code }
func (e *NotFoundError) GetParam() string { return e.Param }

type ConflictError struct {
	Code    string
	Message string
	Param   string
}

func (e *ConflictError) Error() string    { return e.Message }
func (e *ConflictError) StatusCode() int  { return http.StatusConflict }
func (e *ConflictError) GetCode() string  { return e.Code }
func (e *ConflictError) GetParam() string { return e.Param }

type ValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param,omitempty"`
}

type Claims struct {
	UserId string `json:"userId"`
	jwt.RegisteredClaims
}

type TokenResponse struct {
	AccessToken           string `json:"accessToken"`
	AccessTokenExpiresIn  int    `json:"accessTokenExpiresIn"`
	RefreshToken          string `json:"refreshToken"`
	RefreshTokenExpiresIn int    `json:"refreshTokenExpiresIn"`
	TokenType             string `json:"tokenType"`
}

type CursorPage[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
	HasMore    bool   `json:"hasMore"`
}

type ProfileAvatarImage struct {
	Id        string
	Bucket    string
	ObjectKey string
	MimeType  string
	Size      int64
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
}

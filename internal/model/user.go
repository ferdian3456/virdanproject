package model

import (
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
)

const (
	SignupStepStart       = "start_signup"
	SignupStepOTPVerified = "otp_verified"
	SignupStepUsernameSet = "username_set"
	SignupStepPasswordSet = "password_set"
)

type UserCreateRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UsernameUpdateRequest struct {
	Username string `json:"username"`
}

type FullnameUpdateRequest struct {
	Fullname string `json:"fullname"`
}

type UserSignupStartRequest struct {
	Email string `json:"email"`
}

type UserVerifyPasswordRequest struct {
	SessionId string `json:"sessionId"`
	Password  string `json:"password"`
}

type OTPTemplateData struct {
	OTP       string
	ExpiresIn int64
}

type UserVerifyOTPRequest struct {
	SessionId string `json:"sessionId"`
	OTP       string `json:"otp"`
}

type UserVerifyUsernameRequest struct {
	SessionId string `json:"sessionId"`
	Username  string `json:"username"`
}

type UserSignupStartResponse struct {
	SessionId    uuid.UUID `json:"sessionId"`
	OtpExpiresAt int64     `json:"otpExpiresAt"`
}
type UserResponse struct {
	Id             string    `json:"id"`
	Username       string    `json:"username"`
	Fullname       string    `json:"fullname"`
	Email          string    `json:"email"`
	AvatarImage    *string   `json:"avatarImage"`
	CreateDatetime time.Time `json:"createDatetime"`
	UpdateDatetime time.Time `json:"updateDatetime"`
}

type UserSignupStatus struct {
	SessionId uuid.UUID `json:"sessionId"`
	Step      string    `json:"step"`
}

type User struct {
	Id             uuid.UUID
	Username       string
	Fullname       string
	Bio            *string
	AvatarImageId  *uuid.UUID
	Email          string
	Password       string
	Settings       sonic.NoCopyRawMessage
	CreateDatetime time.Time
	UpdateDatetime time.Time
	CreateUserId   uuid.UUID
	UpdateUserId   uuid.UUID
}

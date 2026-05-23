package model

import (
	"time"

	"github.com/bytedance/sonic"
)

const (
	SignupStepStart       = "start_signup"
	SignupStepOTPVerified = "otp_verified"
	SignupStepPasswordSet = "password_set"
)

type UserCreateRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
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

type UserResendOTPRequest struct {
	SessionId string `json:"sessionId"`
}

type UserSignupStartResponse struct {
	SessionId    string `json:"sessionId"`
	OtpExpiresAt int64  `json:"otpExpiresAt"`
}

type UserResponse struct {
	Id        string                 `json:"id"`
	Email     string                 `json:"email"`
	Settings  sonic.NoCopyRawMessage `json:"settings" swaggertype:"object"`
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
}

type UserSignupStatus struct {
	SessionId string `json:"sessionId"`
	Step      string `json:"step"`
}

type User struct {
	Id        string
	Email     string
	Password  string
	Settings  []byte
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
	DeletedAt *time.Time
}

type UserVerifyCurrentPasswordRequest struct {
	Password string `json:"password"`
}

type UserChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type UserChangeEmailRequestRequest struct {
	NewEmail string `json:"newEmail"`
}

type UserChangeEmailConfirmRequest struct {
	OTP string `json:"otp"`
}

type UserChangeEmailRequestResponse struct {
	OtpExpiresAt int64 `json:"otpExpiresAt"`
}

type OTPSignupData struct {
	OTP       string `json:"otp"`
	ExpiresAt int64  `json:"expiresAt"`
}

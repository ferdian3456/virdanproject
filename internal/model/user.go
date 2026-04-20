package model

import (
	"time"

	"github.com/bytedance/sonic"
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

type BioUpdateRequest struct {
	Bio string `json:"bio"`
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

type UserVerifyUsernameRequest struct {
	SessionId string `json:"sessionId"`
	Username  string `json:"username"`
}

type UserSignupStartResponse struct {
	SessionId    string `json:"sessionId"`
	OtpExpiresAt int64     `json:"otpExpiresAt"`
}
type UserResponse struct {
	Id        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type UserSignupStatus struct {
	SessionId string `json:"sessionId"`
	Step      string    `json:"step"`
}

type User struct {
	Id       string
	Username string
	Email    string
	Password string
	Settings sonic.NoCopyRawMessage
	Audit
}

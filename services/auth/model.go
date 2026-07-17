package auth

import "time"

const (
	SignupStepStart       = "start_signup"
	SignupStepOTPVerified = "otp_verified"
	SignupStepPasswordSet = "password_set"
)

type UserLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserSignupStartRequest struct {
	Email string `json:"email"`
}

type UserSignupStartResponse struct {
	SessionId    string `json:"sessionId"`
	OtpExpiresAt int64  `json:"otpExpiresAt"`
}

type UserVerifyOTPRequest struct {
	SessionId string `json:"sessionId"`
	OTP       string `json:"otp"`
}

type UserResendOTPRequest struct {
	SessionId string `json:"sessionId"`
}

type UserVerifyPasswordRequest struct {
	SessionId string `json:"sessionId"`
	Password  string `json:"password"`
}

type UserSignupStatus struct {
	SessionId string `json:"sessionId"`
	Step      string `json:"step"`
}

type OTPSignupData struct {
	OTP       string `json:"otp"`
	ExpiresAt int64  `json:"expiresAt"`
}

type OTPTemplateData struct {
	OTP       string
	ExpiresIn int64
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
}

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

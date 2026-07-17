package user

import (
	"time"

	"github.com/bytedance/sonic"
)

type UserResponse struct {
	Id        string                 `json:"id"`
	Email     string                 `json:"email"`
	Settings  sonic.NoCopyRawMessage `json:"settings" swaggertype:"object"`
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
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

type UserChangeEmailRequestResponse struct {
	OtpExpiresAt int64 `json:"otpExpiresAt"`
}

type UserChangeEmailConfirmRequest struct {
	OTP string `json:"otp"`
}

type OTPTemplateData struct {
	OTP       string
	ExpiresIn int64
}

type NotificationPrefs struct {
	NotifLike    bool
	NotifComment bool
	NotifReply   bool
}

type UpdateNotificationPreferencesRequest struct {
	NotifLike    bool `json:"notifLike"`
	NotifComment bool `json:"notifComment"`
	NotifReply   bool `json:"notifReply"`
}

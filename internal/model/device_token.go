package model

import "time"

type DeviceToken struct {
	Id        string
	UserId    string
	Token     string
	Platform  string
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
}

// after login/finish signup
type DeviceTokenRegisterRequest struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

// when logout
type DeviceTokenDeleteRequest struct {
	Token string `json:"token"`
}

type PushPayload struct {
	Title string            // notification title, ex "Hi, new promo is coming"
	Body  string            // notification body, ex "Please check out our latest promo, its good i swear"
	Data  map[string]string // to help fe/app identify the notification for deep link, ex: i click the notification and then app will route to mention page
}

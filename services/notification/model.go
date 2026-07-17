package notification

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

type DeviceTokenRegisterRequest struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

type DeviceTokenDeleteRequest struct {
	Token string `json:"token"`
}

type Notification struct {
	Id              string
	RecipientUserId string
	ActorUserId     string
	ActorProfileId  string
	Type            string
	ServerId        string
	PostId          *string
	CommentId       *string
	ReadAt          *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CreatedBy       string
	UpdatedBy       string
}

type NotificationEvent struct {
	Type            string
	RecipientUserId string
	ActorUserId     string
	ActorProfileId  string
	ServerId        string
	PostId          string
	CommentId       *string
}

type NotificationResponse struct {
	Id             string     `json:"id"`
	Type           string     `json:"type"`
	ActorUsername  string     `json:"actorUsername"`
	ActorAvatarUrl *string    `json:"actorAvatarUrl"`
	ServerId       string     `json:"serverId"`
	PostId         *string    `json:"postId"`
	CommentId      *string    `json:"commentId"`
	ReadAt         *time.Time `json:"readAt"`
	CreatedAt      time.Time  `json:"createdAt"`
}

type NotificationListResponse struct {
	Data []NotificationResponse `json:"data"`
	Page Page                   `json:"page"`
}

type Page struct {
	NextCursor string `json:"nextCursor"`
}

type UnreadCountResponse struct {
	Count int `json:"count"`
}

type NotificationCursor struct {
	CreatedAt time.Time `json:"createdAt"`
	Id        string    `json:"id"`
}

type NotificationPrefs struct {
	NotifLike    bool
	NotifComment bool
	NotifReply   bool
}

package model

import "time"

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

// NotificationEvent is the intermediate work unit from post_usecase to Notify(). Not stored
// directly — used for per-recipient dedup, then turned into a Notification row.
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

type UpdateNotificationPreferencesRequest struct {
	NotifLike    bool `json:"notifLike"`
	NotifComment bool `json:"notifComment"`
	NotifReply   bool `json:"notifReply"`
}

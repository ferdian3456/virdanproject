package model

import (
	"time"

)

type ServerInvites struct {
	Id              string
	ServerId        string
	Code            string
	MaxUses         int
	UsedCount       int
	ExpiresDatetime time.Time
	IsActive        bool
	Audit
}

type ServerInviteLinkRequest struct {
	ExpiresInMinutes int `json:"expiresInMinutes"`
	MaxUses          int `json:"maxUses"`
}

type ServerJoinRequest struct {
	InviteCode    string     `json:"inviteCode"`
	Nickname      string     `json:"nickname"`
	Bio           *string    `json:"bio"`
	AvatarImageId *string `json:"avatarImageId"`
}

type ServerInviteLinkResponse struct {
	InviteCode string    `json:"inviteCode"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

type ServerInfoForInviteResponse struct {
	OwnerName     string  `json:"ownerName"`
	ServerName    string  `json:"serverName"`
	Description   *string `json:"description"`
	AvatarImageId *string `json:"avatarImageId"`
	BannerImageId *string `json:"bannerImageId"`
}

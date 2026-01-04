package model

import (
	"time"

	"github.com/google/uuid"
)

type ServerInvites struct {
	Id              uuid.UUID
	ServerId        uuid.UUID
	Code            string
	MaxUses         int
	UsedCount       int
	ExpiresDatetime time.Time
	IsActive        bool
	CreateDatetime  time.Time
	UpdateDatetime  time.Time
	CreateUserId    uuid.UUID
	UpdateUserId    uuid.UUID
}

type ServerInviteLinkRequest struct {
	ExpiresInMinutes int `json:"expiresInMinutes"`
	MaxUses          int `json:"maxUses"`
}

type ServerJoinRequest struct {
	InviteCode string `json:"inviteCode"`
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

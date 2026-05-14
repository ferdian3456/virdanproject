package model

import (
	"time"
)

type ServerMemberProfile struct {
	Id            string
	ServerId      string
	UserId        string
	Nickname      string
	Bio           *string
	AvatarImageId *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CreatedBy     string
	UpdatedBy     string
}

type ServerMemberProfileResponse struct {
	ProfileId      string    `json:"profileId"`
	ServerId       string    `json:"serverId"`
	Nickname       string    `json:"nickname"`
	Bio            *string   `json:"bio"`
	AvatarImageId  *string   `json:"avatarImageId"`
	AvatarImageUrl *string   `json:"avatarImageUrl"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type GetProfileHistoryResponseItem struct {
	ProfileId      string    `json:"profileId"`
	ServerId       string    `json:"serverId"`
	ServerName     string    `json:"serverName"`
	Nickname       string    `json:"nickname"`
	Bio            *string   `json:"bio"`
	AvatarImageId  *string   `json:"avatarImageId"`
	AvatarImageUrl *string   `json:"avatarImageUrl"`
	IsStillMember  bool      `json:"isStillMember"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type GetProfileHistoryResponse struct {
	Data []GetProfileHistoryResponseItem `json:"data"`
}

type ServerProfileUpdateRequest struct {
	Nickname string  `json:"nickname"`
	Bio      *string `json:"bio"`
}

type ServerProfileUpdateResponse struct {
	ProfileId string    `json:"profileId"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type IdentityInputRequest struct {
	Nickname      string  `json:"nickname"`
	Bio           *string `json:"bio"`
	AvatarImageId *string `json:"avatarImageId"`
}

type ProfileUpdateRequest struct {
	Nickname string  `json:"nickname"`
	Bio      *string `json:"bio"`
}

type AuthorIdentityResponse struct {
	UserId         string       `json:"userId"`
	Nickname       string       `json:"nickname"`
	AvatarImageUrl *string      `json:"avatarImageUrl"`
	Status         AuthorStatus `json:"status"`
}

package model

import (
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
)

type Server struct {
	Id             uuid.UUID
	OwnerId        uuid.UUID
	Name           string
	ShortName      string
	CategoryId     *int
	AvatarImageId  *uuid.UUID
	BannerImageId  *uuid.UUID
	Description    *string
	Settings       sonic.NoCopyRawMessage
	CreateDatetime time.Time
	UpdateDatetime time.Time
	CreateUserId   uuid.UUID
	UpdateUserId   uuid.UUID
}

type ServerCreateRequest struct {
	Name        string                      `json:"name"`
	ShortName   string                      `json:"shortName"`
	CategoryId  *int                        `json:"categoryId"`
	Description *string                     `json:"description"`
	Settings    ServerSettingsCreateRequest `json:"settings"`
}

type ServerCreateResponse struct {
	Id             uuid.UUID              `json:"id"`
	OwnerId        uuid.UUID              `json:"ownerId"`
	Name           string                 `json:"name"`
	ShortName      string                 `json:"shortName"`
	CategoryId     *int                   `json:"categoryId"`
	Description    *string                `json:"description"`
	Settings       sonic.NoCopyRawMessage `json:"settings"`
	CreateDatetime time.Time              `json:"createDatetime"`
	UpdateDatetime time.Time              `json:"updateDatetime"`
	CreateUserId   uuid.UUID              `json:"createUserId"`
	UpdateUserId   uuid.UUID              `json:"updateUserId"`
}

type ServerUpdateResponse struct {
	Id             uuid.UUID              `json:"id"`
	OwnerId        uuid.UUID              `json:"ownerId"`
	Name           string                 `json:"name"`
	ShortName      string                 `json:"shortName"`
	CategoryId     *int                   `json:"categoryId"`
	Description    *string                `json:"description"`
	Settings       sonic.NoCopyRawMessage `json:"settings"`
	CreateDatetime time.Time              `json:"createDatetime"`
	UpdateDatetime time.Time              `json:"updateDatetime"`
	CreateUserId   uuid.UUID              `json:"createUserId"`
	UpdateUserId   uuid.UUID              `json:"updateUserId"`
}

type ServerSettingsCreateRequest struct {
	IsPrivate bool `json:"isPrivate"`
}

type DiscoveryServerResponse struct {
	Data []ServerInfoResponse `json:"data"`
	Page Page                 `json:"page"`
}

type ServerDiscoveryCursor struct {
	Id             string    `json:"id"`
	CreateDatetime time.Time `json:"createDatetime"`
}

type ServerInfoResponse struct {
	Id             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	ShortName      string    `json:"shortName"`
	CategoryName   string    `json:"categoryName"`
	AvatarImageUrl *string   `json:"avatarImageUrl"`
	BannerImageUrl *string   `json:"bannerImageUrl"`
	Description    *string   `json:"description"`
	CreateDatetime time.Time `json:"-"` // tidak di-serialize ke JSON, hanya untuk cursor
}

type ServerUserListResponse struct {
	Data []ServerUserResponse `json:"data"`
	Page Page                 `json:"page"`
}

type ServerUserCursor struct {
	ServerId       string    `json:"serverId"`
	JoinedDatetime time.Time `json:"joinedDatetime"`
}

type ServerUserResponse struct {
	Id             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	ShortName      string    `json:"shortName"`
	AvatarImageUrl *string   `json:"avatarImageUrl"`
	JoinedDatetime time.Time `json:"-"` // tidak di-serialize ke JSON, hanya untuk cursor
}

type ServerResponse struct {
	Id             uuid.UUID `json:"id"`
	OwnerName      string    `json:"ownerName"`
	Name           string    `json:"name"`
	ShortName      string    `json:"shortName"`
	CategoryName   string    `json:"categoryName"`
	AvatarImageUrl *string   `json:"avatarImageUrl"`
	BannerImageUrl *string   `json:"bannerImageUrl"`
	Description    *string   `json:"description"`
}

type ServerUpdateNameRequest struct {
	Name string `json:"name"`
}

type ServerUpdateShortNameRequest struct {
	ShortName string `json:"shortName"`
}

type ServerUpdateCategoryRequest struct {
	CategoryId *int `json:"categoryId"`
}

type ServerUpdateDescriptionRequest struct {
	Description *string `json:"description"`
}

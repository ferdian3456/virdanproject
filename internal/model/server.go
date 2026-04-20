package model

import (
	"time"

	"github.com/bytedance/sonic"
)

type Server struct {
	Id            string
	OwnerId       string
	Name          string
	ShortName     string
	CategoryId    *int
	AvatarImageId *string
	BannerImageId *string
	Description   *string
	Settings      sonic.NoCopyRawMessage
	Audit
}

type ServerCreateRequest struct {
	Name        string                      `json:"name"`
	ShortName   string                      `json:"shortName"`
	CategoryId  *int                        `json:"categoryId"`
	Description *string                     `json:"description"`
	Settings    ServerSettingsCreateRequest `json:"settings"`
}

type ServerCreateResponse struct {
	Id             string              `json:"id"`
	OwnerId        string              `json:"ownerId"`
	Name           string                 `json:"name"`
	ShortName      string                 `json:"shortName"`
	CategoryId     *int                   `json:"categoryId"`
	Description    *string                `json:"description"`
	Settings       sonic.NoCopyRawMessage `json:"settings" swaggertype:"object"`
	CreateDatetime time.Time              `json:"createDatetime"`
	UpdateDatetime time.Time              `json:"updateDatetime"`
	CreateUserId   string              `json:"createUserId"`
	UpdateUserId   string              `json:"updateUserId"`
}

type ServerUpdateResponse struct {
	Id             string              `json:"id"`
	OwnerId        string              `json:"ownerId"`
	Name           string                 `json:"name"`
	ShortName      string                 `json:"shortName"`
	CategoryId     *int                   `json:"categoryId"`
	AvatarImageId  *string             `json:"avatarImageId"`
	BannerImageId  *string             `json:"bannerImageId"`
	Description    *string                `json:"description"`
	Settings       sonic.NoCopyRawMessage `json:"settings" swaggertype:"object"`
	CreateDatetime time.Time              `json:"createDatetime"`
	UpdateDatetime time.Time              `json:"updateDatetime"`
	CreateUserId   string              `json:"createUserId"`
	UpdateUserId   string              `json:"updateUserId"`
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
	Id             string `json:"id"`
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
	Id             string `json:"id"`
	Name           string    `json:"name"`
	ShortName      string    `json:"shortName"`
	AvatarImageUrl *string   `json:"avatarImageUrl"`
	JoinedDatetime time.Time `json:"-"` // tidak di-serialize ke JSON, hanya untuk cursor
}

type ServerResponse struct {
	Id             string `json:"id"`
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

type ServerCategoryListResponse struct {
	Data []ServerCategoryResponse `json:"data"`
	Page Page                     `json:"page"`
}

type ServerCategoryResponse struct {
	Id           int    `json:"id"`
	CategoryName string `json:"categoryName"`
}

type ServerCategoryCursor struct {
	Id int `json:"id"`
}

type ServerDetailResponse struct {
	Id             string `json:"id"`
	Name           string    `json:"name"`
	ShortName      string    `json:"shortName"`
	CategoryName   string    `json:"categoryName"`
	AvatarImageUrl *string   `json:"avatarImageUrl"`
	BannerImageUrl *string   `json:"bannerImageUrl"`
	Description    *string   `json:"description"`
	CreateDatetime time.Time `json:"createDatetime"`
	CreatedBy      string    `json:"createdBy"`
	IsPrivate      *bool     `json:"-"` // Internal use only, not exposed to API response
}

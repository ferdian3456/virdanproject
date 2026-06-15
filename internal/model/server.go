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
	Settings      []byte
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CreatedBy     string
	UpdatedBy     string
}

type ServerSettings struct {
	IsPrivate bool `json:"isPrivate"`
}

type ServerCheckEligibleInfo struct {
	Exists    bool
	IsPrivate bool
}

// --- Request structs ---

type ServerCreateRequest struct {
	Name        string                      `json:"name"`
	ShortName   string                      `json:"shortName"`
	CategoryId  *int                        `json:"categoryId"`
	Description *string                     `json:"description"`
	Settings    ServerSettingsCreateRequest `json:"settings"`
}

type ServerSettingsCreateRequest struct {
	IsPrivate bool `json:"isPrivate"`
}

type ServerUpdateNameRequest struct {
	Name string `json:"name"`
}

type ServerUpdateShortNameRequest struct {
	ShortName string `json:"shortName"`
}

type ServerUpdateCategoryRequest struct {
	CategoryId int `json:"categoryId"`
}

type ServerUpdateDescriptionRequest struct {
	Description string `json:"description"`
}

type ServerUpdateSettingsRequest struct {
	IsPrivate bool `json:"isPrivate"`
}

type ServerJoinByInviteRequest struct {
	InviteCode    string  `json:"inviteCode"`
	Nickname      string  `json:"nickname"`
	Username      string  `json:"username"`
	Bio           *string `json:"bio"`
	AvatarImageId *string `json:"avatarImageId"`
}

type ServerInviteLinkRequest struct {
	MaxUses   int        `json:"maxUses"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

// --- Response structs ---

type ServerCreateResponse struct {
	Server   ServerDetailResponse        `json:"server"`
	Identity ServerMemberProfileResponse `json:"identity"`
}

type ServerUpdateResponse struct {
	Id        string    `json:"id"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ServerDetailResponse struct {
	Id            string                 `json:"id"`
	OwnerId       string                 `json:"ownerId"`
	OwnerNickname string                 `json:"ownerNickname"`
	Name          string                 `json:"name"`
	ShortName     string                 `json:"shortName"`
	CategoryId    *int                   `json:"categoryId"`
	CategoryName  *string                `json:"categoryName"`
	AvatarUrl     *string                `json:"avatarUrl"`
	BannerUrl     *string                `json:"bannerUrl"`
	Description   *string                `json:"description"`
	Settings      sonic.NoCopyRawMessage `json:"settings" swaggertype:"object"`
	MemberCount   int                    `json:"memberCount"`
	IsMember      bool                   `json:"isMember"`
	PlusActive    bool                   `json:"plusActive"`
	PlusExpiresAt *time.Time             `json:"plusExpiresAt"`
	CreatedAt     time.Time              `json:"createdAt"`
	UpdatedAt     time.Time              `json:"updatedAt"`
}

type ServerInfoResponse struct {
	Id           string    `json:"id"`
	Name         string    `json:"name"`
	ShortName    string    `json:"shortName"`
	CategoryId   *int      `json:"categoryId"`
	CategoryName *string   `json:"categoryName"`
	AvatarUrl    *string   `json:"avatarUrl"`
	BannerUrl    *string   `json:"bannerUrl"`
	MemberCount  int       `json:"memberCount"`
	IsMember     bool      `json:"isMember"`
	Description  *string   `json:"description"`
	CreatedAt    time.Time `json:"createdAt"`
}

type ServerUserResponse struct {
	Id           string    `json:"id"`
	Name         string    `json:"name"`
	ShortName    string    `json:"shortName"`
	AvatarUrl    *string   `json:"avatarUrl"`
	CategoryId   *int      `json:"categoryId"`
	CategoryName *string   `json:"categoryName"`
	MemberCount  int       `json:"memberCount"`
	JoinedAt     time.Time `json:"joinedAt"`
	MyNickname   string    `json:"myNickname"`
	MyAvatarUrl  *string   `json:"myAvatarUrl"`
}

type ServerInfoForInviteResponse struct {
	Code            string     `json:"code"`
	ServerId        string     `json:"serverId"`
	ServerName      string     `json:"serverName"`
	ServerAvatarUrl *string    `json:"serverAvatarUrl"`
	OwnerNickname   string     `json:"ownerNickname"`
	MemberCount     int        `json:"memberCount"`
	ExpiresAt       *time.Time `json:"expiresAt"`
}

type ServerInviteLinkResponse struct {
	Code      string     `json:"code"`
	InviteUrl string     `json:"inviteUrl"`
	MaxUses   int        `json:"maxUses"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

// --- List/pagination response wrappers ---

type DiscoveryServerResponse struct {
	Data []ServerInfoResponse `json:"data"`
	Page Page                 `json:"page"`
}

type ServerDiscoveryCursor struct {
	Id        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
}

type ServerUserListResponse struct {
	Data []ServerUserResponse `json:"data"`
	Page Page                 `json:"page"`
}

type ServerUserCursor struct {
	ServerId string    `json:"serverId"`
	JoinedAt time.Time `json:"joinedAt"`
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

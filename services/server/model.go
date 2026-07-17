package server

import (
	"time"

	"github.com/bytedance/sonic"
)

type Page struct {
	NextCursor string `json:"nextCursor"`
}

const (
	OwnerRole  = "Owner"
	AdminRole  = "Admin"
	MemberRole = "Member"
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

type ServerRole struct {
	Id          string
	ServerId    string
	Name        string
	Permissions []byte
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedBy   string
	UpdatedBy   string
}

type ServerMember struct {
	Id           string
	ServerId     string
	UserId       string
	ServerRoleId string
	JoinedAt     time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CreatedBy    string
	UpdatedBy    string
}

type ServerInvite struct {
	Id        string
	ServerId  string
	Code      string
	MaxUses   int
	UsedCount int
	ExpiresAt *time.Time
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
}

type ServerAvatarImage struct {
	Id        string
	Bucket    string
	ObjectKey string
	MimeType  string
	Size      int64
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
}

type ServerBannerImage struct {
	Id        string
	Bucket    string
	ObjectKey string
	MimeType  string
	Size      int64
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
}

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

type AssignMemberRoleRequest struct {
	Role string `json:"role"`
}

type TransferOwnershipRequest struct {
	NewOwnerId string `json:"newOwnerId"`
}

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

type ServerMemberItem struct {
	UserId    string    `json:"userId"`
	Role      string    `json:"role"`
	Nickname  string    `json:"nickname"`
	Username  string    `json:"username"`
	AvatarUrl *string   `json:"avatarUrl"`
	JoinedAt  time.Time `json:"joinedAt"`
}

type ServerMemberListResponse struct {
	Data []ServerMemberItem `json:"data"`
	Page Page               `json:"page"`
}

type ServerMemberCursor struct {
	JoinedAt time.Time `json:"joinedAt"`
	UserId   string    `json:"userId"`
}

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

type ServerMemberProfile struct {
	Id            string
	ServerId      string
	UserId        string
	Nickname      string
	Username      string
	Bio           *string
	AvatarImageId *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CreatedBy     string
	UpdatedBy     string
}

type ServerMemberProfileResponse struct {
	ProfileId     string    `json:"profileId"`
	ServerId      string    `json:"serverId"`
	Nickname      string    `json:"nickname"`
	Username      string    `json:"username"`
	Bio           *string   `json:"bio"`
	AvatarImageId *string   `json:"avatarImageId"`
	AvatarUrl     *string   `json:"avatarUrl"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type GetProfileHistoryResponseItem struct {
	ProfileId     string    `json:"profileId"`
	ServerId      string    `json:"serverId"`
	ServerName    string    `json:"serverName"`
	Nickname      string    `json:"nickname"`
	Username      string    `json:"username"`
	Bio           *string   `json:"bio"`
	AvatarImageId *string   `json:"avatarImageId"`
	AvatarUrl     *string   `json:"avatarUrl"`
	IsStillMember bool      `json:"isStillMember"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type GetProfileHistoryResponse struct {
	Data []GetProfileHistoryResponseItem `json:"data"`
}

type ServerProfileUpdateResponse struct {
	ProfileId string    `json:"profileId"`
	UpdatedAt time.Time `json:"updatedAt"`
}

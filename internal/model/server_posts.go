package model

import (
	"time"
)

type ServerPost struct {
	Id          string
	ServerId    string
	AuthorId    string
	PostImageId *string
	Caption     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedBy   string
	UpdatedBy   string
}

type ServerPostUpdateCaptionRequest struct {
	Caption string `json:"caption"`
}

type ServerPostCursor struct {
	Id        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
}

type ServerPostListResponse struct {
	Data []ServerPostResponse `json:"data"`
	Page Page                 `json:"page"`
}

type ServerPostResponse struct {
	Id           string                `json:"id"`
	ServerId     string                `json:"serverId"`
	Caption      string                `json:"caption"`
	ImageUrl     *string               `json:"imageUrl"`
	Author       AuthorIdentityResponse `json:"author"`
	LikeCount    int                   `json:"likeCount"`
	CommentCount int                   `json:"commentCount"`
	UserLiked    bool                  `json:"userLiked"`
	UserSaved    bool                  `json:"userSaved"`
	SavedAt      *time.Time            `json:"savedAt,omitempty"`
	IsOwner      bool                  `json:"isOwner"`
	CreatedAt    time.Time             `json:"createdAt"`
	UpdatedAt    time.Time             `json:"updatedAt"`
}

type PostLikeResponse struct {
	PostId    string `json:"postId"`
	UserLiked bool   `json:"userLiked"`
	LikeCount int    `json:"likeCount"`
}

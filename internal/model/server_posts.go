package model

import (
	"time"

)

type ServerPosts struct {
	Id          string
	ServerId    string
	AuthorId    string
	PostImageId string
	Caption     string
	Audit
}

type ServerPostUpdateCaptionRequest struct {
	Caption string `json:"caption"`
}

type ServerPostCursor struct {
	Id             string `json:"id"`
	CreateDatetime time.Time `json:"createDatetime"`
}

type ServerPostListResponse struct {
	Data []ServerPostResponse `json:"data"`
	Page Page                 `json:"page"`
}

type ServerPostResponse struct {
	OwnerId        string `json:"ownerId"`
	OwnerName      string    `json:"ownerName"`
	OwnerImageUrl  *string   `json:"ownerImageUrl"`
	PostId         string `json:"postId"`
	PostImageUrl   string    `json:"postImageUrl"`
	Caption        string    `json:"caption"`
	CommentCount   int       `json:"commentCount"`
	LikeCount      int       `json:"likeCount"`
	IsLiked        bool      `json:"isLiked"`
	CreateDatetime time.Time `json:"createDatetime"`
	UpdateDatetime time.Time `json:"updateDatetime"`
}

// PostLikeResponse represents response after like/unlike operation
type PostLikeResponse struct {
	LikeCount int `json:"likeCount"`
}

type ServerPostForMeResponse struct {
	Data []ServerPostForMe `json:"data"`
	Page Page              `json:"page"`
}

type ServerPostForMe struct {
	PostId         string `json:"postId"`
	PostImageUrl   string    `json:"postImageUrl"`
	CreateDatetime time.Time `json:"createDatetime"`
}

package model

import (
	"time"
)

type ServerPostComment struct {
	Id        string
	PostId    string
	AuthorId  string
	ParentId  *string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
}

type ServerCommentCreateRequest struct {
	Content  string  `json:"content"`
	ParentId *string `json:"parentId"`
}

type ServerCommentCursor struct {
	Id        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
}

type ServerCommentListResponse struct {
	Data []ServerCommentResponse `json:"data"`
	Page Page                    `json:"page"`
}

type ServerCommentResponse struct {
	Id        string                `json:"id"`
	PostId    string                `json:"postId"`
	ParentId  *string               `json:"parentId"`
	Content   string                `json:"content"`
	Author    AuthorIdentityResponse `json:"author"`
	IsOwner   bool                  `json:"isOwner"`
	CreatedAt time.Time             `json:"createdAt"`
	UpdatedAt time.Time             `json:"updatedAt"`
}

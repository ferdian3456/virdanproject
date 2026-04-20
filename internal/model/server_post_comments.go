package model

import (
	"time"

)

type ServerPostComments struct {
	Id             string
	PostId         string
	AuthorId       string
	ParentId       *string
	Content        string
	Audit
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
	Id             string  `json:"id"`
	AuthorId       string  `json:"authorId"`
	AuthorName     string     `json:"authorName"`
	AuthorAvatar   *string    `json:"authorAvatar"`
	ParentId       *string `json:"parentId"`
	Content  string `json:"content"`
	Audit
}

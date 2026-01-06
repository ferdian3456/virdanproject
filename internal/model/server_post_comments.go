package model

import (
	"time"

	"github.com/google/uuid"
)

type ServerPostComments struct {
	Id             uuid.UUID
	PostId         uuid.UUID
	AuthorId       uuid.UUID
	ParentId       *uuid.UUID
	Content        string
	CreateDatetime time.Time
	UpdateDatetime time.Time
	CreateUserId   uuid.UUID
	UpdateUserId   uuid.UUID
}

type ServerCommentCreateRequest struct {
	Content  string    `json:"content"`
	ParentId *uuid.UUID `json:"parentId"`
}

type ServerCommentCursor struct {
	Id             uuid.UUID `json:"id"`
	CreateDatetime time.Time `json:"createDatetime"`
}

type ServerCommentListResponse struct {
	Data []ServerCommentResponse `json:"data"`
	Page Page                      `json:"page"`
}

type ServerCommentResponse struct {
	Id             uuid.UUID  `json:"id"`
	AuthorId       uuid.UUID  `json:"authorId"`
	ParentId       *uuid.UUID `json:"parentId"`
	Content        string     `json:"content"`
	CreateDatetime time.Time  `json:"createDatetime"`
	UpdateDatetime time.Time  `json:"updateDatetime"`
}

package model

import (
	"time"

	"github.com/google/uuid"
)

type ServerPosts struct {
	Id             uuid.UUID
	ServerId       uuid.UUID
	AuthorId       uuid.UUID
	PostImageId    uuid.UUID
	Caption        string
	CreateDatetime time.Time
	UpdateDatetime time.Time
	CreateUserId   uuid.UUID
	UpdateUserId   uuid.UUID
}

type ServerPostUpdateCaptionRequest struct {
	Caption string `json:"caption"`
}

type ServerPostCursor struct {
	Id             uuid.UUID `json:"id"`
	CreateDatetime time.Time `json:"createDatetime"`
}

type ServerPostListResponse struct {
	Data []ServerPostResponse `json:"data"`
	Page Page                 `json:"page"`
}

type ServerPostResponse struct {
	OwnerId        uuid.UUID  `json:"ownerId"`
	PostId         uuid.UUID  `json:"postId"`
	PostImageUrl   string     `json:"postImageUrl"`
	Caption        string     `json:"caption"`
	CreateDatetime time.Time  `json:"createDatetime"`
	UpdateDatetime time.Time  `json:"updateDatetime"`
}

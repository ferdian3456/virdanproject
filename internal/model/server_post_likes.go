package model

import (
	"time"
)

type ServerPostLike struct {
	Id        string
	PostId    string
	UserId    string
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
}

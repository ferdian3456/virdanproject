package model

import (
	"time"
)

type ProfileAvatarImage struct {
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

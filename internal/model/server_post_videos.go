package model

import (
	"time"
)

type ServerPostVideo struct {
	Id                 string
	Bucket             string
	ObjectKey          string
	MimeType           string
	Size               int64
	Duration           int
	Width              int
	Height             int
	ThumbnailObjectKey string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	CreatedBy          string
	UpdatedBy          string
}

package model

import (
	"time"

)

type ServerPostImages struct {
	Id             string
	Bucket         string
	ObjectKey      string
	MimeType       string
	Size           int64
	Audit
}

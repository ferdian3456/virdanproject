package model

import (
	"time"

)

type ServerPostLikes struct {
	Id             string
	PostId         string
	UserId         string
	Audit
}

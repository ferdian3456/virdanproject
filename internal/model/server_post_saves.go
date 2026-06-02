package model

import (
	"time"
)

type ServerPostSave struct {
	Id        string
	PostId    string
	UserId    string
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy string
	UpdatedBy string
}

type PostSaveResponse struct {
	PostId    string `json:"postId"`
	UserSaved bool   `json:"userSaved"`
}

// SavedPostCursor paginates the saved feed by save time (server_post_saves.created_at),
// newest-save-first — not by post creation time.
type SavedPostCursor struct {
	SavedAt time.Time `json:"savedAt"`
	PostId  string    `json:"postId"`
}

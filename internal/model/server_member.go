package model

import (
	"time"
)

type ServerMember struct {
	Id           string
	ServerId     string
	UserId       string
	ServerRoleId string
	JoinedAt     time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CreatedBy    string
	UpdatedBy    string
}

type AssignMemberRoleRequest struct {
	Role string `json:"role"` // "Admin" | "Member"
}

type TransferOwnershipRequest struct {
	NewOwnerId string `json:"newOwnerId"`
}

type ServerMemberItem struct {
	UserId    string    `json:"userId"`
	Role      string    `json:"role"`
	Nickname  string    `json:"nickname"`
	Username  string    `json:"username"`
	AvatarUrl *string   `json:"avatarUrl"`
	JoinedAt  time.Time `json:"joinedAt"`
}

type ServerMemberListResponse struct {
	Data []ServerMemberItem `json:"data"`
	Page Page               `json:"page"`
}

type ServerMemberCursor struct {
	JoinedAt time.Time `json:"joinedAt"`
	UserId   string    `json:"userId"`
}

package model

type AuthorStatus string

const (
	AuthorStatusActive      AuthorStatus = "active"
	AuthorStatusUserLeft    AuthorStatus = "user_left"
	AuthorStatusUserDeleted AuthorStatus = "user_deleted"
)

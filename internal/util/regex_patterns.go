package util

import "regexp"

var (
	nicknameRegex     = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	usernameRegex     = regexp.MustCompile(`^[a-zA-Z0-9_.]+$`)
	UsernameRegex     = usernameRegex
	UsernameErrorText = "Username may only contain letters, digits, underscores and dots"
)

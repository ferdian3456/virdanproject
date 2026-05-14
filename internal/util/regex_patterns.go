package util

import "regexp"

var (
	nicknameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

package util

import "strings"

func GenerateShortName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) == 0 {
		return ""
	}

	// ambil max 12 char untuk UI comfort
	runes := []rune(name)
	if len(runes) > 12 {
		return string(runes[:12])
	}

	return name
}

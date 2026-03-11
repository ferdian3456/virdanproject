package util

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

func GenerateOTP() (string, error) {
	const digits = "0123456789"
	const length = 6

	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	for i := range b {
		b[i] = digits[b[i]%10]
	}

	return string(b), nil
}

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

func FormatRemainingTime(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%d seconds", seconds)
	}
	minutes := seconds / 60
	remainingSecs := seconds % 60
	if remainingSecs == 0 {
		return fmt.Sprintf("%d minutes", minutes)
	}
	return fmt.Sprintf("%d minutes and %d seconds", minutes, remainingSecs)
}

func GenerateInviteCode() (string, error) {
	const digits = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 8
	b := make([]byte, length)

	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		b[i] = digits[num.Int64()]
	}
	return string(b), nil
}

func HashSHA256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

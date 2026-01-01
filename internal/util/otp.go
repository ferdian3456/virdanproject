package util

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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

func HashSHA256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

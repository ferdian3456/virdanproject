package util

import (
	"encoding/base64"

	"github.com/bytedance/sonic"
)

// EncodeCursor serializes a cursor value to a base64-encoded JSON string.
func EncodeCursor[T any](cursor T) string {
	b, err := sonic.Marshal(cursor)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

// DecodeCursor deserializes a base64-encoded JSON cursor string into T.
func DecodeCursor[T any](encoded string) (*T, error) {
	b, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	var cursor T
	if err = sonic.Unmarshal(b, &cursor); err != nil {
		return nil, err
	}
	return &cursor, nil
}

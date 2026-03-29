package setup

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// REDIS HELPERS FOR TEST MANIPULATION
// These helpers allow direct Redis access for testing edge cases
// ============================================================================

// GetTestRedisClient creates a Redis client for testing
// Use this to manipulate Redis data directly in tests
//
// Example:
//
//	redisClient := setup.GetTestRedisClient(t, redisURL)
//	defer redisClient.Close()
func GetTestRedisClient(t *testing.T, redisURL string) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: redisURL,
		DB:   0,
	})

	// Test connection
	ctx := context.Background()
	err := client.Ping(ctx).Err()
	require.NoError(t, err, "redis client should connect")

	return client
}

// DeleteSignupSession completely deletes a signup session from Redis
// This simulates session expiration
//
// Example:
//
//	setup.DeleteSignupSession(t, redisURL, sessionId)
func DeleteSignupSession(t *testing.T, redisURL, sessionId string) {
	client := GetTestRedisClient(t, redisURL)
	defer client.Close()

	ctx := context.Background()
	err := client.Del(ctx, "signup:"+sessionId).Err()
	require.NoError(t, err, "should delete signup session")
	t.Logf("Deleted signup session: %s", sessionId)
}

// ExpireOTP manually marks OTP as expired by setting expiration to past timestamp
//
// Example:
//
//	setup.ExpireOTP(t, redisURL, sessionId)
func ExpireOTP(t *testing.T, redisURL, sessionId string) {
	client := GetTestRedisClient(t, redisURL)
	defer client.Close()

	ctx := context.Background()
	// Set OTP expiration to 1 hour ago (past timestamp)
	pastTime := time.Now().Add(-1 * time.Hour).Unix()
	err := client.HSet(ctx, "signup:"+sessionId, "otp_expires_at", pastTime).Err()
	require.NoError(t, err, "should set OTP expiration to past")
	t.Logf("Set OTP as expired for session: %s", sessionId)
}

// GetSignupSessionData retrieves all session data from Redis
// Useful for debugging or verifying session state
//
// Example:
//
//	data := setup.GetSignupSessionData(t, redisURL, sessionId)
//	t.Logf("Session data: %+v", data)
func GetSignupSessionData(t *testing.T, redisURL, sessionId string) map[string]string {
	client := GetTestRedisClient(t, redisURL)
	defer client.Close()

	ctx := context.Background()
	data, err := client.HGetAll(ctx, "signup:"+sessionId).Result()
	require.NoError(t, err, "should get session data")

	return data
}

// GetSessionOTP retrieves the OTP hash and expiration for a session
//
// Example:
//
//	otpHash, expiry := setup.GetSessionOTP(t, redisURL, sessionId)
func GetSessionOTP(t *testing.T, redisURL, sessionId string) (otpHash string, otpExpiresAt int64) {
	client := GetTestRedisClient(t, redisURL)
	defer client.Close()

	ctx := context.Background()
	otpHash, err := client.HGet(ctx, "signup:"+sessionId, "otp").Result()
	require.NoError(t, err, "should get OTP hash")

	expiryStr, err := client.HGet(ctx, "signup:"+sessionId, "otp_expires_at").Result()
	require.NoError(t, err, "should get OTP expiration")

	expiry, _ := strconv.ParseInt(expiryStr, 10, 64)

	return otpHash, expiry
}

// SetSessionStep directly sets the signup session step
// Useful for skipping steps in tests
//
// Example:
//
//	setup.SetSessionStep(t, redisURL, sessionId, "username_set")
func SetSessionStep(t *testing.T, redisURL, sessionId, step string) {
	client := GetTestRedisClient(t, redisURL)
	defer client.Close()

	ctx := context.Background()
	err := client.HSet(ctx, "signup:"+sessionId, "step", step).Err()
	require.NoError(t, err, "should set session step")
	t.Logf("Set session step to: %s", step)
}

// ClearAuthTokens removes all auth tokens for a user from Redis
// Useful for testing token expiration
//
// Example:
//
//	setup.ClearAuthTokens(t, redisURL, userId)
func ClearAuthTokens(t *testing.T, redisURL, userId string) {
	client := GetTestRedisClient(t, redisURL)
	defer client.Close()

	ctx := context.Background()

	// Clear access and refresh tokens
	pattern := fmt.Sprintf("auth*:%s", userId)
	iter := client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		client.Del(ctx, key)
	}

	t.Logf("Cleared auth tokens for user: %s", userId)
}

// VerifySessionExists checks if a signup session exists in Redis
//
// Example:
//
//	exists := setup.VerifySessionExists(t, redisURL, sessionId)
//	require.True(t, exists, "session should exist")
func VerifySessionExists(t *testing.T, redisURL, sessionId string) bool {
	client := GetTestRedisClient(t, redisURL)
	defer client.Close()

	ctx := context.Background()
	exists, err := client.Exists(ctx, "signup:"+sessionId).Result()
	require.NoError(t, err, "should check session existence")

	return exists > 0
}

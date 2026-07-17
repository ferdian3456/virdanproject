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

func GetTestRedisClient(t *testing.T, redisURL string) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: redisURL,
		DB:   0,
	})

	ctx := context.Background()
	err := client.Ping(ctx).Err()
	require.NoError(t, err, "redis client should connect")

	return client
}

func DeleteSignupSession(t *testing.T, redisURL, sessionId string) {
	client := GetTestRedisClient(t, redisURL)
	defer client.Close()

	ctx := context.Background()
	err := client.Del(ctx, "signup:"+sessionId).Err()
	require.NoError(t, err, "should delete signup session")
	t.Logf("Deleted signup session: %s", sessionId)
}

func ExpireOTP(t *testing.T, redisURL, sessionId string) {
	client := GetTestRedisClient(t, redisURL)
	defer client.Close()

	ctx := context.Background()
	pastTime := time.Now().Add(-1 * time.Hour).Unix()
	err := client.HSet(ctx, "signup:"+sessionId, "otp_expires_at", pastTime).Err()
	require.NoError(t, err, "should set OTP expiration to past")
	t.Logf("Set OTP as expired for session: %s", sessionId)
}

func GetSignupSessionData(t *testing.T, redisURL, sessionId string) map[string]string {
	client := GetTestRedisClient(t, redisURL)
	defer client.Close()

	ctx := context.Background()
	data, err := client.HGetAll(ctx, "signup:"+sessionId).Result()
	require.NoError(t, err, "should get session data")

	return data
}

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

func SetSessionStep(t *testing.T, redisURL, sessionId, step string) {
	client := GetTestRedisClient(t, redisURL)
	defer client.Close()

	ctx := context.Background()
	err := client.HSet(ctx, "signup:"+sessionId, "step", step).Err()
	require.NoError(t, err, "should set session step")
	t.Logf("Set session step to: %s", step)
}

func ClearAuthTokens(t *testing.T, redisURL, userId string) {
	client := GetTestRedisClient(t, redisURL)
	defer client.Close()

	ctx := context.Background()

	pattern := fmt.Sprintf("auth*:%s", userId)
	iter := client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		client.Del(ctx, key)
	}

	t.Logf("Cleared auth tokens for user: %s", userId)
}

func VerifySessionExists(t *testing.T, redisURL, sessionId string) bool {
	client := GetTestRedisClient(t, redisURL)
	defer client.Close()

	ctx := context.Background()
	exists, err := client.Exists(ctx, "signup:"+sessionId).Result()
	require.NoError(t, err, "should check session existence")

	return exists > 0
}

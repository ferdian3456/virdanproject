package auth

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestSignupStatusGet_AfterStart tests signup status after starting signup
func TestSignupStatusGet_AfterStart(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err)
	defer func() { _ = infra.Terminate(ctx, t) }()

	_ = setup.RunMigration(infra.PgURL, t)
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Setup: Start signup
	reqBody := []byte(`{"email":"status@example.com"}`)
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	// Test: Get signup status
	t.Log("=== Testing Signup Status After Start ===")
	req = setup.CreateJSONRequest(http.MethodGet, fmt.Sprintf("/api/auth/signup/%s/status", sessionId), nil)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "get status request should succeed")
	setup.RequireStatus(t, resp, 200)

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "sessionId", "response should contain sessionId")
	require.Contains(t, result, "step", "response should contain step")

	step := result["step"].(string)
	require.Equal(t, "start_signup", step, "step should be start_signup")

	t.Logf("Signup status retrieved successfully, step: %s", step)
}

// TestSignupStatusGet_AfterOTPVerification tests signup status after OTP verification
func TestSignupStatusGet_AfterOTPVerification(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err)
	defer func() { _ = infra.Terminate(ctx, t) }()

	_ = setup.RunMigration(infra.PgURL, t)
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Setup: Start signup and verify OTP
	testEmail := "statusotp@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	_, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "OTP verification should succeed")

	// Test: Get signup status
	t.Log("=== Testing Signup Status After OTP Verification ===")
	req = setup.CreateJSONRequest(http.MethodGet, fmt.Sprintf("/api/auth/signup/%s/status", sessionId), nil)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "get status request should succeed")
	setup.RequireStatus(t, resp, 200)

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "step", "response should contain step")

	step := result["step"].(string)
	require.Equal(t, "otp_verified", step, "step should be otp_verified")

	t.Logf("Signup status retrieved successfully, step: %s", step)
}

// TestSignupStatusGet_InvalidSessionId tests signup status with invalid session ID
func TestSignupStatusGet_InvalidSessionId(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err)
	defer func() { _ = infra.Terminate(ctx, t) }()

	_ = setup.RunMigration(infra.PgURL, t)
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Test: Get status with invalid session ID
	t.Log("=== Testing Signup Status with Invalid Session ID ===")
	req := setup.CreateJSONRequest(http.MethodGet, "/api/auth/signup/00000000-0000-0000-0000-000000000000/status", nil)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get status request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	t.Logf("Error message for invalid session: %s", errMsg)
}

// TestSignupStatusGet_ExpiredSession tests signup status with expired session
func TestSignupStatusGet_ExpiredSession(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err)
	defer func() { _ = infra.Terminate(ctx, t) }()

	_ = setup.RunMigration(infra.PgURL, t)
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Setup: Start signup
	reqBody := []byte(`{"email":"expiredstatus@example.com"}`)
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	// Manually delete the session using Redis helper
	t.Log("=== Testing Signup Status with Deleted Session ===")
	setup.DeleteSignupSession(t, infra.RedisURL, sessionId)

	// Test: Get status with deleted/expired session
	req = setup.CreateJSONRequest(http.MethodGet, fmt.Sprintf("/api/auth/signup/%s/status", sessionId), nil)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "get status request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "expired", "error message should mention session expired or not exists")

	t.Logf("Correctly rejected deleted session")
}

// TestSignupStatusGet_InvalidSessionIdFormat tests signup status with malformed session ID
func TestSignupStatusGet_InvalidSessionIdFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err)
	defer func() { _ = infra.Terminate(ctx, t) }()

	_ = setup.RunMigration(infra.PgURL, t)
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Test: Get status with malformed session ID
	t.Log("=== Testing Signup Status with Malformed Session ID ===")
	req := setup.CreateJSONRequest(http.MethodGet, "/api/auth/signup/invalid-uuid-format/status", nil)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get status request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	t.Logf("Error message for malformed session ID: %s", errMsg)
}

package auth

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestSignupOTPPost_Success tests successful OTP verification
func TestSignupOTPPost_Success(t *testing.T) {
	t.Parallel() // Enable parallel execution

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()

	// Setup: Start signup
	testEmail := "otpverify@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	// Get OTP from MailHog
	t.Log("=== Testing OTP Verification with Correct OTP ===")
	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	t.Logf("OTP obtained: %s", otp)

	// Verify OTP with logging
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "OTP verification should succeed")
	setup.RequireStatus(t, resp, 200)

	t.Logf("✅ OTP verified successfully")
}

// TestSignupOTPPost_EmptyOTP tests OTP verification with empty OTP
func TestSignupOTPPost_EmptyOTP(t *testing.T) {
	t.Parallel() // Enable parallel execution

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Setup: Start signup
	testEmail := "emptyotp_test_validation@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	// Test: Empty OTP with logging
	t.Log("=== Testing OTP Verification with Empty OTP ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":""}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "verify OTP request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "required", "error message should mention OTP is required")
	t.Logf("✅ Correctly rejected empty OTP")
}

// TestSignupOTPPost_OTPLessThan6Chars tests OTP validation for minimum length
func TestSignupOTPPost_OTPLessThan6Chars(t *testing.T) {
	t.Parallel() // Enable parallel execution

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Setup: Start signup
	testEmail := "shortotp_test_validation@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	// Test: OTP less than 6 characters with logging
	t.Log("=== Testing OTP Verification with Less Than 6 Characters ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"12345"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "verify OTP request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "at least 6 characters", "error message should mention minimum length")
	t.Logf("✅ Correctly rejected short OTP")
}

// TestSignupOTPPost_WrongOTP tests OTP verification with incorrect OTP
func TestSignupOTPPost_WrongOTP(t *testing.T) {
	t.Parallel() // Enable parallel execution

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Setup: Start signup
	testEmail := "wrongotp@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	// Test: Wrong OTP with logging
	t.Log("=== Testing OTP Verification with Wrong OTP ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"999999"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "verify OTP request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "does not match", "error message should mention OTP doesn't match")
	t.Logf("✅ Correctly rejected wrong OTP")
}

// TestSignupOTPPost_ExpiredOTP tests OTP verification with expired OTP
func TestSignupOTPPost_ExpiredOTP(t *testing.T) {
	t.Parallel() // Enable parallel execution

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	infra := setup.GetGlobalInfra()

	// Setup: Start signup
	testEmail := "expiredotp@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	// Get correct OTP first (we won't use it, just need to set up the session)
	_ = setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)

	// Manually expire the OTP using Redis helper
	t.Log("=== Testing OTP Verification with Expired OTP ===")
	setup.ExpireOTP(t, infra.RedisURL, sessionId)

	// Try to verify with expired OTP with logging
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"123456"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "verify OTP request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "expired", "error message should mention OTP is expired")

	t.Logf("✅ Correctly rejected expired OTP")
}

// TestSignupOTPPost_InvalidSessionId tests OTP verification with invalid session ID
func TestSignupOTPPost_InvalidSessionId(t *testing.T) {
	t.Parallel() // Enable parallel execution

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Test: Invalid session ID (non-existent or invalid UUID) with logging
	t.Log("=== Testing OTP Verification with Invalid Session ID ===")
	reqBody := []byte(`{"sessionId":"00000000-0000-0000-0000-000000000000","otp":"123456"}`)
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "verify OTP request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	t.Logf("✅ Error message for invalid session: %s", errMsg)
}

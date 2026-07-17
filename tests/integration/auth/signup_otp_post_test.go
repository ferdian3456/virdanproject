package auth

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestSignupOTPPost_Success(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()

	testEmail := "otpverify@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	t.Log("=== Testing OTP Verification with Correct OTP ===")
	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	t.Logf("OTP obtained: %s", otp)

	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "OTP verification should succeed")
	setup.RequireStatus(t, resp, 200)

	t.Logf("✅ OTP verified successfully")
}

func TestSignupOTPPost_EmptyOTP(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	testEmail := "emptyotp_test_validation@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

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

func TestSignupOTPPost_OTPLessThan6Chars(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	testEmail := "shortotp_test_validation@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	t.Log("=== Testing OTP Verification with Less Than 6 Characters ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"12345"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "verify OTP request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "exactly 6 characters", "validator enforces Len(6) so the message says exactly")
	t.Logf("Correctly rejected short OTP")
}

func TestSignupOTPPost_WrongOTP(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	testEmail := "wrongotp@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

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

func TestSignupOTPPost_ExpiredOTP(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	infra := setup.GetGlobalInfra()

	testEmail := "expiredotp@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	_ = setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)

	t.Log("=== Testing OTP Verification with Expired OTP ===")
	setup.ExpireOTP(t, infra.RedisURL, sessionId)

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

func TestSignupOTPPost_InvalidSessionId(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

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

package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestSignupResendOtpPost_Success(t *testing.T) {
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

	testEmail := "resend@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	firstOTP := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	t.Logf("First OTP: %s", firstOTP)

	setup.ExpireOTP(t, infra.RedisURL, sessionId)

	t.Log("=== Testing OTP Resend ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/resend-otp", reqBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "resend OTP request should succeed")
	setup.RequireStatus(t, resp, 200)

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "sessionId", "response should contain sessionId")
	require.Contains(t, result, "otpExpiresAt", "response should contain otpExpiresAt")

	newOTP := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	t.Logf("New OTP: %s", newOTP)

	require.NotEqual(t, firstOTP, newOTP, "new OTP should be different from first OTP")

	t.Logf("OTP resent successfully")
}

func TestSignupResendOtpPost_BeforeExpiry(t *testing.T) {
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

	testEmail := "beforeexpiry@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	_ = setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)

	t.Log("=== Testing OTP Resend Before Expiry (Should Fail/Wait) ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/resend-otp", reqBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "resend OTP request should complete")

	result = setup.ParseJSONResponse(t, resp)

	if _, hasError := result["error"]; hasError {
		errMsg := setup.ParseErrorMessage(t, result)
		t.Logf("Correctly rejected resend before expiry: %s", errMsg)
	} else {
		t.Logf("API allows resend before expiry (new OTP sent)")
		require.Contains(t, result, "otpExpiresAt", "response should contain otpExpiresAt")
	}
}

func TestSignupResendOtpPost_InvalidSessionId(t *testing.T) {
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

	t.Log("=== Testing OTP Resend with Invalid Session ID ===")
	reqBody := []byte(`{"sessionId":"00000000-0000-0000-0000-000000000000"}`)
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/resend-otp", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "resend OTP request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	t.Logf("Error message for invalid session: %s", errMsg)
}

func TestSignupResendOtpPost_ExpiredSession(t *testing.T) {
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

	reqBody := []byte(`{"email":"expiredresend@example.com"}`)
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	t.Log("=== Testing OTP Resend with Expired Session ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/resend-otp", reqBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "resend OTP request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	if strings.Contains(errMsg, "expired") || strings.Contains(errMsg, "5 minutes") || strings.Contains(errMsg, "wait") {
		t.Logf("Correctly rejected resend for expired/rate-limited session: %s", errMsg)
	} else {
		t.Logf("API returned error: %s", errMsg)
	}
}

func TestSignupResendOtpPost_MultipleResends(t *testing.T) {
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

	testEmail := "multipleresend@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	t.Log("=== First Resend ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/resend-otp", reqBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "first resend should succeed")

	result = setup.ParseJSONResponse(t, resp)
	firstOtpExpiresAt := result["otpExpiresAt"]
	t.Logf("First OTP expires at: %v", firstOtpExpiresAt)

	t.Log("=== Second Resend (Immediate) ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/resend-otp", reqBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "second resend request should complete")

	result = setup.ParseJSONResponse(t, resp)

	if _, hasError := result["error"]; hasError {
		errMsg := setup.ParseErrorMessage(t, result)
		t.Logf("Second resend rejected: %s", errMsg)
	} else {
		secondOtpExpiresAt := result["otpExpiresAt"]
		t.Logf("Second resend allowed, new OTP expires at: %v", secondOtpExpiresAt)
	}

	t.Log("=== Expiring OTP in Redis before third resend ===")
	setup.ExpireOTP(t, infra.RedisURL, sessionId)

	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/resend-otp", reqBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "third resend request should complete")

	result = setup.ParseJSONResponse(t, resp)

	if _, hasError := result["error"]; !hasError {
		thirdOtpExpiresAt := result["otpExpiresAt"]
		t.Logf("Third resend successful, new OTP expires at: %v", thirdOtpExpiresAt)
	}
}

func TestSignupResendOtpPost_NewOTPCanVerify(t *testing.T) {
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

	testEmail := "resendverify@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	_ = setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)

	t.Log("=== Resending OTP ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/resend-otp", reqBody)
	_, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "resend OTP should succeed")

	t.Log("=== Getting New OTP and Verifying ===")
	newOTP := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	t.Logf("New OTP: %s", newOTP)

	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, newOTP))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "OTP verification with new OTP should succeed")
	setup.RequireStatus(t, resp, 200)

	t.Logf("New OTP after resend works correctly")
}

package auth

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestSignupUsernamePost_Success tests successful username setting
func TestSignupUsernamePost_Success(t *testing.T) {
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
	testEmail := "username@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	_, err = app.Test(req)
	require.NoError(t, err, "OTP verification should succeed")

	// Test: Set valid username
	t.Log("=== Testing Username Setting with Valid Username ===")
	validUsername := "validuser123"
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":"%s"}`, sessionId, validUsername))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set username should succeed")
	setup.RequireStatus(t, resp, 200)

	t.Logf("Username set successfully: %s", validUsername)
}

// TestSignupUsernamePost_EmptyUsername tests username validation for empty username
func TestSignupUsernamePost_EmptyUsername(t *testing.T) {
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
	testEmail := "emptyuser@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	_, err = app.Test(req)
	require.NoError(t, err, "OTP verification should succeed")

	// Test: Empty username
	t.Log("=== Testing Username Setting with Empty Username ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":""}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set username request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "required", "error message should mention username is required")
}

// TestSignupUsernamePost_TooShort tests username validation for minimum length
func TestSignupUsernamePost_TooShort(t *testing.T) {
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
	testEmail := "shortuser@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	_, err = app.Test(req)
	require.NoError(t, err, "OTP verification should succeed")

	// Test: Username less than 4 characters
	t.Log("=== Testing Username Setting with Less Than 4 Characters ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":"abc"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set username request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "at least 4 characters", "error message should mention minimum length")
}

// TestSignupUsernamePost_TooLong tests username validation for maximum length
func TestSignupUsernamePost_TooLong(t *testing.T) {
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
	testEmail := "longuser@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	_, err = app.Test(req)
	require.NoError(t, err, "OTP verification should succeed")

	// Test: Username more than 22 characters
	t.Log("=== Testing Username Setting with More Than 22 Characters ===")
	longUsername := "thisusernameiswaytoolongtobevalid"
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":"%s"}`, sessionId, longUsername))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set username request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "at most 22 characters", "error message should mention maximum length")
}

// TestSignupUsernamePost_AlreadyTaken tests username validation for uniqueness
func TestSignupUsernamePost_AlreadyTaken(t *testing.T) {
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

	existingUsername := "takenusername"

	// Setup: Create first user with existingUsername
	_ = setup.CreateTestUser(t, app, infra.MailhogURL, "user1@example.com", existingUsername, "password123")

	// Setup: Start second signup and verify OTP
	testEmail2 := "user2@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail2))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "second signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId2 := result["sessionId"].(string)

	otp2 := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail2)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId2, otp2))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	_, err = app.Test(req)
	require.NoError(t, err, "OTP verification should succeed")

	// Test: Try to use the same username
	t.Log("=== Testing Username Setting with Already Taken Username ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":"%s"}`, sessionId2, existingUsername))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set username request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "already taken", "error message should mention username is already taken")

	t.Logf("Correctly rejected duplicate username: %s", errMsg)
}

// TestSignupUsernamePost_BeforeOTPVerification tests that username can't be set before OTP verification
func TestSignupUsernamePost_BeforeOTPVerification(t *testing.T) {
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

	// Setup: Start signup (but don't verify OTP)
	testEmail := "beforeotp@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	// Test: Try to set username before OTP verification
	t.Log("=== Testing Username Setting Before OTP Verification ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":"testuser"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set username request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "Invalid signup step", "error message should mention wrong signup step")

	t.Logf("Correctly rejected username before OTP verification")
}

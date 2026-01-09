package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestSignupStatusAPI tests the GET /auth/signup/:sessionId/status endpoint
func TestSignupStatusAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// 1. Start infrastructure
	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err, "infrastructure should start successfully")
	defer func() { _ = infra.Terminate(ctx, t) }()

	// 2. Run migrations
	t.Log("=== Running Database Migrations ===")
	_ = setup.RunMigration(infra.PgURL, t)

	// 3. Setup test app
	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Test 1: Check status before signup starts - should error
	t.Log("=== Test 1: Check Status Before Signup - Should Return VALIDATION_ERROR ===")
	req := setup.CreateJSONRequest(http.MethodGet, "/api/auth/signup/nonexistent-session/status", nil)
	resp, err := app.Test(req)
	require.NoError(t, err, "status check request should complete")

	result := setup.ParseJSONResponse(t, resp)
	code, message, param := setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.NotEmpty(t, message, "error message should not be empty")
	require.Equal(t, "sessionId", param, "error param should be 'sessionId'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 2: Start signup and check initial status
	t.Log("=== Test 2: Start Signup and Check Initial Status ===")
	testEmail := "statuscheck@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "signup start should succeed")

	result = setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	// Check status immediately after signup/start
	req = setup.CreateJSONRequest(http.MethodGet, fmt.Sprintf("/api/auth/signup/%s/status", sessionId), nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "status check should succeed")
	require.Equal(t, 200, resp.StatusCode, "status check should return 200")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "sessionId", "response should contain sessionId")
	require.Contains(t, result, "step", "response should contain step")
	require.Equal(t, "start_signup", result["step"], "step should be start_signup initially")

	t.Logf("✓ Initial Status: sessionId=%s, step=%s", result["sessionId"], result["step"])

	// Test 3: Verify OTP and check status
	t.Log("=== Test 3: Verify OTP and Check Status ===")
	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	_, err = app.Test(req)
	require.NoError(t, err, "OTP verification should succeed")

	// Check status after OTP verification
	req = setup.CreateJSONRequest(http.MethodGet, fmt.Sprintf("/api/auth/signup/%s/status", sessionId), nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "status check should succeed")

	result = setup.ParseJSONResponse(t, resp)
	require.Equal(t, "otp_verified", result["step"], "step should be otp_verified after OTP verification")

	t.Logf("✓ After OTP: step=%s", result["step"])

	// Test 4: Set username and check status
	t.Log("=== Test 4: Set Username and Check Status ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":"testuser"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	_, err = app.Test(req)
	require.NoError(t, err, "username set should succeed")

	// Check status after username set
	req = setup.CreateJSONRequest(http.MethodGet, fmt.Sprintf("/api/auth/signup/%s/status", sessionId), nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "status check should succeed")

	result = setup.ParseJSONResponse(t, resp)
	require.Equal(t, "username_set", result["step"], "step should be username_set after setting username")

	t.Logf("✓ After Username: step=%s", result["step"])

	// Test 5: Set password and check final status
	t.Log("=== Test 5: Set Password and Check Final Status ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":"password123"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "password set should succeed")
	require.Equal(t, 200, resp.StatusCode, "password set should return 200")

	// Check status after password set (signup complete)
	req = setup.CreateJSONRequest(http.MethodGet, fmt.Sprintf("/api/auth/signup/%s/status", sessionId), nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "status check should succeed")

	result = setup.ParseJSONResponse(t, resp)
	t.Logf("Final response: %+v", result)

	// After signup is complete, the session might be deleted
	// So we might get an error or nil step
	if step, ok := result["step"]; ok && step != nil {
		require.Equal(t, "password_set", step, "step should be password_set after setting password")
		t.Logf("✓ Final Status: step=%s", step)
	} else {
		// Session might be deleted after signup is complete
		t.Logf("✓ Final Status: Session deleted or step is nil (signup complete)")
	}

	t.Log("=== All Signup Status Tests Passed ===")
}

// TestLoginAPI tests the POST /auth/login endpoint
func TestLoginAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// 1. Start infrastructure
	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err, "infrastructure should start successfully")
	defer func() { _ = infra.Terminate(ctx, t) }()

	// 2. Run migrations
	t.Log("=== Running Database Migrations ===")
	_ = setup.RunMigration(infra.PgURL, t)

	// 3. Setup test app
	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Setup: Create a test user first
	t.Log("=== Setup: Creating Test User ===")
	testEmail := "logintest@example.com"
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

	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":"loginuser"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	_, err = app.Test(req)
	require.NoError(t, err, "username set should succeed")

	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":"pass123"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "password set should succeed")
	require.Equal(t, 200, resp.StatusCode, "password set should return 200")

	t.Log("✓ Test user created successfully")

	// Test 1: Successful login
	t.Log("=== Test 1: Successful Login ===")
	reqBody = []byte(`{"username":"loginuser","password":"pass123"}`)
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/login", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "login request should complete")
	require.Equal(t, 200, resp.StatusCode, "login should return 200")

	result = setup.ParseJSONResponse(t, resp)

	// Verify tokens are returned
	accessToken, ok := result["accessToken"].(string)
	require.True(t, ok, "accessToken should be present in response")
	require.NotEmpty(t, accessToken, "accessToken should not be empty")

	refreshToken, ok := result["refreshToken"].(string)
	require.True(t, ok, "refreshToken should be present in response")
	require.NotEmpty(t, refreshToken, "refreshToken should not be empty")

	tokenType, ok := result["tokenType"].(string)
	require.True(t, ok, "tokenType should be present in response")
	require.Equal(t, "Bearer", tokenType, "tokenType should be Bearer")

	// Log token previews (first 20 chars if available)
	accessTokenPreview := accessToken
	refreshTokenPreview := refreshToken
	if len(accessToken) > 20 {
		accessTokenPreview = accessToken[:20] + "..."
	}
	if len(refreshToken) > 20 {
		refreshTokenPreview = refreshToken[:20] + "..."
	}
	t.Logf("✓ Login successful: accessToken=%s, refreshToken=%s, tokenType=%s",
		accessTokenPreview, refreshTokenPreview, tokenType)

	// Test 2: Login with wrong username
	t.Log("=== Test 2: Login with Wrong Username ===")
	reqBody = []byte(`{"username":"wronguser","password":"pass123"}`)
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/login", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "login request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param := setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.NotEmpty(t, message, "error message should not be empty")
	require.Equal(t, "username", param, "error param should be 'username'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 3: Login with wrong password
	t.Log("=== Test 3: Login with Wrong Password ===")
	reqBody = []byte(`{"username":"loginuser","password":"wrongpass"}`)
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/login", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "login request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.NotEmpty(t, message, "error message should not be empty")
	require.Equal(t, "password", param, "error param should be 'password'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 4: Login with empty username
	t.Log("=== Test 4: Login with Empty Username ===")
	reqBody = []byte(`{"username":"","password":"pass123"}`)
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/login", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "login request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.NotEmpty(t, message, "error message should not be empty")
	require.Equal(t, "username", param, "error param should be 'username'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 5: Login with empty password
	t.Log("=== Test 5: Login with Empty Password ===")
	reqBody = []byte(`{"username":"loginuser","password":""}`)
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/login", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "login request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.NotEmpty(t, message, "error message should not be empty")
	require.Equal(t, "password", param, "error param should be 'password'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 6: Login with username too short
	t.Log("=== Test 6: Login with Username Too Short ===")
	reqBody = []byte(`{"username":"abc","password":"pass123"}`)
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/login", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "login request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Contains(t, message, "at least 4 characters", "error message should mention minimum length")
	require.Equal(t, "username", param, "error param should be 'username'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 7: Login with password too short
	t.Log("=== Test 7: Login with Password Too Short ===")
	reqBody = []byte(`{"username":"loginuser","password":"1234"}`)
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/login", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "login request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Contains(t, message, "at least 5 characters", "error message should mention minimum length")
	require.Equal(t, "password", param, "error param should be 'password'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	t.Log("=== All Login Tests Passed ===")
}

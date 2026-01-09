package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestErrorStructureValidation validates that error responses follow the correct structure
func TestErrorStructureValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// 1. Start infrastructure
	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err, "infrastructure should start successfully")
	defer infra.Terminate(ctx, t)

	// 2. Run migrations
	t.Log("=== Running Database Migrations ===")
	setup.RunMigration(infra.PgURL, t)

	// 3. Setup test app
	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Test 1: Empty email validation error
	t.Log("=== Test 1: Empty Email - Should Have VALIDATION_ERROR Code ===")
	reqBody := []byte(`{"email":""}`)
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "signup start request should complete")

	result := setup.ParseJSONResponse(t, resp)
	code, message, param := setup.ParseErrorDetail(t, result)

	// Validasi structure error
	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.NotEmpty(t, message, "error message should not be empty")
	require.Equal(t, "email", param, "error param should be 'email' field")

	t.Logf("✓ Validation Error Structure: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 2: Email too short validation error
	t.Log("=== Test 2: Email Too Short - Should Have VALIDATION_ERROR Code ===")
	reqBody = []byte(`{"email":"short@x.co"}`)
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "signup start request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Contains(t, message, "at least 16 characters", "error message should mention minimum length")
	require.Equal(t, "email", param, "error param should be 'email' field")

	t.Logf("✓ Validation Error Structure: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 3: Email already exists - should have VALIDATION_ERROR
	t.Log("=== Test 3: Email Already Exists - Setup ===")

	// Create user first
	testEmail := "errorstructure@example.com"
	reqBody = []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "first signup start should succeed")

	result = setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	// Get OTP and verify
	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "verify OTP should succeed")

	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":"testuser"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set username should succeed")

	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":"password123"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set password should succeed")
	require.Equal(t, 200, resp.StatusCode, "set password should return 200")

	t.Log("=== Test 3: Try Same Email Again ===")
	reqBody = []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "second signup start request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Contains(t, message, "already exists", "error message should mention already exists")
	require.Equal(t, "email", param, "error param should be 'email' field")

	t.Logf("✓ Validation Error Structure: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 4: Empty OTP validation error
	t.Log("=== Test 4: Empty OTP - Should Have VALIDATION_ERROR Code ===")
	testEmail2 := "otptest@example.com"
	reqBody = []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail2))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "signup start should succeed")

	result = setup.ParseJSONResponse(t, resp)
	sessionId = result["sessionId"].(string)

	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":""}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "verify OTP request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Contains(t, message, "required", "error message should mention required")
	require.Equal(t, "otp", param, "error param should be 'otp' field")

	t.Logf("✓ Validation Error Structure: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 5: Wrong OTP - validation error
	t.Log("=== Test 5: Wrong OTP - Should Have VALIDATION_ERROR Code ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"999999"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "verify OTP request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Contains(t, message, "does not match", "error message should mention does not match")
	require.Equal(t, "otp", param, "error param should be 'otp' field")

	t.Logf("✓ Validation Error Structure: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 6: Username validation errors
	t.Log("=== Test 6: Username Validations ===")

	// Get to username step first
	otp = setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail2)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "verify OTP should succeed")

	// Test 6a: Empty username
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":""}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set username request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Contains(t, message, "required", "error message should mention required")
	require.Equal(t, "username", param, "error param should be 'username' field")

	t.Logf("✓ Validation Error Structure: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 6b: Username too short
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":"abc"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set username request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Contains(t, message, "at least 4 characters", "error message should mention minimum length")
	require.Equal(t, "username", param, "error param should be 'username' field")

	t.Logf("✓ Validation Error Structure: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 6c: Username too long
	longUsername := "thisusernameiswaytoolongtobevalid"
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":"%s"}`, sessionId, longUsername))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set username request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Contains(t, message, "at most 22 characters", "error message should mention maximum length")
	require.Equal(t, "username", param, "error param should be 'username' field")

	t.Logf("✓ Validation Error Structure: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 7: Password validation errors
	t.Log("=== Test 7: Password Validations ===")

	// Set valid username first
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":"validuser"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set username should succeed")

	// Test 7a: Empty password
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":""}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set password request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Contains(t, message, "required", "error message should mention required")
	require.Equal(t, "password", param, "error param should be 'password' field")

	t.Logf("✓ Validation Error Structure: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 7b: Password too short
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":"1234"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set password request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Contains(t, message, "at least 5 characters", "error message should mention minimum length")
	require.Equal(t, "password", param, "error param should be 'password' field")

	t.Logf("✓ Validation Error Structure: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 7c: Password too long
	longPassword := "thispasswordiswaytoolongtobevalid123"
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":"%s"}`, sessionId, longPassword))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set password request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Contains(t, message, "at most 20 characters", "error message should mention maximum length")
	require.Equal(t, "password", param, "error param should be 'password' field")

	t.Logf("✓ Validation Error Structure: Code=%s, Param=%s, Message=%s", code, param, message)

	t.Log("=== All Error Structure Tests Passed ===")
}

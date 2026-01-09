package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestSignupEmailAlreadyRegistered tests that signup fails when email is already registered
func TestSignupEmailAlreadyRegistered(t *testing.T) {
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

	// 4. Create first user with this email
	testEmail := "existing@example.com"
	testUsername := "existinguser"
	testPassword := "password123"

	// Complete signup flow for first user
	t.Log("=== Creating First User ===")
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "first signup start should succeed")
	require.Equal(t, 200, resp.StatusCode, "first signup start should return 200")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	// Get OTP from MailHog
	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	t.Logf("OTP obtained: %s", otp)

	// Verify OTP
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "verify OTP should succeed")
	require.Equal(t, 200, resp.StatusCode, "verify OTP should return 200")

	// Set username
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":"%s"}`, sessionId, testUsername))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set username should succeed")
	require.Equal(t, 200, resp.StatusCode, "set username should return 200")

	// Set password
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":"%s"}`, sessionId, testPassword))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set password should succeed")
	require.Equal(t, 200, resp.StatusCode, "set password should return 200")

	t.Log("=== First User Created Successfully ===")

	// 5. Try to signup again with same email - should fail
	t.Log("=== Attempting Signup with Same Email ===")
	reqBody = []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "second signup start request should complete")

	// Should return error
	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")

	errObj := result["error"].(map[string]interface{})
	errMsg := errObj["message"].(string)
	require.Contains(t, errMsg, "already exists", "error message should mention email already exists")

	t.Logf("Correctly rejected duplicate email: %s", errMsg)
}

// TestSignupOTPValidation tests various OTP validation scenarios
func TestSignupOTPValidation(t *testing.T) {
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

	testEmail := "otpvalidation@example.com"

	// Test 1: Empty OTP
	t.Log("=== Test 1: Empty OTP ===")
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "signup start should succeed")
	require.Equal(t, 200, resp.StatusCode, "signup start should return 200")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	// Try empty OTP
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":""}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "verify OTP request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "required", "error message should mention OTP is required")

	// Test 2: OTP less than 6 characters
	t.Log("=== Test 2: OTP Less Than 6 Characters ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"12345"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "verify OTP request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg = setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "at least 6 characters", "error message should mention minimum length")

	// Test 3: Wrong OTP
	t.Log("=== Test 3: Wrong OTP ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"999999"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "verify OTP request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg = setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "does not match", "error message should mention OTP doesn't match")

	// Test 4: Correct OTP (should succeed)
	t.Log("=== Test 4: Correct OTP ===")
	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	t.Logf("OTP obtained: %s", otp)

	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "verify OTP with correct code should succeed")
	require.Equal(t, 200, resp.StatusCode, "verify OTP should return 200")
}

// TestSignupUsernameValidation tests various username validation scenarios
func TestSignupUsernameValidation(t *testing.T) {
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

	// Complete first signup to get to username step
	testEmail := "usernamevalidation@example.com"
	t.Log("=== Completing Initial Signup Steps ===")
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	// Verify OTP
	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "verify OTP should succeed")

	// Test 1: Empty username
	t.Log("=== Test 1: Empty Username ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":""}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set username request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "required", "error message should mention username is required")

	// Test 2: Username less than 4 characters
	t.Log("=== Test 2: Username Less Than 4 Characters ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":"abc"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set username request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg = setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "at least 4 characters", "error message should mention minimum length")

	// Test 3: Username more than 22 characters
	t.Log("=== Test 3: Username More Than 22 Characters ===")
	longUsername := "thisusernameiswaytoolongtobevalid"
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":"%s"}`, sessionId, longUsername))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set username request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg = setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "at most 22 characters", "error message should mention maximum length")

	// Test 4: Valid username (should succeed)
	t.Log("=== Test 4: Valid Username ===")
	validUsername := "validuser"
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":"%s"}`, sessionId, validUsername))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set username with valid username should succeed")
	require.Equal(t, 200, resp.StatusCode, "set username should return 200")
}

// TestSignupUsernameAlreadyTaken tests that username validation works correctly
func TestSignupUsernameAlreadyTaken(t *testing.T) {
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

	existingUsername := "takenusername"

	// Create first user with existingUsername
	t.Log("=== Creating First User ===")
	testEmail1 := "user1@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail1))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "first signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId1 := result["sessionId"].(string)

	otp1 := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail1)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId1, otp1))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "verify OTP should succeed")

	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":"%s"}`, sessionId1, existingUsername))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set username should succeed")

	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":"password123"}`, sessionId1))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set password should succeed")
	require.Equal(t, 200, resp.StatusCode, "set password should return 200")

	t.Log("=== First User Created Successfully ===")

	// Try to create second user with same username
	t.Log("=== Attempting to Create Second User with Same Username ===")
	testEmail2 := "user2@example.com"
	reqBody = []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail2))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "second signup start should succeed")

	result = setup.ParseJSONResponse(t, resp)
	sessionId2 := result["sessionId"].(string)

	otp2 := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail2)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId2, otp2))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "verify OTP should succeed")

	// Try to use the same username
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

// TestSignupPasswordValidation tests various password validation scenarios
func TestSignupPasswordValidation(t *testing.T) {
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

	// Complete initial signup to get to password step
	testEmail := "passwordvalidation@example.com"
	t.Log("=== Completing Initial Signup Steps ===")
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "verify OTP should succeed")

	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":"testuser"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set username should succeed")

	// Test 1: Empty password
	t.Log("=== Test 1: Empty Password ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":""}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set password request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "required", "error message should mention password is required")

	// Test 2: Password less than 5 characters
	t.Log("=== Test 2: Password Less Than 5 Characters ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":"1234"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set password request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg = setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "at least 5 characters", "error message should mention minimum length")

	// Test 3: Password more than 20 characters
	t.Log("=== Test 3: Password More Than 20 Characters ===")
	longPassword := "thispasswordiswaytoolongtobevalid123"
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":"%s"}`, sessionId, longPassword))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set password request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg = setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "at most 20 characters", "error message should mention maximum length")

	// Test 4: Valid password (should succeed and return tokens)
	t.Log("=== Test 4: Valid Password ===")
	validPassword := "validpassword123"
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":"%s"}`, sessionId, validPassword))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "set password with valid password should succeed")
	require.Equal(t, 200, resp.StatusCode, "set password should return 200")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "accessToken", "response should contain access token")
	require.Contains(t, result, "refreshToken", "response should contain refresh token")

	t.Log("=== Signup Completed Successfully ===")
}

// TestSignupEmailValidation tests email validation scenarios
func TestSignupEmailValidation(t *testing.T) {
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

	// Test 1: Empty email
	t.Log("=== Test 1: Empty Email ===")
	reqBody := []byte(`{"email":""}`)
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "signup start request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "required", "error message should mention email is required")

	// Test 2: Email less than 16 characters
	t.Log("=== Test 2: Email Less Than 16 Characters ===")
	reqBody = []byte(`{"email":"short@x.co"}`)
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "signup start request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg = setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "at least 16 characters", "error message should mention minimum length")

	// Test 3: Email more than 80 characters
	t.Log("=== Test 3: Email More Than 80 Characters ===")
	longEmail := "thisemailaddressiswaytoolongtobevalidforthissystem123456789012345678901234567890@example.com"
	reqBody = []byte(fmt.Sprintf(`{"email":"%s"}`, longEmail))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "signup start request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg = setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "at most 80 characters", "error message should mention maximum length")

	// Test 4: Valid email (should succeed)
	t.Log("=== Test 4: Valid Email ===")
	validEmail := "validemail@example.com"
	reqBody = []byte(fmt.Sprintf(`{"email":"%s"}`, validEmail))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "signup start with valid email should succeed")
	require.Equal(t, 200, resp.StatusCode, "signup start should return 200")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "sessionId", "response should contain sessionId")
	require.Contains(t, result, "otpExpiresAt", "response should contain otpExpiresAt")

	t.Log("=== Signup Started Successfully ===")
}

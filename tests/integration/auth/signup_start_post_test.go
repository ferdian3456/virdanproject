package auth

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestSignupStartPost_Success tests successful signup start with valid email
func TestSignupStartPost_Success(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestSignupStartPost_Success")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	setup.LogTestStep(t, "Testing Signup Start with Valid Email")
	reqBody := []byte(`{"email":"signup_start_success@example.com"}`)
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)

	result := setup.RequireJSONWithLog(t, app, req, 200)

	require.Contains(t, result, "sessionId", "response should contain sessionId")
	require.Contains(t, result, "otpExpiresAt", "response should contain otpExpiresAt")

	sessionId := result["sessionId"].(string)
	require.NotEmpty(t, sessionId, "sessionId should not be empty")

	setup.LogTestPass(t, "TestSignupStartPost_Success")
	t.Logf("Signup started successfully, sessionId: %s", sessionId)
}

// TestSignupStartPost_EmailAlreadyRegistered tests signup with already registered email
func TestSignupStartPost_EmailAlreadyRegistered(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestSignupStartPost_EmailAlreadyRegistered")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()

	// Setup: Create first user
	testEmail := "existing@example.com"
	setup.LogTestStep(t, "Creating existing user: %s", testEmail)
	_ = setup.CreateTestUser(t, app, infra.MailhogURL, testEmail, "existinguser", "password123")

	// Test: Try to signup with same email
	setup.LogTestStep(t, "Testing Signup Start with Already Registered Email")
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)

	result := setup.RequireJSONWithLog(t, app, req, 400) // Expecting 400 for duplicate

	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "already exists", "error message should mention email already exists")

	setup.LogTestPass(t, "TestSignupStartPost_EmailAlreadyRegistered")
	t.Logf("Correctly rejected duplicate email: %s", errMsg)
}

// TestSignupStartPost_EmailValidation tests email validation
func TestSignupStartPost_EmailValidation(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestSignupStartPost_EmailValidation")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Test 1: Empty email
	setup.LogTestStep(t, "Test 1: Empty Email")
	reqBody := []byte(`{"email":""}`)
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)

	result := setup.RequireJSONWithLog(t, app, req, 400)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "required", "error message should mention email is required")
	t.Logf("Correctly rejected empty email: %s", errMsg)

	// Test 2: Email less than 16 characters
	setup.LogTestStep(t, "Test 2: Email Less Than 16 Characters")
	reqBody = []byte(`{"email":"short@x.co"}`)
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)

	result = setup.RequireJSONWithLog(t, app, req, 400)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg = setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "at least 16 characters", "error message should mention minimum length")
	t.Logf("Correctly rejected short email: %s", errMsg)

	// Test 3: Email more than 80 characters
	setup.LogTestStep(t, "Test 3: Email More Than 80 Characters")
	longEmail := "thisemailaddressiswaytoolongtobevalidforthissystem123456789012345678901234567890@example.com"
	reqBody = []byte(fmt.Sprintf(`{"email":"%s"}`, longEmail))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)

	result = setup.RequireJSONWithLog(t, app, req, 400)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg = setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "at most 80 characters", "error message should mention maximum length")
	t.Logf("Correctly rejected long email: %s", errMsg)

	setup.LogTestPass(t, "TestSignupStartPost_EmailValidation")
}

// TestSignupStartPost_ReplacesExistingSession tests that new signup replaces existing session for same email
func TestSignupStartPost_ReplacesExistingSession(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestSignupStartPost_ReplacesExistingSession")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	testEmail := "replace@example.com"

	// First signup attempt
	setup.LogTestStep(t, "First Signup Attempt")
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)

	result := setup.RequireJSONWithLog(t, app, req, 200)
	firstSessionId := result["sessionId"].(string)
	t.Logf("First session ID: %s", firstSessionId)

	// Second signup attempt with same email (should create new session)
	setup.LogTestStep(t, "Second Signup Attempt with Same Email")
	reqBody = []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)

	result = setup.RequireJSONWithLog(t, app, req, 200)
	secondSessionId := result["sessionId"].(string)
	t.Logf("Second session ID: %s", secondSessionId)

	// Session IDs should be different
	require.NotEqual(t, firstSessionId, secondSessionId, "new signup should create new session")

	setup.LogTestPass(t, "TestSignupStartPost_ReplacesExistingSession")
	t.Logf("Correctly created new session, replacing old one")
}

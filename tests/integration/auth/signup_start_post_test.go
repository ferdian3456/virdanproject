package auth

import (
	"fmt"
	"net/http"
	"strings"
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
	_ = setup.CreateTestUser(t, app, infra.MailhogURL, testEmail, "password123")

	// Test: Try to signup with same email
	setup.LogTestStep(t, "Testing Signup Start with Already Registered Email")
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)

	// Duplicate-email path returns ConflictError → HTTP 409 with the
	// "Email is already registered" copy from StartSignup.
	result := setup.RequireJSONWithLog(t, app, req, 409)

	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "already registered", "error message should mention email already registered")

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

	// Test 2: Email shorter than the validator's MinLen(5).
	setup.LogTestStep(t, "Test 2: Email shorter than 5 characters")
	reqBody = []byte(`{"email":"a@b"}`)
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)

	result = setup.RequireJSONWithLog(t, app, req, 400)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg = setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "at least 5 characters", "error message should mention minimum length")
	t.Logf("Correctly rejected short email: %s", errMsg)

	// Test 3: Email longer than the validator's MaxLen(255). Build a 256-char
	// string with a valid email suffix so the only failing constraint is
	// length, not the Email() regex.
	setup.LogTestStep(t, "Test 3: Email longer than 255 characters")
	longEmail := strings.Repeat("a", 251) + "@b.co" // 251 + 5 = 256 chars
	require.Equal(t, 256, len(longEmail), "longEmail should be 256 chars to exceed MaxLen(255)")
	reqBody = []byte(fmt.Sprintf(`{"email":"%s"}`, longEmail))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)

	result = setup.RequireJSONWithLog(t, app, req, 400)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg = setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "at most 255 characters", "error message should mention maximum length")
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

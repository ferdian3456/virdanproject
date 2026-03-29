package user

import (
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestMeGet_Success tests successful user profile retrieval
func TestMeGet_Success(t *testing.T) {
	t.Parallel() // Enable parallel execution

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestMeGet_Success")

	// Use singleton infrastructure with parallel test setup
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Setup: Create and login user
	testEmail := "meget@example.com"
	testUsername := "megetuser"

	// Get global infrastructure for MailHog URL
	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, testEmail, testUsername, "password123")

	// Test: Get user profile with logging
	setup.LogTestStep(t, "Testing Get User Profile")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/users/me", nil, token)

	// Execute request with logging
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get profile request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "id", "response should contain id")
	require.Contains(t, result, "username", "response should contain username")
	require.Contains(t, result, "email", "response should contain email")
	require.Contains(t, result, "fullname", "response should contain fullname")
	require.Contains(t, result, "bio", "response should contain bio")

	username := result["username"].(string)
	require.Equal(t, testUsername, username, "username should match")

	t.Logf("User profile retrieved successfully")

	setup.LogTestPass(t, "TestMeGet_Success")
}

// TestMeGet_Unauthorized tests profile retrieval without authentication
func TestMeGet_Unauthorized(t *testing.T) {
	t.Parallel() // Enable parallel execution

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestMeGet_Unauthorized")

	// Use singleton infrastructure with parallel test setup
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Test: Get profile without token
	setup.LogTestStep(t, "Testing Get Profile Without Auth")
	req := setup.CreateJSONRequest(http.MethodGet, "/api/users/me", nil)

	// Execute request with logging
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get profile request should complete")

	// Should return unauthorized
	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated request")

	setup.LogTestPass(t, "TestMeGet_Unauthorized")
}

// TestMeGet_InvalidToken tests profile retrieval with invalid token
func TestMeGet_InvalidToken(t *testing.T) {
	t.Parallel() // Enable parallel execution

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestMeGet_InvalidToken")

	// Use singleton infrastructure with parallel test setup
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Test: Get profile with invalid token
	setup.LogTestStep(t, "Testing Get Profile with Invalid Token")
	invalidToken := "invalid.token.here"
	req := setup.CreateAuthRequest(http.MethodGet, "/api/users/me", nil, invalidToken)

	// Execute request with logging
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get profile request should complete")

	// Should return unauthorized
	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 with invalid token")
	t.Logf("Correctly rejected invalid token")

	setup.LogTestPass(t, "TestMeGet_InvalidToken")
}

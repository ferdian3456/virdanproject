package user

import (
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestLogoutPost_Success tests successful logout
func TestLogoutPost_Success(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestLogoutPost_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Setup: Create user and login
	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "logout@example.com", "logoutuser", "password123")

	// Test: Logout
	setup.LogTestStep(t, "Testing Logout")
	req := setup.CreateAuthRequest(http.MethodPost, "/api/users/logout", nil, token)
	_ = setup.RequireJSONWithLog(t, app, req, 200)

	t.Logf("Logout successful")

	setup.LogTestPass(t, "TestLogoutPost_Success")
}

// TestLogoutPost_Unauthorized tests logout without authentication
func TestLogoutPost_Unauthorized(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestLogoutPost_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Test: Logout without token
	setup.LogTestStep(t, "Testing Logout Without Auth")
	req := setup.CreateAuthRequest(http.MethodPost, "/api/users/logout", nil, "")
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "logout request should complete")

	// Should return unauthorized (404 from auth middleware)
	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated logout request")

	setup.LogTestPass(t, "TestLogoutPost_Unauthorized")
}

// TestLogoutPost_TokenCleared tests that logout clears auth tokens
func TestLogoutPost_TokenCleared(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestLogoutPost_TokenCleared")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Setup: Create user and login
	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "logoutclear@example.com", "logoutclearuser", "password123")

	// Test: Logout
	setup.LogTestStep(t, "Testing Logout")
	req := setup.CreateAuthRequest(http.MethodPost, "/api/users/logout", nil, token)
	_ = setup.RequireJSONWithLog(t, app, req, 200)

	// Try to use the same token after logout
	setup.LogTestStep(t, "Testing Token Cleared After Logout")
	req = setup.CreateAuthRequest(http.MethodGet, "/api/users/me", nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "request should complete")

	// Should return unauthorized (404 from auth middleware)
	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 with logged out token")
	t.Logf("Correctly rejected request after logout")

	setup.LogTestPass(t, "TestLogoutPost_TokenCleared")
}

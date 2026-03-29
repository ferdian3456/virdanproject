package server

import (
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestCategoriesGet_LimitNegative tests get categories with negative limit
func TestCategoriesGet_LimitNegative(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestCategoriesGet_LimitNegative")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "neglimit@example.com", "neglimituser", "password123")

	// Test: Get categories with negative limit
	setup.LogTestStep(t, "Testing Get Categories with Negative Limit")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/categories?limit=-1", nil, token)
	result := setup.RequireJSONWithLog(t, app, req, 400)

	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "greater or equal than 0", "error message should mention limit must be >= 0")

	setup.LogTestPass(t, "TestCategoriesGet_LimitNegative")
	t.Logf("Correctly rejected negative limit: %s", errMsg)
}

// TestCategoriesGet_LimitExceeded tests get categories with limit > MAX_LIMIT (20)
func TestCategoriesGet_LimitExceeded(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestCategoriesGet_LimitExceeded")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "maxlimit@example.com", "maxlimituser", "password123")

	// Test: Get categories with limit > 20
	setup.LogTestStep(t, "Testing Get Categories with Limit Exceeded")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/categories?limit=21", nil, token)
	result := setup.RequireJSONWithLog(t, app, req, 400)

	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "exceeded max limit", "error message should mention max limit")

	setup.LogTestPass(t, "TestCategoriesGet_LimitExceeded")
	t.Logf("Correctly rejected exceeded limit: %s", errMsg)
}

// TestMeGet_LimitNegative tests get user servers with negative limit
func TestMeGet_LimitNegative(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestMeGet_LimitNegative")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "neglimitme@example.com", "neglimitmeuser", "password123")

	// Test: Get user servers with negative limit
	setup.LogTestStep(t, "Testing Get User Servers with Negative Limit")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/me?limit=-1", nil, token)
	result := setup.RequireJSONWithLog(t, app, req, 400)

	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "greater or equal than 0", "error message should mention limit must be >= 0")

	setup.LogTestPass(t, "TestMeGet_LimitNegative")
	t.Logf("Correctly rejected negative limit: %s", errMsg)
}

// TestMeGet_LimitExceeded tests get user servers with limit > MAX_LIMIT
func TestMeGet_LimitExceeded(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestMeGet_LimitExceeded")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "maxlimitme@example.com", "maxlimitmeuser", "password123")

	// Test: Get user servers with limit > 20
	setup.LogTestStep(t, "Testing Get User Servers with Limit Exceeded")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/me?limit=21", nil, token)
	result := setup.RequireJSONWithLog(t, app, req, 400)

	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "exceeded max limit", "error message should mention max limit")

	setup.LogTestPass(t, "TestMeGet_LimitExceeded")
	t.Logf("Correctly rejected exceeded limit: %s", errMsg)
}

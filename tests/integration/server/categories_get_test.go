package server

import (
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestCategoriesGet_Success tests successful get server categories
func TestCategoriesGet_Success(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestCategoriesGet_Success")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Setup: Create user
	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "categories@example.com", "categoriesuser", "password123")

	// Test: Get categories
	setup.LogTestStep(t, "Testing Get Server Categories")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/categories", nil, token)
	result := setup.RequireJSONWithLog(t, app, req, 200)

	require.Contains(t, result, "data", "response should contain data")

	setup.LogTestPass(t, "TestCategoriesGet_Success")
	t.Logf("Server categories retrieved successfully")
}

// TestCategoriesGet_Unauthorized tests get categories without authentication
func TestCategoriesGet_Unauthorized(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestCategoriesGet_Unauthorized")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Test: Get categories without token
	setup.LogTestStep(t, "Testing Get Categories Without Auth")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/categories", nil, "")
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get categories request should complete")

	// Should return unauthorized (404 from auth middleware)
	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")

	setup.LogTestPass(t, "TestCategoriesGet_Unauthorized")
	t.Logf("Correctly rejected unauthenticated get categories request")
}

// TestCategoriesGet_WithPagination tests get categories with pagination
func TestCategoriesGet_WithPagination(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestCategoriesGet_WithPagination")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "categoriespage@example.com", "categoriespageuser", "password123")

	// Test: Get categories with limit
	setup.LogTestStep(t, "Testing Get Categories With Pagination")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/categories?limit=5", nil, token)
	result := setup.RequireJSONWithLog(t, app, req, 200)

	require.Contains(t, result, "data", "response should contain data")

	setup.LogTestPass(t, "TestCategoriesGet_WithPagination")
	t.Logf("Server categories retrieved successfully with pagination")
}

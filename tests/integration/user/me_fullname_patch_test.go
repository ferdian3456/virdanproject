package user

import (
	"fmt"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestMeFullnamePatch_Success tests successful fullname update
func TestMeFullnamePatch_Success(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestMeFullnamePatch_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Setup: Create and login user
	testEmail := "fullname@example.com"
	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, testEmail, "fullnameuser", "password123")

	// Test: Update fullname
	setup.LogTestStep(t, "Testing Update Fullname")
	newFullname := "John Doe"
	reqBody := []byte(fmt.Sprintf(`{"fullname":"%s"}`, newFullname))
	req := setup.CreateAuthRequest("PUT", "/api/users/fullname", reqBody, token)
	result := setup.RequireJSONWithLog(t, app, req, 200)

	// API returns {"status":"OK"} for successful update
	require.Contains(t, result, "status", "response should contain status")
	require.Equal(t, "OK", result["status"], "status should be OK")

	t.Logf("Fullname updated successfully")

	setup.LogTestPass(t, "TestMeFullnamePatch_Success")
}

// TestMeFullnamePatch_EmptyFullname tests fullname validation for empty fullname
func TestMeFullnamePatch_EmptyFullname(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestMeFullnamePatch_EmptyFullname")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Setup: Create and login user
	testEmail := "emptyfullname@example.com"
	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, testEmail, "emptyfullnameuser", "password123")

	// Test: Update with empty fullname
	setup.LogTestStep(t, "Testing Update Fullname with Empty Value")
	reqBody := []byte(`{"fullname":""}`)
	req := setup.CreateAuthRequest("PUT", "/api/users/fullname", reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "update fullname request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "required", "error message should mention fullname is required")

	t.Logf("Correctly rejected empty fullname")

	setup.LogTestPass(t, "TestMeFullnamePatch_EmptyFullname")
}

// TestMeFullnamePatch_Unauthorized tests fullname update without authentication
func TestMeFullnamePatch_Unauthorized(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestMeFullnamePatch_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Test: Update fullname without token
	setup.LogTestStep(t, "Testing Update Fullname Without Auth")
	reqBody := []byte(`{"fullname":"John Doe"}`)
	req := setup.CreateJSONRequest("PUT", "/api/users/fullname", reqBody)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "update fullname request should complete")

	// Should return unauthorized
	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated request")

	setup.LogTestPass(t, "TestMeFullnamePatch_Unauthorized")
}

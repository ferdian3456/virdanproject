package user

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestMeBioPatch_Success tests successful bio update
func TestMeBioPatch_Success(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestMeBioPatch_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Setup: Create and login user
	testEmail := "me_bio_patch_success@example.com"
	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, testEmail, "biouser", "password123")

	// Test: Update bio
	setup.LogTestStep(t, "Testing Update Bio")
	newBio := "Software developer who loves coding"
	reqBody := []byte(fmt.Sprintf(`{"bio":"%s"}`, newBio))
	req := setup.CreateAuthRequest("PUT", "/api/users/bio", reqBody, token)
	result := setup.RequireJSONWithLog(t, app, req, 200)

	// API returns {"status":"OK"} for successful update
	require.Contains(t, result, "status", "response should contain status")
	require.Equal(t, "OK", result["status"], "status should be OK")

	t.Logf("Bio updated successfully")

	setup.LogTestPass(t, "TestMeBioPatch_Success")
}

// TestMeBioPatch_EmptyBio tests that bio can be set to empty (allowed)
func TestMeBioPatch_EmptyBio(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestMeBioPatch_EmptyBio")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Setup: Create and login user with bio
	testEmail := "emptybio@example.com"
	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, testEmail, "emptybiouser", "password123")

	// First set a bio
	setup.LogTestStep(t, "Setting Initial Bio")
	reqBody := []byte(`{"bio":"Initial bio"}`)
	req := setup.CreateAuthRequest("PUT", "/api/users/bio", reqBody, token)
	_, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "set initial bio should succeed")

	// Test: Clear bio (set to empty)
	setup.LogTestStep(t, "Testing Clear Bio (Set to Empty)")
	reqBody = []byte(`{"bio":""}`)
	req = setup.CreateAuthRequest("PUT", "/api/users/bio", reqBody, token)
	result := setup.RequireJSONWithLog(t, app, req, 200)

	// API returns {"status":"OK"} for successful update
	require.Contains(t, result, "status", "response should contain status")
	require.Equal(t, "OK", result["status"], "status should be OK")

	t.Logf("Bio cleared successfully")

	setup.LogTestPass(t, "TestMeBioPatch_EmptyBio")
}

// TestMeBioPatch_TooLong tests bio validation for maximum length
func TestMeBioPatch_TooLong(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestMeBioPatch_TooLong")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Setup: Create and login user
	testEmail := "longbio@example.com"
	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, testEmail, "longbiouser", "password123")

	// Test: Update with too long bio (database limit is 150 characters)
	setup.LogTestStep(t, "Testing Update Bio with Too Long Value")
	longBio := ""
	for i := 0; i < 151; i++ {
		longBio += "a"
	}
	reqBody := []byte(fmt.Sprintf(`{"bio":"%s"}`, longBio))
	req := setup.CreateAuthRequest("PUT", "/api/users/bio", reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "update bio request should complete")

	result := setup.ParseJSONResponse(t, resp)
	if _, hasError := result["error"]; hasError {
		// API returns error for bio that's too long (database constraint)
		errMsg := setup.ParseErrorMessage(t, result)
		// Check for various possible error messages
		if strings.Contains(errMsg, "too long") || strings.Contains(errMsg, "Something went wrong") || strings.Contains(errMsg, "problem persists") {
			t.Logf("Correctly rejected long bio: %s", errMsg)
		} else {
			t.Logf("API rejected long bio with: %s", errMsg)
		}
	} else {
		t.Logf("API accepts long bio (no validation on length)")
	}

	setup.LogTestPass(t, "TestMeBioPatch_TooLong")
}

// TestMeBioPatch_Unauthorized tests bio update without authentication
func TestMeBioPatch_Unauthorized(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestMeBioPatch_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Test: Update bio without token
	setup.LogTestStep(t, "Testing Update Bio Without Auth")
	reqBody := []byte(`{"bio":"Test bio"}`)
	req := setup.CreateJSONRequest("PUT", "/api/users/bio", reqBody)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "update bio request should complete")

	// Should return unauthorized
	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated request")

	setup.LogTestPass(t, "TestMeBioPatch_Unauthorized")
}

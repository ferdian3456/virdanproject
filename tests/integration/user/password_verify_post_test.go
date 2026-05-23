package user

import (
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestPasswordVerify_Success confirms the user's current password (step 1 of
// the FE change-password flow).
func TestPasswordVerify_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestPasswordVerify_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "pwverify@example.com", "password123")

	reqBody := []byte(`{"password":"password123"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/users/password/verify", reqBody, token)
	resp, err := app.Test(req)
	require.NoError(t, err, "verify current password should succeed")
	setup.RequireStatus(t, resp, 200)
	setup.LogTestPass(t, "TestPasswordVerify_Success")
}

// TestPasswordVerify_WrongPassword rejects an incorrect current password.
func TestPasswordVerify_WrongPassword(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestPasswordVerify_WrongPassword")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "pwverifybad@example.com", "password123")

	reqBody := []byte(`{"password":"not-the-real-one"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/users/password/verify", reqBody, token)
	resp, err := app.Test(req)
	require.NoError(t, err, "verify current password request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "wrong password should produce an error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "Current password is incorrect", "error message should mention incorrect password")
	setup.LogTestPass(t, "TestPasswordVerify_WrongPassword")
}

// TestPasswordVerify_Unauthorized rejects unauthenticated callers.
func TestPasswordVerify_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestPasswordVerify_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	reqBody := []byte(`{"password":"whatever"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/users/password/verify", reqBody, "")
	resp, err := app.Test(req)
	require.NoError(t, err, "verify current password request should complete")
	require.NotEqual(t, 200, resp.StatusCode, "unauthenticated verify must not succeed")
	setup.LogTestPass(t, "TestPasswordVerify_Unauthorized")
}

package user

import (
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestMeDelete_Success soft-deletes the authenticated account and verifies the
// token can no longer be used.
func TestMeDelete_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestMeDelete_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "deleteme@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodDelete, "/api/users/me", nil, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "delete account should succeed")
	setup.RequireStatus(t, resp, 200)

	// Token should be invalidated after soft-delete.
	req = setup.CreateAuthRequest(http.MethodGet, "/api/users/me", nil, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "follow-up me request should complete")
	require.NotEqual(t, 200, resp.StatusCode, "old token should not work after account deletion")
	setup.LogTestPass(t, "TestMeDelete_Success")
}

// TestMeDelete_Unauthorized verifies the endpoint rejects unauthenticated callers.
func TestMeDelete_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestMeDelete_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	req := setup.CreateAuthRequest(http.MethodDelete, "/api/users/me", nil, "")
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "delete account request should complete")
	require.NotEqual(t, 200, resp.StatusCode, "unauthenticated delete must not succeed")
	setup.LogTestPass(t, "TestMeDelete_Unauthorized")
}

// TestMeDelete_Idempotent verifies a deleted user cannot be deleted twice
// (the second call should hit the "not found / already deleted" branch).
func TestMeDelete_Idempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestMeDelete_Idempotent")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "deletetwice@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodDelete, "/api/users/me", nil, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "first delete should succeed")
	setup.RequireStatus(t, resp, 200)

	req = setup.CreateAuthRequest(http.MethodDelete, "/api/users/me", nil, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "second delete request should complete")
	require.NotEqual(t, 200, resp.StatusCode, "second delete must not succeed with the now-stale token")
	setup.LogTestPass(t, "TestMeDelete_Idempotent")
}

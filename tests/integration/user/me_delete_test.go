package user

import (
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

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

	req = setup.CreateAuthRequest(http.MethodGet, "/api/users/me", nil, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "follow-up me request should complete")
	require.NotEqual(t, 200, resp.StatusCode, "old token should not work after account deletion")
	setup.LogTestPass(t, "TestMeDelete_Success")
}

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

func TestMeDelete_BlockedWhenOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestMeDelete_BlockedWhenOwner")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "owner-delete@example.com", "password123")
	setup.CreateTestServer(t, app, infra.RedisURL, token, "Owned Server", "owned", 1, false)

	req := setup.CreateAuthRequest(http.MethodDelete, "/api/users/me", nil, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "delete account request should complete")
	setup.RequireStatus(t, resp, http.StatusConflict)

	req = setup.CreateAuthRequest(http.MethodGet, "/api/users/me", nil, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "follow-up me request should complete")
	setup.RequireStatus(t, resp, 200)
	setup.LogTestPass(t, "TestMeDelete_BlockedWhenOwner")
}

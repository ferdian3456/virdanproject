package user

import (
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestUpdateNotificationPreferences_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUpdateNotificationPreferences_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "notifprefs@example.com", "password123")

	body := []byte(`{"notifLike":false,"notifComment":true,"notifReply":false}`)
	req := setup.CreateAuthRequest(http.MethodPut, "/api/users/me/notification-preferences", body, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "update notification preferences request should complete")
	setup.RequireStatus(t, resp, 200)
	setup.LogTestPass(t, "TestUpdateNotificationPreferences_Success")
}

func TestUpdateNotificationPreferences_AllToggled(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUpdateNotificationPreferences_AllToggled")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "notifprefs-all@example.com", "password123")

	body := []byte(`{"notifLike":true,"notifComment":true,"notifReply":true}`)
	req := setup.CreateAuthRequest(http.MethodPut, "/api/users/me/notification-preferences", body, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "update notification preferences request should complete")
	setup.RequireStatus(t, resp, 200)

	body = []byte(`{"notifLike":false,"notifComment":false,"notifReply":false}`)
	req = setup.CreateAuthRequest(http.MethodPut, "/api/users/me/notification-preferences", body, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "second update notification preferences request should complete")
	setup.RequireStatus(t, resp, 200)
	setup.LogTestPass(t, "TestUpdateNotificationPreferences_AllToggled")
}

func TestUpdateNotificationPreferences_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUpdateNotificationPreferences_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	body := []byte(`{"notifLike":true,"notifComment":true,"notifReply":true}`)
	req := setup.CreateAuthRequest(http.MethodPut, "/api/users/me/notification-preferences", body, "")
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "update notification preferences request should complete")
	require.NotEqual(t, 200, resp.StatusCode, "unauthenticated update must not succeed")
	setup.LogTestPass(t, "TestUpdateNotificationPreferences_Unauthorized")
}

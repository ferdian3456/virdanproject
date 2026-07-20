package chat

import (
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestWebSocket_MissingToken(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestWebSocket_MissingToken")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	req := setup.CreateJSONRequest(http.MethodGet, "/api/ws/", nil)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request should complete")
	require.NotEqual(t, http.StatusOK, resp.StatusCode, "ws endpoint must reject requests without a token")
	setup.LogTestPass(t, "TestWebSocket_MissingToken")
}

func TestWebSocket_NonUpgradeRequestRejected(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestWebSocket_NonUpgradeRequestRejected")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "ws-nonupgrade@example.com", "password123")

	req := setup.CreateJSONRequest(http.MethodGet, "/api/ws/?token="+token, nil)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request should complete")
	require.Equal(t, http.StatusUpgradeRequired, resp.StatusCode, "a plain HTTP request to the ws endpoint must be rejected")
	setup.LogTestPass(t, "TestWebSocket_NonUpgradeRequestRejected")
}

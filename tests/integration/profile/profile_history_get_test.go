package profile

import (
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestProfileHistoryGet_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestProfileHistoryGet_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "phistory@example.com", "password123")
	_ = setup.CreateTestServer(t, app, infra.RedisURL, token, "History Server", "history", 1, false)

	req := setup.CreateAuthRequest(http.MethodGet, "/api/profiles/history", nil, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "profile history request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "data", "response should contain data array")
	data, ok := result["data"].([]interface{})
	require.True(t, ok, "data should be an array")
	require.GreaterOrEqual(t, len(data), 1, "history should include the server we just created")

	first, ok := data[0].(map[string]interface{})
	require.True(t, ok, "history item should be an object")
	require.Contains(t, first, "profileId", "item should include profileId")
	require.Contains(t, first, "serverName", "item should include serverName")
	require.Contains(t, first, "nickname", "item should include nickname")
	require.Contains(t, first, "username", "item should include username")
	require.Contains(t, first, "isStillMember", "item should include isStillMember flag")
	setup.LogTestPass(t, "TestProfileHistoryGet_Success")
}

func TestProfileHistoryGet_Empty(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestProfileHistoryGet_Empty")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "phistoryempty@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodGet, "/api/profiles/history", nil, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "profile history request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "data", "response should contain data array")
	setup.LogTestPass(t, "TestProfileHistoryGet_Empty")
}

func TestProfileHistoryGet_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestProfileHistoryGet_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	req := setup.CreateAuthRequest(http.MethodGet, "/api/profiles/history", nil, "")
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "profile history request should complete")
	require.NotEqual(t, 200, resp.StatusCode, "unauthenticated request must not succeed")
	setup.LogTestPass(t, "TestProfileHistoryGet_Unauthorized")
}

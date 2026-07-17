package profile

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestServerProfileMeGet_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestServerProfileMeGet_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "spme@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, token, "Profile Me Server", "profme", 1, false)

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/profile/me", serverID), nil, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get server profile me should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "profileId", "response should contain profileId")
	require.Contains(t, result, "nickname", "response should contain nickname")
	require.Contains(t, result, "username", "response should contain username")
	require.Contains(t, result, "serverId", "response should contain serverId")
	require.Equal(t, serverID, result["serverId"], "serverId in response should match the path param")
	setup.LogTestPass(t, "TestServerProfileMeGet_Success")
}

func TestServerProfileMeGet_NotAMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestServerProfileMeGet_NotAMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "spme-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Profile Me Owner Server", "profmeo", 1, false)

	strangerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "spme-stranger@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/profile/me", serverID), nil, strangerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get server profile me should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "non-member should be rejected")
	setup.LogTestPass(t, "TestServerProfileMeGet_NotAMember")
}

func TestServerProfileMeGet_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestServerProfileMeGet_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/00000000-0000-0000-0000-000000000000/profile/me", nil, "")
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get server profile me should complete")
	require.NotEqual(t, 200, resp.StatusCode, "unauthenticated request must not succeed")
	setup.LogTestPass(t, "TestServerProfileMeGet_Unauthorized")
}

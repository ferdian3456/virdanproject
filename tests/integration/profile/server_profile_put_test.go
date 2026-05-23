package profile

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestServerProfilePut_Success updates the caller's per-server nickname,
// username, and bio, then re-fetches the profile to verify the persisted
// changes match the request.
func TestServerProfilePut_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestServerProfilePut_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "spput@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, token, "Profile Put Server", "profput", 1, false)

	body, contentType := setup.CreateMultipartTextOnly(t, map[string]string{
		"nickname": "UpdatedNick",
		"username": "updatednick",
		"bio":      "Updated bio after refactor",
	})
	req := setup.CreateAuthMultipartRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/profile", serverID), body, contentType, token)
	resp, err := app.Test(req)
	require.NoError(t, err, "update server profile should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "profileId", "response should contain profileId")
	require.Contains(t, result, "updatedAt", "response should contain updatedAt")

	// Re-fetch and confirm the new values landed.
	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/profile/me", serverID), nil, token)
	resp, err = app.Test(req)
	require.NoError(t, err, "follow-up get profile should succeed")
	setup.RequireStatus(t, resp, 200)

	result = setup.ParseJSONResponse(t, resp)
	require.Equal(t, "UpdatedNick", result["nickname"], "nickname should be persisted")
	require.Equal(t, "updatednick", result["username"], "username should be persisted")
	require.Equal(t, "Updated bio after refactor", result["bio"], "bio should be persisted")
	setup.LogTestPass(t, "TestServerProfilePut_Success")
}

// TestServerProfilePut_NotAMember rejects updates from non-members.
func TestServerProfilePut_NotAMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestServerProfilePut_NotAMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "spput-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Profile Put Owner Server", "profputo", 1, false)

	strangerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "spput-stranger@example.com", "password123")

	body, contentType := setup.CreateMultipartTextOnly(t, map[string]string{
		"nickname": "Hacker",
		"username": "hacker",
	})
	req := setup.CreateAuthMultipartRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/profile", serverID), body, contentType, strangerToken)
	resp, err := app.Test(req)
	require.NoError(t, err, "update server profile should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "non-member should be rejected")
	setup.LogTestPass(t, "TestServerProfilePut_NotAMember")
}

// TestServerProfilePut_DuplicateUsername rejects a username that is already
// taken by another member of the same server.
func TestServerProfilePut_DuplicateUsername(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestServerProfilePut_DuplicateUsername")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "spput-dup-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Dup Profile Server", "dupprof", 1, false)

	memberToken := setup.CreateTestUser(t, app, infra.MailhogURL, "spput-dup-member@example.com", "password123")
	setup.JoinTestServer(t, app, memberToken, serverID, "MemberOne", "memberone", "")

	// Member tries to take the owner's username (which was set via CreateServer
	// helper to the server's shortName, "dupprof").
	body, contentType := setup.CreateMultipartTextOnly(t, map[string]string{
		"nickname": "MemberClone",
		"username": "dupprof",
	})
	req := setup.CreateAuthMultipartRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/profile", serverID), body, contentType, memberToken)
	resp, err := app.Test(req)
	require.NoError(t, err, "duplicate-username update should complete")

	require.NotEqual(t, 200, resp.StatusCode, "duplicate username must not succeed")
	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "duplicate username should produce an error")
	setup.LogTestPass(t, "TestServerProfilePut_DuplicateUsername")
}

// TestServerProfilePut_Unauthorized rejects unauthenticated callers.
func TestServerProfilePut_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestServerProfilePut_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	body, contentType := setup.CreateMultipartTextOnly(t, map[string]string{
		"nickname": "Anon",
		"username": "anon",
	})
	req := setup.CreateAuthMultipartRequest(http.MethodPut, "/api/servers/00000000-0000-0000-0000-000000000000/profile", body, contentType, "")
	resp, err := app.Test(req)
	require.NoError(t, err, "update server profile should complete")
	require.NotEqual(t, 200, resp.StatusCode, "unauthenticated request must not succeed")
	setup.LogTestPass(t, "TestServerProfilePut_Unauthorized")
}

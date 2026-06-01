package profile

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestServerMemberProfileGet_Success lets a member view another member's
// per-server profile (view-only).
func TestServerMemberProfileGet_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestServerMemberProfileGet_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "smp-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Member Profile Server", "memprof", 1, false)

	memberToken := setup.CreateTestUser(t, app, infra.MailhogURL, "smp-member@example.com", "password123")
	setup.JoinTestServer(t, app, memberToken, serverID, "BudiPro", "budipro", "Always grinding")
	memberID := setup.GetUserId(t, app, memberToken)

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/members/%s/profile", serverID, memberID), nil, ownerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "view member profile should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Equal(t, "budipro", result["username"], "should return the target's per-server username")
	require.Equal(t, "BudiPro", result["nickname"], "should return the target's per-server nickname")
	require.Equal(t, serverID, result["serverId"], "serverId should match the path param")
	setup.LogTestPass(t, "TestServerMemberProfileGet_Success")
}

// TestServerMemberProfileGet_RequesterNotMember rejects a requester that is not
// a member of the server, preventing private-roster enumeration.
func TestServerMemberProfileGet_RequesterNotMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestServerMemberProfileGet_RequesterNotMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "smp2-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Member Profile Guard", "memprofg", 1, false)
	ownerID := setup.GetUserId(t, app, ownerToken)

	strangerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "smp2-stranger@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/members/%s/profile", serverID, ownerID), nil, strangerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request should complete")
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "non-member requester must be forbidden")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "should return an error payload")
	setup.LogTestPass(t, "TestServerMemberProfileGet_RequesterNotMember")
}

// TestServerMemberProfileGet_TargetNoProfile returns 404 when the target user
// has no profile in the server.
func TestServerMemberProfileGet_TargetNoProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestServerMemberProfileGet_TargetNoProfile")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "smp3-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Member Profile Missing", "memprofm", 1, false)

	// A real user who never joined the server: no profile row for this server.
	otherToken := setup.CreateTestUser(t, app, infra.MailhogURL, "smp3-other@example.com", "password123")
	otherID := setup.GetUserId(t, app, otherToken)

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/members/%s/profile", serverID, otherID), nil, ownerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request should complete")
	require.Equal(t, http.StatusNotFound, resp.StatusCode, "missing target profile must return 404")
	setup.LogTestPass(t, "TestServerMemberProfileGet_TargetNoProfile")
}

// TestServerMemberProfileGet_InvalidUserId returns 400 for a non-UUID userId.
func TestServerMemberProfileGet_InvalidUserId(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestServerMemberProfileGet_InvalidUserId")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "smp4-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Member Profile Invalid", "memprofi", 1, false)

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/members/not-a-uuid/profile", serverID), nil, ownerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request should complete")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "invalid userId must return 400")
	setup.LogTestPass(t, "TestServerMemberProfileGet_InvalidUserId")
}

// TestServerMemberProfileGet_Unauthorized rejects unauthenticated callers.
func TestServerMemberProfileGet_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestServerMemberProfileGet_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/00000000-0000-0000-0000-000000000000/members/00000000-0000-0000-0000-000000000000/profile", nil, "")
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request should complete")
	require.NotEqual(t, 200, resp.StatusCode, "unauthenticated request must not succeed")
	setup.LogTestPass(t, "TestServerMemberProfileGet_Unauthorized")
}

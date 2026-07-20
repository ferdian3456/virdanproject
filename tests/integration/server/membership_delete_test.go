package server

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestMembershipDelete_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestMembershipDelete_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "leaveowner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Leave Server", "leavesrv", 1, false)

	memberToken := setup.CreateTestUser(t, app, infra.MailhogURL, "leavemember@example.com", "password123")
	setup.JoinTestServer(t, app, memberToken, serverID, "Leaver", "leaver", "")

	req := setup.CreateAuthRequest(http.MethodDelete, fmt.Sprintf("/api/servers/%s/membership", serverID), nil, memberToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "leave server should succeed")
	setup.RequireStatus(t, resp, 200)
	setup.LogTestPass(t, "TestMembershipDelete_Success")
}

func TestMembershipDelete_NotAMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestMembershipDelete_NotAMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "leavenotmember-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "NotMember Leave Server", "leavenotm", 1, false)

	strangerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "leavenotmember@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodDelete, fmt.Sprintf("/api/servers/%s/membership", serverID), nil, strangerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "leave request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "non-member leave should produce an error")
	setup.LogTestPass(t, "TestMembershipDelete_NotAMember")
}

func TestMembershipDelete_OwnerCannotLeave(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestMembershipDelete_OwnerCannotLeave")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "ownercantleave@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Owner Leave Server", "ownerleav", 1, false)

	// LeaveServer lets a sole owner leave (deleting the server); the "owner
	// cannot leave" conflict only applies while other members still exist.
	memberToken := setup.CreateTestUser(t, app, infra.MailhogURL, "ownercantleave-member@example.com", "password123")
	setup.JoinTestServer(t, app, memberToken, serverID, "StayingMember", "stayingmember", "")

	req := setup.CreateAuthRequest(http.MethodDelete, fmt.Sprintf("/api/servers/%s/membership", serverID), nil, ownerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "leave request should complete")

	require.NotEqual(t, 200, resp.StatusCode, "owner must not be able to leave while other members exist")
	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "owner leave should produce an error")
	setup.LogTestPass(t, "TestMembershipDelete_OwnerCannotLeave")
}

func TestMembershipDelete_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestMembershipDelete_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	req := setup.CreateAuthRequest(http.MethodDelete, "/api/servers/00000000-0000-0000-0000-000000000000/membership", nil, "")
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "leave request should complete")
	require.NotEqual(t, 200, resp.StatusCode, "unauthenticated leave must not succeed")
	setup.LogTestPass(t, "TestMembershipDelete_Unauthorized")
}

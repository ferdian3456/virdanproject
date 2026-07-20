package server

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestGetServerMembers_Member(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestGetServerMembers_Member")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "members-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Members Server", "membsrv", 1, false)

	memberToken := setup.CreateTestUser(t, app, infra.MailhogURL, "members-member@example.com", "password123")
	setup.JoinTestServer(t, app, memberToken, serverID, "Membername", "membername", "")

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/members", serverID), nil, ownerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get members request should complete")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	data, ok := result["data"].([]interface{})
	require.True(t, ok, "response should contain data array, got: %v", result)
	require.Len(t, data, 2, "server should have owner + member")
	setup.LogTestPass(t, "TestGetServerMembers_Member")
}

func TestGetServerMembers_NotMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestGetServerMembers_NotMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "members-guard-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Members Guard Server", "membguard", 1, false)
	strangerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "members-stranger@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/members", serverID), nil, strangerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get members request should complete")
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "non-member must be forbidden")
	setup.LogTestPass(t, "TestGetServerMembers_NotMember")
}

func TestGetMyRoleInServer_Owner(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestGetMyRoleInServer_Owner")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "myrole-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "MyRole Owner Server", "myroleown", 1, false)

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/members/me", serverID), nil, ownerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get my role request should complete")
	result := setup.RequireJSONResponse(t, resp, 200)
	require.Equal(t, "Owner", result["role"], "creator should be Owner")
	setup.LogTestPass(t, "TestGetMyRoleInServer_Owner")
}

func TestGetMyRoleInServer_Member(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestGetMyRoleInServer_Member")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "myrole-owner2@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "MyRole Member Server", "myrolemem", 1, false)

	memberToken := setup.CreateTestUser(t, app, infra.MailhogURL, "myrole-member@example.com", "password123")
	setup.JoinTestServer(t, app, memberToken, serverID, "MyRoleMember", "myrolemember", "")

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/members/me", serverID), nil, memberToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get my role request should complete")
	result := setup.RequireJSONResponse(t, resp, 200)
	require.Equal(t, "Member", result["role"], "joiner should be Member")
	setup.LogTestPass(t, "TestGetMyRoleInServer_Member")
}

func TestGetMyRoleInServer_NotMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestGetMyRoleInServer_NotMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "myrole-guard-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "MyRole Guard Server", "myroleg", 1, false)
	strangerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "myrole-stranger@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/members/me", serverID), nil, strangerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get my role request should complete")
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "non-member must be forbidden")
	setup.LogTestPass(t, "TestGetMyRoleInServer_NotMember")
}

func TestKickMember_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestKickMember_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "kick-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Kick Server", "kicksrv", 1, false)

	memberToken := setup.CreateTestUser(t, app, infra.MailhogURL, "kick-member@example.com", "password123")
	setup.JoinTestServer(t, app, memberToken, serverID, "Kickme", "kickme", "")
	memberID := setup.GetUserId(t, app, memberToken)

	req := setup.CreateAuthRequest(http.MethodDelete, fmt.Sprintf("/api/servers/%s/members/%s", serverID, memberID), nil, ownerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "kick request should complete")
	setup.RequireStatus(t, resp, 200)

	// kicked member should no longer be a member
	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/members/me", serverID), nil, memberToken)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "get my role request should complete")
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "kicked member should no longer be a member")
	setup.LogTestPass(t, "TestKickMember_Success")
}

func TestKickMember_CannotKickSelf(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestKickMember_CannotKickSelf")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "kick-self-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Kick Self Server", "kickself", 1, false)
	ownerID := setup.GetUserId(t, app, ownerToken)

	req := setup.CreateAuthRequest(http.MethodDelete, fmt.Sprintf("/api/servers/%s/members/%s", serverID, ownerID), nil, ownerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "kick request should complete")
	require.NotEqual(t, 200, resp.StatusCode, "cannot kick self")
	setup.LogTestPass(t, "TestKickMember_CannotKickSelf")
}

func TestKickMember_CannotKickOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestKickMember_CannotKickOwner")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "kick-owner2-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Kick Owner Server", "kickownr", 1, false)
	ownerID := setup.GetUserId(t, app, ownerToken)

	adminToken := setup.CreateTestUser(t, app, infra.MailhogURL, "kick-owner2-admin@example.com", "password123")
	setup.JoinTestServer(t, app, adminToken, serverID, "Admin", "kickadmin", "")
	adminID := setup.GetUserId(t, app, adminToken)

	roleBody := []byte(`{"role":"Admin"}`)
	req := setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/members/%s/role", serverID, adminID), roleBody, ownerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "assign admin role should succeed")
	setup.RequireStatus(t, resp, 200)

	req = setup.CreateAuthRequest(http.MethodDelete, fmt.Sprintf("/api/servers/%s/members/%s", serverID, ownerID), nil, adminToken)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "kick owner request should complete")
	require.NotEqual(t, 200, resp.StatusCode, "owner cannot be kicked")
	setup.LogTestPass(t, "TestKickMember_CannotKickOwner")
}

func TestKickMember_Forbidden(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestKickMember_Forbidden")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "kick-forbid-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Kick Forbid Server", "kickforbid", 1, false)

	memberAToken := setup.CreateTestUser(t, app, infra.MailhogURL, "kick-forbid-a@example.com", "password123")
	setup.JoinTestServer(t, app, memberAToken, serverID, "MemberA", "kickmembera", "")

	memberBToken := setup.CreateTestUser(t, app, infra.MailhogURL, "kick-forbid-b@example.com", "password123")
	setup.JoinTestServer(t, app, memberBToken, serverID, "MemberB", "kickmemberb", "")
	memberBID := setup.GetUserId(t, app, memberBToken)

	req := setup.CreateAuthRequest(http.MethodDelete, fmt.Sprintf("/api/servers/%s/members/%s", serverID, memberBID), nil, memberAToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "kick request should complete")
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "plain member cannot kick another member")
	setup.LogTestPass(t, "TestKickMember_Forbidden")
}

func TestAssignMemberRole_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestAssignMemberRole_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "assign-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Assign Role Server", "assignrole", 1, false)

	memberToken := setup.CreateTestUser(t, app, infra.MailhogURL, "assign-member@example.com", "password123")
	setup.JoinTestServer(t, app, memberToken, serverID, "AssignMember", "assignmember", "")
	memberID := setup.GetUserId(t, app, memberToken)

	roleBody := []byte(`{"role":"Admin"}`)
	req := setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/members/%s/role", serverID, memberID), roleBody, ownerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "assign role request should complete")
	setup.RequireStatus(t, resp, 200)

	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/members/me", serverID), nil, memberToken)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "get my role request should complete")
	result := setup.RequireJSONResponse(t, resp, 200)
	require.Equal(t, "Admin", result["role"], "member should now be Admin")
	setup.LogTestPass(t, "TestAssignMemberRole_Success")
}

func TestAssignMemberRole_InvalidRole(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestAssignMemberRole_InvalidRole")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "assign-invalid-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Assign Invalid Server", "assigninv", 1, false)

	memberToken := setup.CreateTestUser(t, app, infra.MailhogURL, "assign-invalid-member@example.com", "password123")
	setup.JoinTestServer(t, app, memberToken, serverID, "InvMember", "invmember", "")
	memberID := setup.GetUserId(t, app, memberToken)

	roleBody := []byte(`{"role":"Owner"}`)
	req := setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/members/%s/role", serverID, memberID), roleBody, ownerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "assign role request should complete")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "cannot assign Owner role via this endpoint")
	setup.LogTestPass(t, "TestAssignMemberRole_InvalidRole")
}

func TestAssignMemberRole_NotOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestAssignMemberRole_NotOwner")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "assign-notowner-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Assign NotOwner Server", "assignno", 1, false)

	memberToken := setup.CreateTestUser(t, app, infra.MailhogURL, "assign-notowner-member@example.com", "password123")
	setup.JoinTestServer(t, app, memberToken, serverID, "PlainMember", "plainmember", "")
	memberID := setup.GetUserId(t, app, memberToken)

	roleBody := []byte(`{"role":"Admin"}`)
	req := setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/members/%s/role", serverID, memberID), roleBody, memberToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "assign role request should complete")
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "only owner can assign roles")
	setup.LogTestPass(t, "TestAssignMemberRole_NotOwner")
}

func TestTransferOwnership_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestTransferOwnership_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "transfer-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Transfer Server", "transfer1", 1, false)

	memberToken := setup.CreateTestUser(t, app, infra.MailhogURL, "transfer-member@example.com", "password123")
	setup.JoinTestServer(t, app, memberToken, serverID, "NewOwner", "newowner", "")
	memberID := setup.GetUserId(t, app, memberToken)

	transferBody := []byte(fmt.Sprintf(`{"newOwnerId":%q}`, memberID))
	req := setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/ownership", serverID), transferBody, ownerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "transfer ownership request should complete")
	setup.RequireStatus(t, resp, 200)

	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/members/me", serverID), nil, memberToken)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "get my role request should complete")
	result := setup.RequireJSONResponse(t, resp, 200)
	require.Equal(t, "Owner", result["role"], "new owner should have Owner role")

	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/members/me", serverID), nil, ownerToken)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "get my role request should complete")
	result = setup.RequireJSONResponse(t, resp, 200)
	require.Equal(t, "Admin", result["role"], "previous owner should be demoted to Admin")
	setup.LogTestPass(t, "TestTransferOwnership_Success")
}

func TestTransferOwnership_NotOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestTransferOwnership_NotOwner")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "transfer-notowner-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Transfer NotOwner Server", "transnoto", 1, false)

	memberToken := setup.CreateTestUser(t, app, infra.MailhogURL, "transfer-notowner-member@example.com", "password123")
	setup.JoinTestServer(t, app, memberToken, serverID, "NotOwnerMember", "notownermember", "")
	memberID := setup.GetUserId(t, app, memberToken)

	transferBody := []byte(fmt.Sprintf(`{"newOwnerId":%q}`, memberID))
	req := setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/ownership", serverID), transferBody, memberToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "transfer ownership request should complete")
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "only owner can transfer ownership")
	setup.LogTestPass(t, "TestTransferOwnership_NotOwner")
}

func TestTransferOwnership_TargetNotMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestTransferOwnership_TargetNotMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "transfer-notmember-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Transfer NotMember Server", "transnotm", 1, false)

	strangerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "transfer-notmember-stranger@example.com", "password123")
	strangerID := setup.GetUserId(t, app, strangerToken)

	transferBody := []byte(fmt.Sprintf(`{"newOwnerId":%q}`, strangerID))
	req := setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/ownership", serverID), transferBody, ownerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "transfer ownership request should complete")
	require.Equal(t, http.StatusNotFound, resp.StatusCode, "new owner must already be a member")
	setup.LogTestPass(t, "TestTransferOwnership_TargetNotMember")
}

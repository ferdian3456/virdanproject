package server

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestInvitesPost_Success tests successful invite link creation
func TestInvitesPost_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestInvitesPost_Success")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	// Setup: Create user and server
	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "invite@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Invite Server", "invite", 1, false)

	// Test: Create invite link
	setup.LogTestStep(t, "Testing Create Invite Link")
	reqBody := []byte(`{"expiresInMinutes":60,"maxUses":10}`)
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/invites", serverID), reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create invite link request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "code", "response should contain invite code")
	require.Contains(t, result, "inviteUrl", "response should contain invite url")

	inviteCode := result["code"].(string)
	require.NotEmpty(t, inviteCode, "invite code should not be empty")

	t.Logf("Invite link created successfully: %s", inviteCode)

	setup.LogTestPass(t, "TestInvitesPost_Success")
}

// TestInvitesPost_Unauthorized tests invite link creation without authentication
func TestInvitesPost_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestInvitesPost_Unauthorized")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "unauthinvite@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Unauth Invite Server", "uninvite", 1, false)

	// Test: Create invite link without token
	setup.LogTestStep(t, "Testing Create Invite Link Without Auth")
	reqBody := []byte(`{"expiresInMinutes":60,"maxUses":10}`)
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/invites", serverID), reqBody, "")
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create invite link request should complete")

	// Should return unauthorized
	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated invite link creation request")

	setup.LogTestPass(t, "TestInvitesPost_Unauthorized")
}

// TestInvitesPost_NotAMember tests invite link creation when user is not a member
func TestInvitesPost_NotAMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestInvitesPost_NotAMember")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	// Setup: Create user and server (user is NOT a member)
	token1 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "ownerinvite@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token1, "Owner Invite Server", "ownerinvite", 1, false)

	// Create another user (not a member of the server)
	token2 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "nonmemberinvite@example.com", "password123")

	// Test: Try to create invite link as non-member
	setup.LogTestStep(t, "Testing Create Invite Link as Non-Member")
	reqBody := []byte(`{"expiresInMinutes":60,"maxUses":10}`)
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/invites", serverID), reqBody, token2)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create invite link request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "Not a member of this server", "error message should mention not a member")

	t.Logf("Correctly rejected invite link creation by non-member: %s", errMsg)

	setup.LogTestPass(t, "TestInvitesPost_NotAMember")
}

// TestInvitesPost_InvalidServerId tests invite link creation with invalid server ID
func TestInvitesPost_InvalidServerId(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestInvitesPost_InvalidServerId")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "invalidserver@example.com", "password123")

	// Test: Create invite link with invalid server ID
	setup.LogTestStep(t, "Testing Create Invite Link with Invalid Server ID")
	reqBody := []byte(`{"expiresInMinutes":60,"maxUses":10}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/servers/00000000-0000-0000-0000-000000000000/invites", reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create invite link request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	t.Logf("Error message for invalid server: %s", errMsg)

	setup.LogTestPass(t, "TestInvitesPost_InvalidServerId")
}

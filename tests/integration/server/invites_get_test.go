package server

import (
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestGetInviteInfo_Success tests successful get server info for invite
func TestGetInviteInfo_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestGetInviteInfo_Success")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	// Setup: Create user and server with invite
	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "inviteinfo@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Invite Info Server", "inviteinfo", 1, false)

	// Create invite link
	reqBody := []byte(`{"expiresInMinutes":60,"maxUses":10}`)
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/invites", serverID), reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create invite link should succeed")
	setup.RequireStatus(t, resp, 200)
	result := setup.ParseJSONResponse(t, resp)
	inviteCode := result["inviteCode"].(string)

	// Test: Get server info for invite (public endpoint, no auth needed)
	setup.LogTestStep(t, "Testing Get Server Info for Invite")
	t.Logf("Using invite code: %s", inviteCode)
	req = setup.CreateJSONRequest(http.MethodGet, "/api/servers/invites/"+inviteCode, nil)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get invite info request should succeed")
	if resp.StatusCode != 200 {
		t.Logf("Expected 200, got %d", resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		t.Logf("Response body: %s", string(body))
	}
	setup.RequireStatus(t, resp, 200)

	result = setup.ParseJSONResponse(t, resp)
	// API returns serverName instead of name
	require.Contains(t, result, "serverName", "response should contain server name")

	t.Logf("Server info for invite retrieved successfully")

	setup.LogTestPass(t, "TestGetInviteInfo_Success")
}

// TestGetInviteInfo_InvalidInviteCode tests get server info with invalid invite code
func TestGetInviteInfo_InvalidInviteCode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestGetInviteInfo_InvalidInviteCode")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Test: Get server info with invalid invite code
	setup.LogTestStep(t, "Testing Get Server Info with Invalid Invite Code")
	req := setup.CreateJSONRequest(http.MethodGet, "/api/servers/invites/INVALID1", nil)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get invite info request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	t.Logf("Error message for invalid invite code: %s", errMsg)

	setup.LogTestPass(t, "TestGetInviteInfo_InvalidInviteCode")
}

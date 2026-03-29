package server

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestJoinFromInvitePost_Success tests successful server join from invite code
func TestJoinFromInvitePost_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestJoinFromInvitePost_Success")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	// Setup: Create user and server with invite
	token1 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "inviteowner@example.com", "invowner", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token1, "Join Server", "joinserver", 1, false)

	// Create invite link
	reqBody := []byte(`{"expiresInMinutes":60,"maxUses":10}`)
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/invites", serverID), reqBody, token1)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create invite link should succeed")
	setup.RequireStatus(t, resp, 200)
	result := setup.ParseJSONResponse(t, resp)
	inviteCode := result["inviteCode"].(string)

	// Create another user
	token2 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "joiner@example.com", "joiner", "password123")

	// Test: Join server from invite
	setup.LogTestStep(t, "Testing Join Server from Invite Code")
	reqBody = []byte(fmt.Sprintf(`{"inviteCode":"%s"}`, inviteCode))
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/join", reqBody, token2)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "join server request should succeed")
	setup.RequireStatus(t, resp, 200)

	t.Logf("Server joined successfully from invite")

	setup.LogTestPass(t, "TestJoinFromInvitePost_Success")
}

// TestJoinFromInvitePost_Unauthorized tests server join without authentication
func TestJoinFromInvitePost_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestJoinFromInvitePost_Unauthorized")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Test: Join server without token
	setup.LogTestStep(t, "Testing Join Server Without Auth")
	reqBody := []byte(`{"inviteCode":"ABC12345"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/servers/join", reqBody, "")
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "join server request should complete")

	// Should return unauthorized
	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated join server request")

	setup.LogTestPass(t, "TestJoinFromInvitePost_Unauthorized")
}

// TestJoinFromInvitePost_InvalidInviteCode tests server join with invalid invite code
func TestJoinFromInvitePost_InvalidInviteCode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestJoinFromInvitePost_InvalidInviteCode")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "invalidjoin@example.com", "invalidjoinuser", "password123")

	// Test: Join server with invalid invite code
	setup.LogTestStep(t, "Testing Join Server with Invalid Invite Code")
	reqBody := []byte(`{"inviteCode":"INVALID1"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/servers/join", reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "join server request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	t.Logf("Error message for invalid invite code: %s", errMsg)

	setup.LogTestPass(t, "TestJoinFromInvitePost_InvalidInviteCode")
}

// TestJoinFromInvitePost_AlreadyMember tests server join when user is already a member
func TestJoinFromInvitePost_AlreadyMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestJoinFromInvitePost_AlreadyMember")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	// Setup: Create user and server with invite
	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "alreadyjoin@example.com", "alreadyjoinuser", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Already Join Server", "alreadyjoin", 1, false)

	// Create invite link
	reqBody := []byte(`{"expiresInMinutes":60,"maxUses":10}`)
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/invites", serverID), reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create invite link should succeed")
	setup.RequireStatus(t, resp, 200)
	result := setup.ParseJSONResponse(t, resp)
	inviteCode := result["inviteCode"].(string)

	// Test: Try to join server when already a member
	setup.LogTestStep(t, "Testing Join Server When Already Member")
	reqBody = []byte(fmt.Sprintf(`{"inviteCode":"%s"}`, inviteCode))
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/join", reqBody, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "join server request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "already a member", "error message should mention already a member")

	t.Logf("Correctly rejected join request from existing member: %s", errMsg)

	setup.LogTestPass(t, "TestJoinFromInvitePost_AlreadyMember")
}

// TestJoinServer_Success tests successful public server join
func TestJoinServer_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestJoinServer_Success")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	// Setup: Create users and public server
	token1 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "publicowner@example.com", "pubowner", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token1, "Public Server", "public", 1, false)

	// Create another user
	token2 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "publicjoiner@example.com", "pubjoiner", "password123")

	// Test: Join public server
	setup.LogTestStep(t, "Testing Join Public Server")
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/join", serverID), nil, token2)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "join server request should succeed")
	setup.RequireStatus(t, resp, 200)

	t.Logf("Public server joined successfully")

	setup.LogTestPass(t, "TestJoinServer_Success")
}

// TestJoinServer_PrivateServer tests joining private server (should fail)
func TestJoinServer_PrivateServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestJoinServer_PrivateServer")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	// Setup: Create users and private server
	token1 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "privateowner@example.com", "privowner", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token1, "Private Server", "private", 1, true)

	// Create another user
	token2 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "privatejoiner@example.com", "privjoiner", "password123")

	// Test: Try to join private server
	setup.LogTestStep(t, "Testing Join Private Server (Should Fail)")
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/join", serverID), nil, token2)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "join server request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "private", "error message should mention server is private")

	t.Logf("Correctly rejected join request to private server: %s", errMsg)

	setup.LogTestPass(t, "TestJoinServer_PrivateServer")
}

// TestJoinServer_AlreadyMember tests joining when already a member
func TestJoinServer_AlreadyMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestJoinServer_AlreadyMember")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	// Setup: Create user and server (user is automatically a member as creator)
	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "alreadyjoindirect@example.com", "alreadyjoindirectuser", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Already Join Direct Server", "alreadyjoin", 1, false)

	// Test: Try to join server when already a member
	setup.LogTestStep(t, "Testing Join Server When Already Member")
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/join", serverID), nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "join server request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "already a member", "error message should mention already a member")

	t.Logf("Correctly rejected join request from existing member: %s", errMsg)

	setup.LogTestPass(t, "TestJoinServer_AlreadyMember")
}

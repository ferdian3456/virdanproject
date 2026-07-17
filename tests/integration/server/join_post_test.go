package server

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestJoinFromInvitePost_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestJoinFromInvitePost_Success")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token1 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "inviteowner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token1, "Join Server", "joinserver", 1, false)

	reqBody := []byte(`{"expiresInMinutes":60,"maxUses":10}`)
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/invites", serverID), reqBody, token1)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create invite link should succeed")
	setup.RequireStatus(t, resp, 200)
	result := setup.ParseJSONResponse(t, resp)
	inviteCode := result["code"].(string)

	token2 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "joiner@example.com", "password123")

	setup.LogTestStep(t, "Testing Join Server from Invite Code")
	reqBody = []byte(fmt.Sprintf(`{"inviteCode":"%s","nickname":"Joiner","username":"joiner"}`, inviteCode))
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/join", reqBody, token2)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "join server request should succeed")
	setup.RequireStatus(t, resp, 200)

	t.Logf("Server joined successfully from invite")

	setup.LogTestPass(t, "TestJoinFromInvitePost_Success")
}

func TestJoinFromInvitePost_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestJoinFromInvitePost_Unauthorized")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	setup.LogTestStep(t, "Testing Join Server Without Auth")
	reqBody := []byte(`{"inviteCode":"ABC12345"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/servers/join", reqBody, "")
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "join server request should complete")

	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated join server request")

	setup.LogTestPass(t, "TestJoinFromInvitePost_Unauthorized")
}

func TestJoinFromInvitePost_InvalidInviteCode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestJoinFromInvitePost_InvalidInviteCode")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "invalidjoin@example.com", "password123")

	setup.LogTestStep(t, "Testing Join Server with Invalid Invite Code")
	reqBody := []byte(`{"inviteCode":"INVALID1","nickname":"InvalidJoiner","username":"invalidjoiner"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/servers/join", reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "join server request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	t.Logf("Error message for invalid invite code: %s", errMsg)

	setup.LogTestPass(t, "TestJoinFromInvitePost_InvalidInviteCode")
}

func TestJoinFromInvitePost_AlreadyMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestJoinFromInvitePost_AlreadyMember")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "alreadyjoin@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Already Join Server", "alrdyjoin", 1, false)

	reqBody := []byte(`{"expiresInMinutes":60,"maxUses":10}`)
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/invites", serverID), reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create invite link should succeed")
	setup.RequireStatus(t, resp, 200)
	result := setup.ParseJSONResponse(t, resp)
	inviteCode := result["code"].(string)

	setup.LogTestStep(t, "Testing Join Server When Already Member")
	reqBody = []byte(fmt.Sprintf(`{"inviteCode":"%s","nickname":"AlreadyJoin","username":"alreadyjoinx"}`, inviteCode))
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/join", reqBody, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "join server request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "Already a member", "error message should mention already a member")

	t.Logf("Correctly rejected join request from existing member: %s", errMsg)

	setup.LogTestPass(t, "TestJoinFromInvitePost_AlreadyMember")
}

func TestJoinServer_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestJoinServer_Success")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token1 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "publicowner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token1, "Public Server", "public", 1, false)

	token2 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "publicjoiner@example.com", "password123")

	setup.LogTestStep(t, "Testing Join Public Server")
	body, contentType := setup.CreateMultipartTextOnly(t, map[string]string{
		"nickname": "PublicJoiner",
		"username": "publicjoiner",
	})
	req := setup.CreateAuthMultipartRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/join", serverID), body, contentType, token2)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "join server request should succeed")
	setup.RequireStatus(t, resp, 200)

	t.Logf("Public server joined successfully")

	setup.LogTestPass(t, "TestJoinServer_Success")
}

func TestJoinServer_PrivateServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestJoinServer_PrivateServer")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token1 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "privateowner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token1, "Private Server", "private", 1, true)

	token2 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "privatejoiner@example.com", "password123")

	setup.LogTestStep(t, "Testing Join Private Server (Should Fail)")
	body, contentType := setup.CreateMultipartTextOnly(t, map[string]string{
		"nickname": "PrivateJoiner",
		"username": "privatejoiner",
	})
	req := setup.CreateAuthMultipartRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/join", serverID), body, contentType, token2)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "join server request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "private", "error message should mention server is private")

	t.Logf("Correctly rejected join request to private server: %s", errMsg)

	setup.LogTestPass(t, "TestJoinServer_PrivateServer")
}

func TestJoinServer_AlreadyMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestJoinServer_AlreadyMember")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "alreadyjoindirect@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Already Join Direct Server", "alrdyjnd", 1, false)

	setup.LogTestStep(t, "Testing Join Server When Already Member")
	body, contentType := setup.CreateMultipartTextOnly(t, map[string]string{
		"nickname": "OwnerRejoin",
		"username": "ownerrejoinx",
	})
	req := setup.CreateAuthMultipartRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/join", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "join server request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "Already a member", "error message should mention already a member")

	t.Logf("Correctly rejected join request from existing member: %s", errMsg)

	setup.LogTestPass(t, "TestJoinServer_AlreadyMember")
}

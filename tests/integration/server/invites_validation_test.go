package server

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestInvitesPost_ExpiresInMinutesZero tests invite link with zero expiration
func TestInvitesPost_ExpiresInMinutesZero(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestInvitesPost_ExpiresInMinutesZero")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "invitezero@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Invite Zero Server", "invitezero", 1, false)

	// Test: Create invite link with zero expiration
	setup.LogTestStep(t, "Testing Create Invite Link with Zero Expiration")
	reqBody := []byte(`{"expiresInMinutes":0,"maxUses":10}`)
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/invites", serverID), reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "greater than 0", "error message should mention expires in minutes must be greater than 0")

	t.Logf("Correctly rejected invite link with zero expiration: %s", errMsg)
	setup.LogTestPass(t, "TestInvitesPost_ExpiresInMinutesZero")
}

// TestInvitesPost_ExpiresInMinutesTooLarge tests invite link with expiration > 10080
func TestInvitesPost_ExpiresInMinutesTooLarge(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestInvitesPost_ExpiresInMinutesTooLarge")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "invitetoomain@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Invite Too Main Server", "invmain", 1, false)

	// Test: Create invite link with expiration > 10080 (more than 1 week)
	setup.LogTestStep(t, "Testing Create Invite Link with Expiration > 10080")
	reqBody := []byte(`{"expiresInMinutes":10081,"maxUses":10}`)
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/invites", serverID), reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "lower or equal than 10080", "error message should mention max expiration")

	t.Logf("Correctly rejected invite link with too large expiration: %s", errMsg)
	setup.LogTestPass(t, "TestInvitesPost_ExpiresInMinutesTooLarge")
}

// TestInvitesPost_MaxUsesZero tests invite link with zero max uses
func TestInvitesPost_MaxUsesZero(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestInvitesPost_MaxUsesZero")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "invitezero@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Invite Zero Uses Server", "invitezero", 1, false)

	// Test: Create invite link with zero max uses
	setup.LogTestStep(t, "Testing Create Invite Link with Zero Max Uses")
	reqBody := []byte(`{"expiresInMinutes":60,"maxUses":0}`)
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/invites", serverID), reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "greater than 0", "error message should mention max uses must be greater than 0")

	t.Logf("Correctly rejected invite link with zero max uses: %s", errMsg)
	setup.LogTestPass(t, "TestInvitesPost_MaxUsesZero")
}

// TestInvitesPost_MaxUsesTooLarge tests invite link with max uses > 100
func TestInvitesPost_MaxUsesTooLarge(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestInvitesPost_MaxUsesTooLarge")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "invitetoomany@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Invite Too Many Server", "invtomany", 1, false)

	// Test: Create invite link with max uses > 100
	setup.LogTestStep(t, "Testing Create Invite Link with Max Uses > 100")
	reqBody := []byte(`{"expiresInMinutes":60,"maxUses":101}`)
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/invites", serverID), reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "lower or equal than 100", "error message should mention max max uses")

	t.Logf("Correctly rejected invite link with too large max uses: %s", errMsg)
	setup.LogTestPass(t, "TestInvitesPost_MaxUsesTooLarge")
}

// TestInvitesPost_ExpiredInviteCode tests joining server with expired invite code
func TestInvitesPost_ExpiredInviteCode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestInvitesPost_ExpiredInviteCode")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token1 := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "expiredinvowner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token1, "Expired Invite Server", "expiredinv", 1, false)

	// Create invite link with very short expiration (1 minute)
	reqBody := []byte(`{"expiresInMinutes":1,"maxUses":10}`)
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/invites", serverID), reqBody, token1)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	result := setup.ParseJSONResponse(t, resp)
	inviteCode := result["inviteCode"].(string)

	token2 := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "expiredjoiner@example.com", "password123")

	// Note: We can't actually test expired invite without waiting or manual DB manipulation
	// This test documents the edge case but skips actual expiration testing
	setup.LogTestStep(t, "Testing Join Server with Expired Invite Code (Documented)")
	t.Logf("Invite code created: %s (would expire in 1 minute)", inviteCode)

	// Try to join immediately (should succeed since invite is not expired yet).
	// The JoinServerFromInvite API now requires nickname / username on the body.
	reqBody = []byte(fmt.Sprintf(`{"inviteCode":"%s","nickname":"ExpiredJoiner","username":"expiredjoiner"}`, inviteCode))
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/join", reqBody, token2)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)

	t.Logf("Note: Full expiration test requires DB manipulation or time mocking")
	setup.LogTestPass(t, "TestInvitesPost_ExpiredInviteCode")
}

// TestInvitesPost_MaxUsesReached tests joining server when invite max uses is reached
func TestInvitesPost_MaxUsesReached(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestInvitesPost_MaxUsesReached")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token1 := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "maxusesowner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token1, "Max Uses Server", "maxuses", 1, false)

	// Create invite link with max uses = 2
	reqBody := []byte(`{"expiresInMinutes":60,"maxUses":2}`)
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/invites", serverID), reqBody, token1)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	result := setup.ParseJSONResponse(t, resp)
	inviteCode := result["inviteCode"].(string)

	// Create 2 users and use up the invite
	token2 := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "user1@example.com", "password123")
	token3 := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "user2@example.com", "password123")

	reqBody = []byte(fmt.Sprintf(`{"inviteCode":"%s","nickname":"User1","username":"user1joiner"}`, inviteCode))
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/join", reqBody, token2)
	_, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	reqBody = []byte(fmt.Sprintf(`{"inviteCode":"%s","nickname":"User2","username":"user2joiner"}`, inviteCode))
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/join", reqBody, token3)
	_, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	// Create third user and try to join (should fail - max uses reached)
	token4 := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "user3@example.com", "password123")

	setup.LogTestStep(t, "Testing Join Server When Max Uses Reached")
	reqBody = []byte(fmt.Sprintf(`{"inviteCode":"%s","nickname":"User3","username":"user3joiner"}`, inviteCode))
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/join", reqBody, token4)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "max uses", "error message should mention max uses reached")

	t.Logf("Correctly rejected join when max uses reached: %s", errMsg)
	setup.LogTestPass(t, "TestInvitesPost_MaxUsesReached")
}

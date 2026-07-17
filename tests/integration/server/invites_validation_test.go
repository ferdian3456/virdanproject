package server

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

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

	reqBody := []byte(`{"maxUses":101}`)
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/invites", serverID), reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "Max uses cannot exceed 100", "error message should mention the max-uses ceiling")
	setup.LogTestPass(t, "TestInvitesPost_MaxUsesTooLarge")
}

func TestInvitesPost_MaxUsesDefaultsWhenZero(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestInvitesPost_MaxUsesDefaultsWhenZero")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "invitedefault@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Invite Default Server", "invdef", 1, false)

	reqBody := []byte(`{"maxUses":0}`)
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/invites", serverID), reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "code", "response should contain invite code")
	require.Equal(t, float64(10), result["maxUses"], "maxUses should default to 10 when caller sends 0")
	setup.LogTestPass(t, "TestInvitesPost_MaxUsesDefaultsWhenZero")
}

func TestInvitesPost_MaxUsesReached(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestInvitesPost_MaxUsesReached")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token1 := setup.CreateTestUser(t, app, infra.MailhogURL, "maxusesowner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, token1, "Max Uses Server", "maxuses", 1, false)

	reqBody := []byte(`{"maxUses":2}`)
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/invites", serverID), reqBody, token1)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)
	result := setup.ParseJSONResponse(t, resp)
	inviteCode := result["code"].(string)

	token2 := setup.CreateTestUser(t, app, infra.MailhogURL, "maxuses-user1@example.com", "password123")
	token3 := setup.CreateTestUser(t, app, infra.MailhogURL, "maxuses-user2@example.com", "password123")

	reqBody = []byte(fmt.Sprintf(`{"inviteCode":"%s","nickname":"User1","username":"user1max"}`, inviteCode))
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/join", reqBody, token2)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)

	reqBody = []byte(fmt.Sprintf(`{"inviteCode":"%s","nickname":"User2","username":"user2max"}`, inviteCode))
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/join", reqBody, token3)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)

	token4 := setup.CreateTestUser(t, app, infra.MailhogURL, "maxuses-user3@example.com", "password123")
	setup.LogTestStep(t, "Testing Join Server When Max Uses Reached")
	reqBody = []byte(fmt.Sprintf(`{"inviteCode":"%s","nickname":"User3","username":"user3max"}`, inviteCode))
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/join", reqBody, token4)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "max uses", "error message should mention max uses reached")
	setup.LogTestPass(t, "TestInvitesPost_MaxUsesReached")
}

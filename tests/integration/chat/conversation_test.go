package chat

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestGetOrCreateConversation_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestGetOrCreateConversation_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	aToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-conv-a@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, aToken, "DM Conv Server", "dmconvsrv", 1, false)

	bToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-conv-b@example.com", "password123")
	setup.JoinTestServer(t, app, bToken, serverID, "PeerB", "dmpeerb", "")
	bID := setup.GetUserId(t, app, bToken)

	body := []byte(fmt.Sprintf(`{"peerUserId":%q}`, bID))
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/conversations", serverID), body, aToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get-or-create conversation request should complete")
	result := setup.RequireJSONResponse(t, resp, 200)
	convID, ok := result["id"].(string)
	require.True(t, ok, "response should contain conversation id, got: %v", result)
	require.NotEmpty(t, convID)
	require.Equal(t, bID, result["peerUserId"])

	// Calling again must be idempotent and return the same conversation.
	req = setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/conversations", serverID), body, aToken)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "second get-or-create conversation request should complete")
	result = setup.RequireJSONResponse(t, resp, 200)
	require.Equal(t, convID, result["id"], "conversation lookup should be idempotent")
	setup.LogTestPass(t, "TestGetOrCreateConversation_Success")
}

func TestGetOrCreateConversation_WithSelf(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestGetOrCreateConversation_WithSelf")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-conv-self@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, token, "DM Conv Self Server", "dmconvself", 1, false)
	userID := setup.GetUserId(t, app, token)

	body := []byte(fmt.Sprintf(`{"peerUserId":%q}`, userID))
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/conversations", serverID), body, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request should complete")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "cannot start a conversation with yourself")
	setup.LogTestPass(t, "TestGetOrCreateConversation_WithSelf")
}

func TestGetOrCreateConversation_PeerNotMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestGetOrCreateConversation_PeerNotMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-conv-peernm@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, token, "DM Conv PeerNM Server", "dmconvpnm", 1, false)

	outsiderToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-conv-outsider@example.com", "password123")
	outsiderID := setup.GetUserId(t, app, outsiderToken)

	body := []byte(fmt.Sprintf(`{"peerUserId":%q}`, outsiderID))
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/conversations", serverID), body, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request should complete")
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "peer must be a member of the server")
	setup.LogTestPass(t, "TestGetOrCreateConversation_PeerNotMember")
}

func TestGetOrCreateConversation_CallerNotMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestGetOrCreateConversation_CallerNotMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-conv-callernm-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "DM Conv CallerNM Server", "dmconvcnm", 1, false)
	ownerID := setup.GetUserId(t, app, ownerToken)

	strangerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-conv-callernm-stranger@example.com", "password123")

	body := []byte(fmt.Sprintf(`{"peerUserId":%q}`, ownerID))
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/conversations", serverID), body, strangerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request should complete")
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "caller must be a member of the server")
	setup.LogTestPass(t, "TestGetOrCreateConversation_CallerNotMember")
}

func TestListMembers_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestListMembers_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	aToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-members-a@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, aToken, "DM Members Server", "dmmemsrv", 1, false)

	bToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-members-b@example.com", "password123")
	setup.JoinTestServer(t, app, bToken, serverID, "DmPeer", "dmmemberb", "")

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/members/dm", serverID), nil, aToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "list dm members request should complete")
	result := setup.RequireJSONResponse(t, resp, 200)
	data, ok := result["data"].([]interface{})
	require.True(t, ok, "response should contain data array, got: %v", result)
	require.Len(t, data, 1, "list should contain the other member, excluding caller")
	setup.LogTestPass(t, "TestListMembers_Success")
}

func TestListMembers_NotMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestListMembers_NotMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-members-guard-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "DM Members Guard Server", "dmmemguard", 1, false)
	strangerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-members-guard-stranger@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/members/dm", serverID), nil, strangerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "list dm members request should complete")
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "non-member must be forbidden")
	setup.LogTestPass(t, "TestListMembers_NotMember")
}

func TestListConversations_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestListConversations_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	aToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-listconv-a@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, aToken, "DM ListConv Server", "dmlistconv", 1, false)

	bToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-listconv-b@example.com", "password123")
	setup.JoinTestServer(t, app, bToken, serverID, "ListConvPeer", "dmlistconvb", "")
	bID := setup.GetUserId(t, app, bToken)

	body := []byte(fmt.Sprintf(`{"peerUserId":%q}`, bID))
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/conversations", serverID), body, aToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get-or-create conversation should succeed")
	convResult := setup.RequireJSONResponse(t, resp, 200)
	convID, ok := convResult["id"].(string)
	require.True(t, ok, "conversation id should be a string")

	// ListConversations only surfaces conversations that have at least one
	// message (last_message_at IS NOT NULL), so an empty conversation from
	// GetOrCreateConversation alone would not appear yet.
	msgBody := []byte(fmt.Sprintf(`{"content":"hi","clientMessageId":%q}`, uuid.New().String()))
	req = setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/conversations/%s/messages", convID), msgBody, aToken)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "send message should succeed")
	setup.RequireStatus(t, resp, 200)

	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/conversations", serverID), nil, aToken)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "list conversations request should complete")
	result := setup.RequireJSONResponse(t, resp, 200)
	data, ok := result["data"].([]interface{})
	require.True(t, ok, "response should contain data array, got: %v", result)
	require.Len(t, data, 1, "caller should see the one conversation created")
	setup.LogTestPass(t, "TestListConversations_Success")
}

func TestListConversations_NotMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestListConversations_NotMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-listconv-guard-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "DM ListConv Guard Server", "dmlistcvg", 1, false)
	strangerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-listconv-guard-stranger@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/conversations", serverID), nil, strangerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "list conversations request should complete")
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "non-member must be forbidden")
	setup.LogTestPass(t, "TestListConversations_NotMember")
}

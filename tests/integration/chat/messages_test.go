package chat

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func getOrCreateConversation(t *testing.T, app *fiber.App, serverID, callerToken, peerID string) string {
	t.Helper()
	body := []byte(fmt.Sprintf(`{"peerUserId":%q}`, peerID))
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/conversations", serverID), body, callerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get-or-create conversation should succeed")
	result := setup.RequireJSONResponse(t, resp, 200)
	convID, ok := result["id"].(string)
	require.True(t, ok, "conversation id should be a string")
	return convID
}

func TestSendMessage_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestSendMessage_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	aToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-send-a@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, aToken, "DM Send Server", "dmsendsrv", 1, false)

	bToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-send-b@example.com", "password123")
	setup.JoinTestServer(t, app, bToken, serverID, "SendPeer", "dmsendpeer", "")
	bID := setup.GetUserId(t, app, bToken)

	convID := getOrCreateConversation(t, app, serverID, aToken, bID)

	clientMsgID := uuid.New().String()
	body := []byte(fmt.Sprintf(`{"content":"hello there","clientMessageId":%q}`, clientMsgID))
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/conversations/%s/messages", convID), body, aToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "send message request should complete")
	result := setup.RequireJSONResponse(t, resp, 200)
	require.Equal(t, "hello there", result["content"])
	require.Equal(t, convID, result["conversationId"])
	require.Equal(t, clientMsgID, result["clientMessageId"])
	setup.LogTestPass(t, "TestSendMessage_Success")
}

func TestSendMessage_IdempotentByClientMessageId(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestSendMessage_IdempotentByClientMessageId")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	aToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-send-idem-a@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, aToken, "DM Send Idem Server", "dmsendidem", 1, false)

	bToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-send-idem-b@example.com", "password123")
	setup.JoinTestServer(t, app, bToken, serverID, "SendIdemPeer", "dmsendidemp", "")
	bID := setup.GetUserId(t, app, bToken)

	convID := getOrCreateConversation(t, app, serverID, aToken, bID)

	clientMsgID := uuid.New().String()
	body := []byte(fmt.Sprintf(`{"content":"first send","clientMessageId":%q}`, clientMsgID))
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/conversations/%s/messages", convID), body, aToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "first send should succeed")
	first := setup.RequireJSONResponse(t, resp, 200)
	firstID := first["id"].(string)

	body = []byte(fmt.Sprintf(`{"content":"first send","clientMessageId":%q}`, clientMsgID))
	req = setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/conversations/%s/messages", convID), body, aToken)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "duplicate send should complete")
	second := setup.RequireJSONResponse(t, resp, 200)
	require.Equal(t, firstID, second["id"], "resending with the same clientMessageId must be idempotent")
	setup.LogTestPass(t, "TestSendMessage_IdempotentByClientMessageId")
}

func TestSendMessage_NotParticipant(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestSendMessage_NotParticipant")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	aToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-send-np-a@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, aToken, "DM Send NP Server", "dmsendnp", 1, false)

	bToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-send-np-b@example.com", "password123")
	setup.JoinTestServer(t, app, bToken, serverID, "SendNPPeer", "dmsendnpp", "")
	bID := setup.GetUserId(t, app, bToken)

	cToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-send-np-c@example.com", "password123")
	setup.JoinTestServer(t, app, cToken, serverID, "SendNPOutsider", "dmsendnpo", "")

	convID := getOrCreateConversation(t, app, serverID, aToken, bID)

	body := []byte(fmt.Sprintf(`{"content":"sneaky","clientMessageId":%q}`, uuid.New().String()))
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/conversations/%s/messages", convID), body, cToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request should complete")
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "non-participant must be forbidden")
	setup.LogTestPass(t, "TestSendMessage_NotParticipant")
}

func TestSendMessage_ConversationNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestSendMessage_ConversationNotFound")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-send-nf@example.com", "password123")

	body := []byte(fmt.Sprintf(`{"content":"hello","clientMessageId":%q}`, uuid.New().String()))
	req := setup.CreateAuthRequest(http.MethodPost, "/api/conversations/00000000-0000-0000-0000-000000000000/messages", body, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request should complete")
	require.Equal(t, http.StatusNotFound, resp.StatusCode, "non-existent conversation must 404")
	setup.LogTestPass(t, "TestSendMessage_ConversationNotFound")
}

func TestSendMessage_InvalidConversationId(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestSendMessage_InvalidConversationId")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-send-invalid@example.com", "password123")

	body := []byte(fmt.Sprintf(`{"content":"hello","clientMessageId":%q}`, uuid.New().String()))
	req := setup.CreateAuthRequest(http.MethodPost, "/api/conversations/not-a-uuid/messages", body, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request should complete")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "invalid conversation id must be rejected")
	setup.LogTestPass(t, "TestSendMessage_InvalidConversationId")
}

func TestListMessages_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestListMessages_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	aToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-listmsg-a@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, aToken, "DM ListMsg Server", "dmlistmsg", 1, false)

	bToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-listmsg-b@example.com", "password123")
	setup.JoinTestServer(t, app, bToken, serverID, "ListMsgPeer", "dmlistmsgp", "")
	bID := setup.GetUserId(t, app, bToken)

	convID := getOrCreateConversation(t, app, serverID, aToken, bID)

	body := []byte(fmt.Sprintf(`{"content":"first message","clientMessageId":%q}`, uuid.New().String()))
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/conversations/%s/messages", convID), body, aToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "send message should succeed")
	setup.RequireStatus(t, resp, 200)

	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/conversations/%s/messages", convID), nil, bToken)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "list messages request should complete")
	result := setup.RequireJSONResponse(t, resp, 200)
	data, ok := result["data"].([]interface{})
	require.True(t, ok, "response should contain data array, got: %v", result)
	require.Len(t, data, 1, "peer should see the sent message")
	setup.LogTestPass(t, "TestListMessages_Success")
}

func TestListMessages_NotParticipant(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestListMessages_NotParticipant")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	aToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-listmsg-np-a@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, aToken, "DM ListMsg NP Server", "dmlistmnp", 1, false)

	bToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-listmsg-np-b@example.com", "password123")
	setup.JoinTestServer(t, app, bToken, serverID, "ListMsgNPPeer", "dmlistmsgnpp", "")
	bID := setup.GetUserId(t, app, bToken)

	cToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-listmsg-np-c@example.com", "password123")
	setup.JoinTestServer(t, app, cToken, serverID, "ListMsgNPOutsider", "dmlistmsgnpo", "")

	convID := getOrCreateConversation(t, app, serverID, aToken, bID)

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/conversations/%s/messages", convID), nil, cToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request should complete")
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "non-participant must be forbidden")
	setup.LogTestPass(t, "TestListMessages_NotParticipant")
}

func TestMarkRead_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestMarkRead_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	aToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-markread-a@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, aToken, "DM MarkRead Server", "dmmarkread", 1, false)

	bToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-markread-b@example.com", "password123")
	setup.JoinTestServer(t, app, bToken, serverID, "MarkReadPeer", "dmmarkreadp", "")
	bID := setup.GetUserId(t, app, bToken)

	convID := getOrCreateConversation(t, app, serverID, aToken, bID)

	body := []byte(fmt.Sprintf(`{"content":"read me","clientMessageId":%q}`, uuid.New().String()))
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/conversations/%s/messages", convID), body, aToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "send message should succeed")
	sent := setup.RequireJSONResponse(t, resp, 200)
	msgID := sent["id"].(string)

	readBody := []byte(fmt.Sprintf(`{"lastReadMessageId":%q}`, msgID))
	req = setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/conversations/%s/read", convID), readBody, bToken)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "mark read request should complete")
	setup.RequireStatus(t, resp, 200)
	setup.LogTestPass(t, "TestMarkRead_Success")
}

func TestMarkRead_NotParticipant(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestMarkRead_NotParticipant")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	aToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-markread-np-a@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, aToken, "DM MarkRead NP Server", "dmmarknp", 1, false)

	bToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-markread-np-b@example.com", "password123")
	setup.JoinTestServer(t, app, bToken, serverID, "MarkReadNPPeer", "dmmarkreadnpp", "")
	bID := setup.GetUserId(t, app, bToken)

	cToken := setup.CreateTestUser(t, app, infra.MailhogURL, "dm-markread-np-c@example.com", "password123")
	setup.JoinTestServer(t, app, cToken, serverID, "MarkReadNPOutsider", "dmmarkreadnpo", "")

	convID := getOrCreateConversation(t, app, serverID, aToken, bID)

	readBody := []byte(`{}`)
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/conversations/%s/read", convID), readBody, cToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request should complete")
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "non-participant must be forbidden")
	setup.LogTestPass(t, "TestMarkRead_NotParticipant")
}

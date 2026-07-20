package notification

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestMarkRead_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestMarkRead_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	authorToken := setup.CreateTestUser(t, app, infra.MailhogURL, "markread-author@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, authorToken, "MarkRead Server", "markread1", 1, false)

	likerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "markread-liker@example.com", "password123")
	setup.JoinTestServer(t, app, likerToken, serverID, "Liker", "markreadliker", "")

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Notify me",
	})
	req := setup.CreateAuthMultipartRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, authorToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.RequireJSONResponse(t, resp, 200)
	postID, ok := result["id"].(string)
	require.True(t, ok, "post id should be a string")

	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postID+"/likes", nil, likerToken)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "like post should succeed")
	setup.RequireStatus(t, resp, 200)

	var notifID string
	for i := 0; i < 20; i++ {
		req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/notifications", serverID), nil, authorToken)
		resp, err = setup.AppTest(t, app, req)
		require.NoError(t, err, "feed request should complete")
		setup.RequireStatus(t, resp, 200)
		feed := setup.ParseJSONResponse(t, resp)
		data, _ := feed["data"].([]interface{})
		if len(data) > 0 {
			first, _ := data[0].(map[string]interface{})
			notifID, _ = first["id"].(string)
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	require.NotEmpty(t, notifID, "expected a like notification to be created for the post author")

	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/notifications/unread-count", serverID), nil, authorToken)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "unread-count request should complete")
	unreadResult := setup.RequireJSONResponse(t, resp, 200)
	require.EqualValues(t, 1, unreadResult["count"], "one unread notification should exist before marking read")

	req = setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/notifications/%s/read", serverID, notifID), nil, authorToken)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "mark read request should complete")
	setup.RequireStatus(t, resp, 200)

	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/notifications/unread-count", serverID), nil, authorToken)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "unread-count request should complete")
	unreadResult = setup.RequireJSONResponse(t, resp, 200)
	require.EqualValues(t, 0, unreadResult["count"], "notification should be marked as read")
	setup.LogTestPass(t, "TestMarkRead_Success")
}

func TestMarkRead_NotMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestMarkRead_NotMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "markread-guard-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "MarkRead Guard Server", "markreadg", 1, false)
	strangerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "markread-guard-stranger@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/notifications/%s/read", serverID, "00000000-0000-0000-0000-000000000000"), nil, strangerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "mark read request should complete")
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "non-member must be forbidden")
	setup.LogTestPass(t, "TestMarkRead_NotMember")
}

func TestMarkRead_InvalidNotificationId(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestMarkRead_InvalidNotificationId")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "markread-invalid@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, token, "MarkRead Invalid Server", "markreadi", 1, false)

	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/notifications/not-a-uuid/read", serverID), nil, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "mark read request should complete")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "invalid notification id must be rejected")
	setup.LogTestPass(t, "TestMarkRead_InvalidNotificationId")
}

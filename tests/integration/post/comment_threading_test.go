package post

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestCreateComment_Reply creates a top-level comment and then a reply that
// references it via parentId. Both rows should land in server_post_comments
// and the reply should expose the parent linkage in the response.
func TestCreateComment_Reply(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestCreateComment_Reply")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "reply@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, token, "Reply Server", "replysrv", 1, false)

	// Setup: create one post we can comment on.
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Thread root",
	})
	req := setup.CreateAuthMultipartRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "create post should succeed")
	setup.RequireStatus(t, resp, 200)
	postID := setup.ParseJSONResponse(t, resp)["id"].(string)

	// Top-level comment.
	reqBody := []byte(`{"content":"Top-level comment"}`)
	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postID+"/comments", reqBody, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "create top-level comment should succeed")
	setup.RequireStatus(t, resp, 200)

	parentResult := setup.ParseJSONResponse(t, resp)
	parentID, ok := parentResult["id"].(string)
	require.True(t, ok, "parent comment id should be string")
	require.Nil(t, parentResult["parentId"], "top-level comment parentId must be null")

	// Reply referencing parent.
	reqBody = []byte(fmt.Sprintf(`{"content":"Reply comment","parentId":"%s"}`, parentID))
	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postID+"/comments", reqBody, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "create reply should succeed")
	setup.RequireStatus(t, resp, 200)

	replyResult := setup.ParseJSONResponse(t, resp)
	require.Equal(t, parentID, replyResult["parentId"], "reply parentId should match the top-level comment id")
	require.NotEqual(t, parentID, replyResult["id"], "reply must have its own id")

	// List comments and confirm both entries are present.
	req = setup.CreateAuthRequest(http.MethodGet, "/api/posts/"+postID+"/comments", nil, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "list comments should succeed")
	setup.RequireStatus(t, resp, 200)

	list := setup.ParseJSONResponse(t, resp)
	data, ok := list["data"].([]interface{})
	require.True(t, ok, "data should be an array")
	require.Len(t, data, 2, "should contain both the parent and the reply")
	setup.LogTestPass(t, "TestCreateComment_Reply")
}

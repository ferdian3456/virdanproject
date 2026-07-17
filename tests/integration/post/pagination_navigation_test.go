package post

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestGetServerPosts_NextPage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestGetServerPosts_NextPage")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "pagnav@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, token, "Pagination Nav Server", "pagnav", 1, false)

	for i := 1; i <= 3; i++ {
		imageData := setup.CreateTestWebPImage(t)
		body, contentType := setup.CreateMultipartFormData(t, "image", fmt.Sprintf("nav%d.webp", i), imageData, map[string]string{
			"caption": fmt.Sprintf("Post %d", i),
		})
		req := setup.CreateAuthMultipartRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
		resp, err := setup.AppTest(t, app, req)
		require.NoError(t, err, "create post %d should succeed", i)
		setup.RequireStatus(t, resp, 200)
	}

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/posts?limit=2", serverID), nil, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "first page should succeed")
	setup.RequireStatus(t, resp, 200)

	page1 := setup.ParseJSONResponse(t, resp)
	data1, ok := page1["data"].([]interface{})
	require.True(t, ok, "page 1 data must be an array")
	require.Len(t, data1, 2, "page 1 should contain exactly 2 posts")

	pageMeta, ok := page1["page"].(map[string]interface{})
	require.True(t, ok, "page 1 must include a page object")
	cursor, ok := pageMeta["nextCursor"].(string)
	require.True(t, ok, "page 1 nextCursor must be a string")
	require.NotEmpty(t, cursor, "page 1 nextCursor must not be empty when there is more data")

	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/posts?limit=2&cursor=%s", serverID, cursor), nil, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "second page should succeed")
	setup.RequireStatus(t, resp, 200)

	page2 := setup.ParseJSONResponse(t, resp)
	data2, ok := page2["data"].([]interface{})
	require.True(t, ok, "page 2 data must be an array")
	require.Len(t, data2, 1, "page 2 should contain the remaining 1 post")

	idsPage1 := map[string]bool{}
	for _, item := range data1 {
		m := item.(map[string]interface{})
		idsPage1[m["id"].(string)] = true
	}
	for _, item := range data2 {
		m := item.(map[string]interface{})
		require.False(t, idsPage1[m["id"].(string)], "page 2 must not repeat ids from page 1")
	}
	setup.LogTestPass(t, "TestGetServerPosts_NextPage")
}

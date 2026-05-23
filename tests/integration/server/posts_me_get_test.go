package server

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestPostsMeGet_Success retrieves the caller's posts inside a server. The
// owner creates one post and then asks the API to return only their own posts.
func TestPostsMeGet_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestPostsMeGet_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "postsme@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, token, "Posts Me Server", "postsme", 1, false)

	// Create one post so the owner has something to retrieve.
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Owner post",
	})
	req := setup.CreateAuthMultipartRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := app.Test(req)
	require.NoError(t, err, "create post should succeed")
	setup.RequireStatus(t, resp, 200)

	// Retrieve the caller's posts for this server.
	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/posts/me", serverID), nil, token)
	resp, err = app.Test(req)
	require.NoError(t, err, "get my posts should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "data", "response should contain data array")
	data, ok := result["data"].([]interface{})
	require.True(t, ok, "data should be an array")
	require.GreaterOrEqual(t, len(data), 1, "owner should see at least the post they just created")
	setup.LogTestPass(t, "TestPostsMeGet_Success")
}

// TestPostsMeGet_NotAMember rejects a non-member's request to view their own
// posts inside the server.
func TestPostsMeGet_NotAMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestPostsMeGet_NotAMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "postsme-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Posts Me Owner Server", "postsmeo", 1, false)

	strangerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "postsme-stranger@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/posts/me", serverID), nil, strangerToken)
	resp, err := app.Test(req)
	require.NoError(t, err, "get my posts should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "non-member should be rejected")
	setup.LogTestPass(t, "TestPostsMeGet_NotAMember")
}

// TestPostsMeGet_Unauthorized rejects unauthenticated callers.
func TestPostsMeGet_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestPostsMeGet_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/00000000-0000-0000-0000-000000000000/posts/me", nil, "")
	resp, err := app.Test(req)
	require.NoError(t, err, "get my posts should complete")
	require.NotEqual(t, 200, resp.StatusCode, "unauthenticated request must not succeed")
	setup.LogTestPass(t, "TestPostsMeGet_Unauthorized")
}

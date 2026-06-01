package server

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestPostsByUserGet_Success lets a member view another member's posts in a
// server (their profile grid).
func TestPostsByUserGet_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestPostsByUserGet_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "pbu-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Posts By User Server", "pbuserv", 1, false)

	memberToken := setup.CreateTestUser(t, app, infra.MailhogURL, "pbu-member@example.com", "password123")
	setup.JoinTestServer(t, app, memberToken, serverID, "MemberX", "memberx", "")
	memberID := setup.GetUserId(t, app, memberToken)

	// The member publishes one post.
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Member post",
	})
	req := setup.CreateAuthMultipartRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, memberToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "member create post should succeed")
	setup.RequireStatus(t, resp, 200)

	// The owner views the member's posts.
	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/members/%s/posts", serverID, memberID), nil, ownerToken)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "view member posts should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "data", "response should contain data array")
	data, ok := result["data"].([]interface{})
	require.True(t, ok, "data should be an array")
	require.GreaterOrEqual(t, len(data), 1, "should see the member's post")
	setup.LogTestPass(t, "TestPostsByUserGet_Success")
}

// TestPostsByUserGet_RequesterNotMember rejects a requester that is not a member.
func TestPostsByUserGet_RequesterNotMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestPostsByUserGet_RequesterNotMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "pbu2-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Posts By User Guard", "pbuguard", 1, false)
	ownerID := setup.GetUserId(t, app, ownerToken)

	strangerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "pbu2-stranger@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/members/%s/posts", serverID, ownerID), nil, strangerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request should complete")
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "non-member requester must be forbidden")
	setup.LogTestPass(t, "TestPostsByUserGet_RequesterNotMember")
}

// TestPostsByUserGet_Unauthorized rejects unauthenticated callers.
func TestPostsByUserGet_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestPostsByUserGet_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/00000000-0000-0000-0000-000000000000/members/00000000-0000-0000-0000-000000000000/posts", nil, "")
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request should complete")
	require.NotEqual(t, 200, resp.StatusCode, "unauthenticated request must not succeed")
	setup.LogTestPass(t, "TestPostsByUserGet_Unauthorized")
}

// TestPostsByUserGet_UserLikedReflectsRequester verifies userLiked reflects the
// viewer (requester), not the post author — the reason this endpoint uses a
// dedicated query instead of reusing GetServerPostForMe.
func TestPostsByUserGet_UserLikedReflectsRequester(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestPostsByUserGet_UserLikedReflectsRequester")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "pbulike-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Posts Like Server", "pbulike", 1, false)
	ownerID := setup.GetUserId(t, app, ownerToken)

	// Owner publishes a post.
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Likeable",
	})
	req := setup.CreateAuthMultipartRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, ownerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "create post should succeed")
	setup.RequireStatus(t, resp, 200)

	// Viewer joins and views the owner's posts to grab the post id (userLiked false).
	viewerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "pbulike-viewer@example.com", "password123")
	setup.JoinTestServer(t, app, viewerToken, serverID, "Viewer", "viewerx", "")

	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/members/%s/posts", serverID, ownerID), nil, viewerToken)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)
	result := setup.ParseJSONResponse(t, resp)
	data, ok := result["data"].([]interface{})
	require.True(t, ok, "data should be an array")
	require.GreaterOrEqual(t, len(data), 1, "viewer should see the owner's post")
	first := data[0].(map[string]interface{})
	postID := first["id"].(string)
	require.Equal(t, false, first["userLiked"], "viewer has not liked yet")

	// Viewer likes the post.
	likeReq := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/posts/%s/likes", postID), nil, viewerToken)
	resp, err = setup.AppTest(t, app, likeReq)
	require.NoError(t, err, "like should succeed")
	setup.RequireStatus(t, resp, 200)

	// Re-fetch: userLiked now reflects the VIEWER (requester), not the author.
	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/members/%s/posts", serverID, ownerID), nil, viewerToken)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)
	result = setup.ParseJSONResponse(t, resp)
	data = result["data"].([]interface{})
	first = data[0].(map[string]interface{})
	require.Equal(t, true, first["userLiked"], "userLiked should reflect the requester who liked it")
	setup.LogTestPass(t, "TestPostsByUserGet_UserLikedReflectsRequester")
}

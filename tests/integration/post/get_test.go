package post

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestGetServerPosts_Success tests successful get server posts
func TestGetServerPosts_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestGetServerPosts_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	// Setup: Create user, server, and posts
	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "getposts@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Get Posts Server", "getposts", 1, false)

	// Create a post
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	_, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")

	// Test: Get server posts
	setup.LogTestStep(t, "Testing Get Server Posts")
	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/posts", serverID), nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get server posts request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "data", "response should contain data")

	t.Logf("Server posts retrieved successfully")
	setup.LogTestPass(t, "TestGetServerPosts_Success")
}

// TestGetServerPosts_Unauthorized tests get server posts without authentication
func TestGetServerPosts_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestGetServerPosts_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "unauthgetposts@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Unauth Get Posts Server", "unauthget", 1, false)

	// Test: Get server posts without token
	setup.LogTestStep(t, "Testing Get Server Posts Without Auth")
	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/posts", serverID), nil, "")
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get server posts request should complete")

	// Should return unauthorized (404 from auth middleware)
	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated get server posts request")
	setup.LogTestPass(t, "TestGetServerPosts_Unauthorized")
}

// TestGetServerPosts_NotAMember tests get server posts when user is not a member
func TestGetServerPosts_NotAMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestGetServerPosts_NotAMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	// Setup: Create user and server (user is NOT a member)
	token1 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "ownergetposts@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token1, "Owner Get Posts Server", "ownerget", 1, false)

	// Create another user (not a member of the server)
	token2 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "nonmembergetposts@example.com", "password123")

	// Test: Try to get server posts as non-member
	setup.LogTestStep(t, "Testing Get Server Posts as Non-Member")
	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/posts", serverID), nil, token2)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get server posts request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "not a member", "error message should mention not a member")

	t.Logf("Correctly rejected get server posts request from non-member: %s", errMsg)
	setup.LogTestPass(t, "TestGetServerPosts_NotAMember")
}

// TestGetServerPosts_WithPagination tests get server posts with pagination
func TestGetServerPosts_WithPagination(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestGetServerPosts_WithPagination")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "pagposts@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Pag Posts Server", "pagposts", 1, false)

	// Create multiple posts
	for i := 1; i <= 3; i++ {
		imageData := setup.CreateTestWebPImage(t)
		body, contentType := setup.CreateMultipartFormData(t, "image", fmt.Sprintf("test%d.jpg", i), imageData, map[string]string{
			"caption": fmt.Sprintf("Test caption %d", i),
		})
		req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
		resp, err := setup.TestRequestWithLogging(t, app, req)
		require.NoError(t, err, fmt.Sprintf("create post %d should succeed", i))
		setup.RequireStatus(t, resp, 200)
	}

	// Test: Get server posts with limit
	setup.LogTestStep(t, "Testing Get Server Posts With Pagination")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/"+serverID+"/posts?limit=2", nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get server posts request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "data", "response should contain data")

	t.Logf("Server posts retrieved successfully with pagination")
	setup.LogTestPass(t, "TestGetServerPosts_WithPagination")
}

// TestGetPost_Success tests successful get single post
func TestGetPost_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestGetPost_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	// Setup: Create user, server, and post
	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "getpost@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Get Post Server", "getpost", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["id"].(string)

	// Test: Get single post
	setup.LogTestStep(t, "Testing Get Single Post")
	req = setup.CreateAuthRequest(http.MethodGet, "/api/posts/"+postID, nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get post request should succeed")
	setup.RequireStatus(t, resp, 200)

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "id", "response should contain post id")

	t.Logf("Single post retrieved successfully")
	setup.LogTestPass(t, "TestGetPost_Success")
}

// TestGetPost_Unauthorized tests get single post without authentication
func TestGetPost_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestGetPost_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "unauthgetpost@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Unauth Get Post Server", "unauthgetpost", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["id"].(string)

	// Test: Get single post without token
	setup.LogTestStep(t, "Testing Get Single Post Without Auth")
	req = setup.CreateAuthRequest(http.MethodGet, "/api/posts/"+postID, nil, "")
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get post request should complete")

	// Should return unauthorized (404 from auth middleware)
	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated get post request")
	setup.LogTestPass(t, "TestGetPost_Unauthorized")
}

// TestGetPost_InvalidPostId tests get single post with invalid post ID
func TestGetPost_InvalidPostId(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestGetPost_InvalidPostId")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "invalidgetpost@example.com", "password123")

	// Test: Get single post with invalid post ID
	setup.LogTestStep(t, "Testing Get Single Post with Invalid Post ID")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/posts/00000000-0000-0000-0000-000000000000", nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get post request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	t.Logf("Error message for invalid post ID: %s", errMsg)
	setup.LogTestPass(t, "TestGetPost_InvalidPostId")
}

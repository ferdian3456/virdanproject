package post

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestUpdatePost_Success tests successful post caption update
func TestUpdatePost_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestUpdatePost_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	// Setup: Create user, server, and post
	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "updatepost@example.com", "updatepostuser", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Update Post Server", "updatepost", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Original caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["postId"].(string)

	// Test: Update post caption
	setup.LogTestStep(t, "Testing Update Post Caption")
	reqBody := []byte(`{"caption":"Updated caption"}`)
	req = setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/posts/%s", serverID, postID), reqBody, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "update post request should succeed")
	setup.RequireStatus(t, resp, 200)

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "caption", "response should contain updated caption")

	updatedCaption := result["caption"].(string)
	require.Equal(t, "Updated caption", updatedCaption, "caption should be updated")

	t.Logf("Post caption updated successfully")
	setup.LogTestPass(t, "TestUpdatePost_Success")
}

// TestUpdatePost_Unauthorized tests post update without authentication
func TestUpdatePost_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestUpdatePost_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "unauthupdpost@example.com", "unauthupdpostuser", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Unauth Update Post Server", "unauthup", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Original caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["postId"].(string)

	// Test: Update post without token
	setup.LogTestStep(t, "Testing Update Post Without Auth")
	reqBody := []byte(`{"caption":"Updated caption"}`)
	req = setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/posts/%s", serverID, postID), reqBody, "")
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "update post request should complete")

	// Should return unauthorized
	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated update post request")
	setup.LogTestPass(t, "TestUpdatePost_Unauthorized")
}

// TestUpdatePost_NotOwner tests post update when user is not post owner
func TestUpdatePost_NotOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestUpdatePost_NotOwner")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	// Setup: Create user, server, and post
	token1 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "ownerupdpost@example.com", "ownerupdpostuser", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token1, "Owner Update Post Server", "ownerup", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Original caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token1)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["postId"].(string)

	// Create another user (not owner of the post)
	token2 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "notownerupdpost@example.com", "notownerupdpostuser", "password123")

	// Test: Try to update post as non-owner
	setup.LogTestStep(t, "Testing Update Post as Non-Owner")
	reqBody := []byte(`{"caption":"Hacked caption"}`)
	req = setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/posts/%s", serverID, postID), reqBody, token2)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "update post request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "not a member", "error message should mention not a member")

	t.Logf("Correctly rejected post update by non-owner: %s", errMsg)
	setup.LogTestPass(t, "TestUpdatePost_NotOwner")
}

// TestDeletePost_Success tests successful post deletion
func TestDeletePost_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestDeletePost_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	// Setup: Create user, server, and post
	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "deletepost@example.com", "deletepostuser", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Delete Post Server", "deletepost", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["postId"].(string)

	// Test: Delete post
	setup.LogTestStep(t, "Testing Delete Post")
	req = setup.CreateAuthRequest(http.MethodDelete, fmt.Sprintf("/api/servers/%s/posts/%s", serverID, postID), nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "delete post request should succeed")
	setup.RequireStatus(t, resp, 200)

	t.Logf("Post deleted successfully")
	setup.LogTestPass(t, "TestDeletePost_Success")
}

// TestDeletePost_Unauthorized tests post deletion without authentication
func TestDeletePost_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestDeletePost_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "unauthdelpost@example.com", "unauthdelpostuser", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Unauth Delete Post Server", "unauthdel", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["postId"].(string)

	// Test: Delete post without token
	setup.LogTestStep(t, "Testing Delete Post Without Auth")
	req = setup.CreateAuthRequest(http.MethodDelete, fmt.Sprintf("/api/servers/%s/posts/%s", serverID, postID), nil, "")
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "delete post request should complete")

	// Should return unauthorized
	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated delete post request")
	setup.LogTestPass(t, "TestDeletePost_Unauthorized")
}

// TestDeletePost_NotOwner tests post deletion when user is not post owner
func TestDeletePost_NotOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestDeletePost_NotOwner")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	// Setup: Create user, server, and post
	token1 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "ownerdelpost@example.com", "ownerdelpostuser", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token1, "Owner Delete Post Server", "ownerdel", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token1)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["postId"].(string)

	// Create another user (not owner of the post)
	token2 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "notownerdelpost@example.com", "notownerdelpostuser", "password123")

	// Test: Try to delete post as non-owner
	setup.LogTestStep(t, "Testing Delete Post as Non-Owner")
	req = setup.CreateAuthRequest(http.MethodDelete, fmt.Sprintf("/api/servers/%s/posts/%s", serverID, postID), nil, token2)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "delete post request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "not a member", "error message should mention not a member")

	t.Logf("Correctly rejected post deletion by non-owner: %s", errMsg)
	setup.LogTestPass(t, "TestDeletePost_NotOwner")
}

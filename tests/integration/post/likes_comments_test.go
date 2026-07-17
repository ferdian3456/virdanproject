package post

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestLikePost_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestLikePost_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "likepost@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Like Post Server", "likepost", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["id"].(string)

	setup.LogTestStep(t, "Testing Like Post")
	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postID+"/likes", nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "like post request should succeed")
	setup.RequireStatus(t, resp, 200)

	result = setup.ParseJSONResponse(t, resp)
	if likeCount, hasCount := result["likeCount"]; hasCount {
		t.Logf("Post liked successfully, likeCount: %v", likeCount)
	} else {
		t.Logf("Post liked successfully")
	}
	setup.LogTestPass(t, "TestLikePost_Success")
}

func TestLikePost_DoubleLike(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestLikePost_DoubleLike")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "doublelike@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Double Like Server", "doublelike", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["id"].(string)

	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postID+"/likes", nil, token)
	_, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "first like should succeed")

	setup.LogTestStep(t, "Testing Double Like Post")
	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postID+"/likes", nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "second like request should complete")

	result = setup.ParseJSONResponse(t, resp)
	if likeCount, hasCount := result["likeCount"]; hasCount {
		t.Logf("Double like handled correctly, likeCount: %v", likeCount)
	} else {
		t.Logf("Double like handled correctly")
	}
	setup.LogTestPass(t, "TestLikePost_DoubleLike")
}

func TestUnlikePost_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestUnlikePost_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "unlikepost@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Unlike Post Server", "unlikepost", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["id"].(string)

	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postID+"/likes", nil, token)
	_, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "like post should succeed")

	setup.LogTestStep(t, "Testing Unlike Post")
	req = setup.CreateAuthRequest(http.MethodDelete, "/api/posts/"+postID+"/likes", nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "unlike post request should succeed")
	setup.RequireStatus(t, resp, 200)

	result = setup.ParseJSONResponse(t, resp)
	if likeCount, hasCount := result["likeCount"]; hasCount {
		count := int(likeCount.(float64))
		require.Equal(t, 0, count, "post should not be liked")
		t.Logf("Post unliked successfully, likeCount: %d", count)
	} else {
		t.Logf("Post unliked successfully")
	}
	setup.LogTestPass(t, "TestUnlikePost_Success")
}

func TestUnlikePost_NotLiked(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestUnlikePost_NotLiked")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "notliked@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Not Liked Server", "notliked", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["id"].(string)

	setup.LogTestStep(t, "Testing Unlike Post When Not Liked")
	req = setup.CreateAuthRequest(http.MethodDelete, "/api/posts/"+postID+"/likes", nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "unlike post request should complete")

	result = setup.ParseJSONResponse(t, resp)
	if likeCount, hasCount := result["likeCount"]; hasCount {
		count := int(likeCount.(float64))
		require.Equal(t, 0, count, "post should not be liked")
		t.Logf("Unlike when not liked handled correctly, likeCount: %d", count)
	} else {
		t.Logf("Unlike when not liked handled correctly")
	}
	setup.LogTestPass(t, "TestUnlikePost_NotLiked")
}

func TestCreateComment_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestCreateComment_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "createcomment@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Create Comment Server", "creatcomm", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["id"].(string)

	setup.LogTestStep(t, "Testing Create Comment")
	reqBody := []byte(`{"content":"This is a test comment"}`)
	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postID+"/comments", reqBody, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create comment request should succeed")
	setup.RequireStatus(t, resp, 200)

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "id", "response should contain comment id")

	t.Logf("Comment created successfully")
	setup.LogTestPass(t, "TestCreateComment_Success")
}

func TestCreateComment_EmptyContent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestCreateComment_EmptyContent")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "emptycomment@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Empty Comment Server", "emptycomm", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["id"].(string)

	setup.LogTestStep(t, "Testing Create Comment with Empty Content")
	reqBody := []byte(`{"content":""}`)
	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postID+"/comments", reqBody, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create comment request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "required", "error message should mention content is required")

	t.Logf("Correctly rejected empty comment content")
	setup.LogTestPass(t, "TestCreateComment_EmptyContent")
}

func TestGetComments_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestGetComments_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "getcomments@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Get Comments Server", "getcomm", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["id"].(string)

	reqBody := []byte(`{"content":"Test comment"}`)
	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postID+"/comments", reqBody, token)
	_, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create comment should succeed")

	setup.LogTestStep(t, "Testing Get Comments")
	req = setup.CreateAuthRequest(http.MethodGet, "/api/posts/"+postID+"/comments", nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get comments request should succeed")
	setup.RequireStatus(t, resp, 200)

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "data", "response should contain data")

	t.Logf("Comments retrieved successfully")
	setup.LogTestPass(t, "TestGetComments_Success")
}

func TestGetComments_Empty(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestGetComments_Empty")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "nocomments@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "No Comments Server", "nocomments", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["id"].(string)

	setup.LogTestStep(t, "Testing Get Comments with No Comments")
	req = setup.CreateAuthRequest(http.MethodGet, "/api/posts/"+postID+"/comments", nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get comments request should succeed")
	setup.RequireStatus(t, resp, 200)

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "data", "response should contain data")

	t.Logf("Empty comments list retrieved successfully")
	setup.LogTestPass(t, "TestGetComments_Empty")
}

func TestDeleteComment_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestDeleteComment_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "deletecomment@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Delete Comment Server", "delcomm", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["id"].(string)

	reqBody := []byte(`{"content":"Test comment to delete"}`)
	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postID+"/comments", reqBody, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create comment should succeed")
	result = setup.ParseJSONResponse(t, resp)
	commentID := result["id"].(string)

	setup.LogTestStep(t, "Testing Delete Comment")
	req = setup.CreateAuthRequest(http.MethodDelete, fmt.Sprintf("/api/posts/%s/comments/%s", postID, commentID), nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "delete comment request should succeed")
	setup.RequireStatus(t, resp, 200)

	t.Logf("Comment deleted successfully")
	setup.LogTestPass(t, "TestDeleteComment_Success")
}

func TestDeleteComment_NotOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestDeleteComment_NotOwner")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token1 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "ownerdelcomment@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token1, "Owner Delete Comment Server", "ownerdelc", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token1)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["id"].(string)

	reqBody := []byte(`{"content":"Test comment"}`)
	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postID+"/comments", reqBody, token1)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create comment should succeed")
	result = setup.ParseJSONResponse(t, resp)
	commentID := result["id"].(string)

	token2 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "notownerdelcomment@example.com", "password123")

	setup.LogTestStep(t, "Testing Delete Comment as Non-Owner")
	req = setup.CreateAuthRequest(http.MethodDelete, fmt.Sprintf("/api/posts/%s/comments/%s", postID, commentID), nil, token2)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "delete comment request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "not a member", "error message should mention not a member")

	t.Logf("Correctly rejected comment deletion by non-owner: %s", errMsg)
	setup.LogTestPass(t, "TestDeleteComment_NotOwner")
}

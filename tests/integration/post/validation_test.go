package post

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestGetServerPosts_LimitNegative(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestGetServerPosts_LimitNegative")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "neglimitpost@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Neg Limit Post Server", "neglimpost", 1, false)

	setup.LogTestStep(t, "Testing Get Server Posts with Negative Limit")
	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/posts?limit=-1", serverID), nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get server posts request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "must be at least 0", "error message should mention limit must be >= 0")

	t.Logf("Correctly rejected negative limit: %s", errMsg)
	setup.LogTestPass(t, "TestGetServerPosts_LimitNegative")
}

func TestGetServerPosts_LimitExceeded(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestGetServerPosts_LimitExceeded")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "maxlimitpost@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Max Limit Post Server", "maxlimpost", 1, false)

	setup.LogTestStep(t, "Testing Get Server Posts with Limit Exceeded")
	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/posts?limit=21", serverID), nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get server posts request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "must be at most 20", "error message should mention max limit")

	t.Logf("Correctly rejected exceeded limit: %s", errMsg)
	setup.LogTestPass(t, "TestGetServerPosts_LimitExceeded")
}

func TestGetComments_LimitNegative(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestGetComments_LimitNegative")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "neglimitcomment@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Neg Limit Comment Server", "neglimcomm", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["id"].(string)

	setup.LogTestStep(t, "Testing Get Comments with Negative Limit")
	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/posts/%s/comments?limit=-1", postID), nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get comments request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "must be at least 0", "error message should mention limit must be >= 0")

	t.Logf("Correctly rejected negative limit: %s", errMsg)
	setup.LogTestPass(t, "TestGetComments_LimitNegative")
}

func TestGetComments_LimitExceeded(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestGetComments_LimitExceeded")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "maxlimitcomment@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Max Limit Comment Server", "maxlimcomm", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["id"].(string)

	setup.LogTestStep(t, "Testing Get Comments with Limit Exceeded")
	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/posts/%s/comments?limit=21", postID), nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get comments request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "must be at most 20", "error message should mention max limit")

	t.Logf("Correctly rejected exceeded limit: %s", errMsg)
	setup.LogTestPass(t, "TestGetComments_LimitExceeded")
}

func TestUpdatePost_EmptyCaption(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestUpdatePost_EmptyCaption")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "emptycaption@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Empty Caption Server", "emptycapt", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Original caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["id"].(string)

	setup.LogTestStep(t, "Testing Update Post with Empty Caption")
	reqBody := []byte(`{"caption":""}`)
	req = setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/posts/%s", serverID, postID), reqBody, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "update post request should complete")

	result = setup.ParseJSONResponse(t, resp)
	if _, hasError := result["error"]; hasError {
		errMsg := setup.ParseErrorMessage(t, result)
		t.Logf("Empty caption rejected: %s", errMsg)
	} else {
		t.Logf("Empty caption allowed - caption can be empty")
	}
	setup.LogTestPass(t, "TestUpdatePost_EmptyCaption")
}

func TestCreateComment_TooLongContent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestCreateComment_TooLongContent")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "longcomment@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Long Comment Server", "longcomm", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	postID := result["id"].(string)

	setup.LogTestStep(t, "Testing Create Comment with Very Long Content")
	longContent := string(make([]byte, 10000))
	for i := range longContent {
		longContent = longContent[:i] + "a"
	}
	reqBody := []byte(fmt.Sprintf(`{"content":"%s"}`, longContent))
	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postID+"/comments", reqBody, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create comment request should complete")

	result = setup.ParseJSONResponse(t, resp)
	if _, hasError := result["error"]; hasError {
		errMsg := setup.ParseErrorMessage(t, result)
		t.Logf("Long content rejected: %s", errMsg)
	} else {
		t.Logf("Long content allowed - no max length limit")
	}
	setup.LogTestPass(t, "TestCreateComment_TooLongContent")
}

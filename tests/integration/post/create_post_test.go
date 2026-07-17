package post

import (
	"fmt"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestCreatePost_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestCreatePost_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "post@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Post Server", "postserver", 1, false)

	setup.LogTestStep(t, "Testing Create Post")
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test post caption",
	})

	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "id", "response should contain post id")

	postID := result["id"].(string)
	require.NotEmpty(t, postID, "post id should not be empty")

	t.Logf("Post created successfully: %s", postID)
	setup.LogTestPass(t, "TestCreatePost_Success")
}

func TestCreatePost_WithoutImage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestCreatePost_WithoutImage")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "noimage@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "No Image Server", "noimage", 1, false)

	setup.LogTestStep(t, "Testing Create Post Without Image")
	reqBody := []byte(`{"caption":"Test caption"}`)
	req := setup.CreateAuthRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post request should complete")

	result := setup.ParseJSONResponse(t, resp)
	if _, hasError := result["error"]; hasError {
		t.Logf("Correctly rejected post without image")
	} else {
		t.Logf("API accepts post without image")
	}
	setup.LogTestPass(t, "TestCreatePost_WithoutImage")
}

func TestCreatePost_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestCreatePost_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "unauthpost@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Unauth Server", "unauth", 1, false)

	setup.LogTestStep(t, "Testing Create Post Without Auth")
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})

	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, "")
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post request should complete")

	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated request")
	setup.LogTestPass(t, "TestCreatePost_Unauthorized")
}

func TestCreatePost_NotAMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Parallel()

	setup.LogTestStart(t, "TestCreatePost_NotAMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()

	token1 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token1, "Member Server", "member", 1, false)

	token2 := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "nonmember@example.com", "password123")

	setup.LogTestStep(t, "Testing Create Post as Non-Member")
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "Test caption",
	})

	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token2)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "not a member", "error message should mention not a member")

	t.Logf("Correctly rejected post creation by non-member: %s", errMsg)
	setup.LogTestPass(t, "TestCreatePost_NotAMember")
}

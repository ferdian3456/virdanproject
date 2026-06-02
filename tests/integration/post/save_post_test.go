package post

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestSavePost_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestSavePost_Success")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "savesuccess@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Save Server", "savesucc", 1, false)
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{"caption": "c"})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	postID := setup.ParseJSONResponse(t, resp)["id"].(string)

	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postID+"/saves", nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "save post request should succeed")
	setup.RequireStatus(t, resp, 200)
	result := setup.ParseJSONResponse(t, resp)
	require.Equal(t, true, result["userSaved"], "userSaved should be true")
	require.Equal(t, postID, result["postId"])
	setup.LogTestPass(t, "TestSavePost_Success")
}

func TestSavePost_AlreadySaved(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestSavePost_AlreadySaved")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "savedup@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Dup Save Server", "savedup", 1, false)
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{"caption": "c"})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	postID := setup.ParseJSONResponse(t, resp)["id"].(string)

	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postID+"/saves", nil, token)
	_, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "first save should succeed")

	setup.LogTestStep(t, "Testing duplicate save -> 409")
	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postID+"/saves", nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 409)
	setup.LogTestPass(t, "TestSavePost_AlreadySaved")
}

func TestSavePost_NotAMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestSavePost_NotAMember")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()

	ownerToken := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "saveowner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, ownerToken, "Owner Server", "saveownr", 1, false)
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{"caption": "c"})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, ownerToken)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	postID := setup.ParseJSONResponse(t, resp)["id"].(string)

	// Outsider never joined the server.
	outsiderToken := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "saveoutsider@example.com", "password123")
	setup.LogTestStep(t, "Testing save by non-member -> 403")
	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postID+"/saves", nil, outsiderToken)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 403)
	setup.LogTestPass(t, "TestSavePost_NotAMember")
}

func TestSavePost_InvalidPostId(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestSavePost_InvalidPostId")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "saveinvalid@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodPost, "/api/posts/not-a-uuid/saves", nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 400)
	setup.LogTestPass(t, "TestSavePost_InvalidPostId")
}

func TestSavePost_PostNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestSavePost_PostNotFound")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "savenotfound@example.com", "password123")

	missingPostID := "11111111-1111-1111-1111-111111111111"
	req := setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+missingPostID+"/saves", nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 404)
	setup.LogTestPass(t, "TestSavePost_PostNotFound")
}

func TestUnsavePost_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestUnsavePost_Success")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "unsavesuccess@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Unsave Server", "unsavesc", 1, false)
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{"caption": "c"})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	postID := setup.ParseJSONResponse(t, resp)["id"].(string)

	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postID+"/saves", nil, token)
	_, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	setup.LogTestStep(t, "Testing unsave -> 200 userSaved false")
	req = setup.CreateAuthRequest(http.MethodDelete, "/api/posts/"+postID+"/saves", nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)
	result := setup.ParseJSONResponse(t, resp)
	require.Equal(t, false, result["userSaved"], "userSaved should be false")
	setup.LogTestPass(t, "TestUnsavePost_Success")
}

func TestUnsavePost_NotSaved(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestUnsavePost_NotSaved")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "unsavenotsaved@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Unsave NS Server", "unsavens", 1, false)
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{"caption": "c"})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	postID := setup.ParseJSONResponse(t, resp)["id"].(string)

	setup.LogTestStep(t, "Testing unsave when never saved -> 404")
	req = setup.CreateAuthRequest(http.MethodDelete, "/api/posts/"+postID+"/saves", nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 404)
	setup.LogTestPass(t, "TestUnsavePost_NotSaved")
}

func TestGetSavedPosts_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestGetSavedPosts_Success")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "savedfeed@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Saved Feed Server", "savedfd", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{"caption": "saved one"})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	savedPostID := setup.ParseJSONResponse(t, resp)["id"].(string)

	// A second post that stays unsaved.
	body2, ct2 := setup.CreateMultipartFormData(t, "image", "test.webp", setup.CreateTestWebPImage(t), map[string]string{"caption": "not saved"})
	req = setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body2, ct2, token)
	_, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+savedPostID+"/saves", nil, token)
	_, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	setup.LogTestStep(t, "Testing saved feed returns only the saved post")
	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/posts/saved", serverID), nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)
	result := setup.ParseJSONResponse(t, resp)
	data := result["data"].([]interface{})
	require.Len(t, data, 1, "saved feed should contain exactly one post")
	item := data[0].(map[string]interface{})
	require.Equal(t, savedPostID, item["id"])
	require.Equal(t, true, item["userSaved"])
	require.NotNil(t, item["savedAt"], "savedAt should be present in saved feed")
	setup.LogTestPass(t, "TestGetSavedPosts_Success")
}

func TestGetSavedPosts_OnlyThisServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestGetSavedPosts_OnlyThisServer")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "savedscope@example.com", "password123")

	serverA := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Scope A", "scopea", 1, false)
	serverB := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Scope B", "scopeb", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{"caption": "post in A"})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverA), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	postInA := setup.ParseJSONResponse(t, resp)["id"].(string)

	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postInA+"/saves", nil, token)
	_, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	setup.LogTestStep(t, "Saved feed of server B must be empty")
	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/posts/saved", serverB), nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)
	dataB := setup.ParseJSONResponse(t, resp)["data"].([]interface{})
	require.Len(t, dataB, 0, "server B saved feed must not contain a save made in server A")

	setup.LogTestStep(t, "Saved feed of server A must contain the post")
	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/posts/saved", serverA), nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)
	dataA := setup.ParseJSONResponse(t, resp)["data"].([]interface{})
	require.Len(t, dataA, 1)
	setup.LogTestPass(t, "TestGetSavedPosts_OnlyThisServer")
}

func TestGetSavedPosts_NotAMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestGetSavedPosts_NotAMember")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "savedfeedowner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, ownerToken, "Saved Feed Guard", "savedgd", 1, false)

	outsiderToken := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "savedfeedoutsider@example.com", "password123")
	setup.LogTestStep(t, "Non-member reading saved feed -> 403")
	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/posts/saved", serverID), nil, outsiderToken)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 403)
	setup.LogTestPass(t, "TestGetSavedPosts_NotAMember")
}

func TestFeed_UserSavedFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestFeed_UserSavedFlag")

	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "savedflag@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Flag Server", "flagsrv", 1, false)
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{"caption": "flag post"})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	postID := setup.ParseJSONResponse(t, resp)["id"].(string)

	// Before save: userSaved false in single-post fetch.
	req = setup.CreateAuthRequest(http.MethodGet, "/api/posts/"+postID, nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)
	require.Equal(t, false, setup.ParseJSONResponse(t, resp)["userSaved"], "userSaved should be false before saving")

	req = setup.CreateAuthRequest(http.MethodPost, "/api/posts/"+postID+"/saves", nil, token)
	_, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	// After save: userSaved true.
	req = setup.CreateAuthRequest(http.MethodGet, "/api/posts/"+postID, nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)
	require.Equal(t, true, setup.ParseJSONResponse(t, resp)["userSaved"], "userSaved should be true after saving")
	setup.LogTestPass(t, "TestFeed_UserSavedFlag")
}

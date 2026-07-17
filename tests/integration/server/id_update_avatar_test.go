package server

import (
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestUpdateAvatar_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUpdateAvatar_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "updateavatar@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Update Avatar Server", "upavatar", 1, false)

	setup.LogTestStep(t, "Testing Update Server Avatar")
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "avatar", "avatar.webp", imageData, nil)

	req := setup.CreateAuthMultipartRequest(http.MethodPut, "/api/servers/"+serverID+"/avatar", body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)

	t.Logf("Server avatar updated successfully")
	setup.LogTestPass(t, "TestUpdateAvatar_Success")
}

func TestUpdateAvatar_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUpdateAvatar_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "unauthupdavatar@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Unauth Update Avatar Server", "unauthupd", 1, false)

	setup.LogTestStep(t, "Testing Update Server Avatar Without Auth")
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "avatar", "avatar.jpg", imageData, nil)

	req := setup.CreateAuthMultipartRequest(http.MethodPut, "/api/servers/"+serverID+"/avatar", body, contentType, "")
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated update server avatar request")
	setup.LogTestPass(t, "TestUpdateAvatar_Unauthorized")
}

func TestUpdateAvatar_NotOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUpdateAvatar_NotOwner")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token1 := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "ownerupdavatar@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token1, "Owner Update Avatar Server", "ownerupd", 1, false)

	token2 := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "notownerupdavatar@example.com", "password123")

	setup.LogTestStep(t, "Testing Update Server Avatar as Non-Owner")
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "avatar", "avatar.jpg", imageData, nil)

	req := setup.CreateAuthMultipartRequest(http.MethodPut, "/api/servers/"+serverID+"/avatar", body, contentType, token2)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "not the owner", "error message should mention not the owner")

	t.Logf("Correctly rejected server avatar update by non-owner: %s", errMsg)
	setup.LogTestPass(t, "TestUpdateAvatar_NotOwner")
}

func TestUpdateBanner_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUpdateBanner_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "updatebanner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Update Banner Server", "updbanner", 1, false)

	setup.LogTestStep(t, "Testing Update Server Banner")
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "banner", "banner.webp", imageData, nil)

	req := setup.CreateAuthMultipartRequest(http.MethodPut, "/api/servers/"+serverID+"/banner", body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)

	t.Logf("Server banner updated successfully")
	setup.LogTestPass(t, "TestUpdateBanner_Success")
}

func TestUpdateBanner_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUpdateBanner_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "unauthupdbanner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Unauth Update Banner Server", "unbannr", 1, false)

	setup.LogTestStep(t, "Testing Update Server Banner Without Auth")
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "banner", "banner.jpg", imageData, nil)

	req := setup.CreateAuthMultipartRequest(http.MethodPut, "/api/servers/"+serverID+"/banner", body, contentType, "")
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated update server banner request")
	setup.LogTestPass(t, "TestUpdateBanner_Unauthorized")
}

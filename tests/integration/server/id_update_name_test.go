package server

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestUpdateName_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUpdateName_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "updatename@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Original Name", "updatename", 1, false)

	setup.LogTestStep(t, "Testing Update Server Name")
	reqBody := []byte(`{"name":"Updated Server Name"}`)
	req := setup.CreateAuthRequest(http.MethodPut, "/api/servers/"+serverID+"/name", reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "id", "response should contain server id")
	require.Contains(t, result, "updatedAt", "response should contain updatedAt timestamp")

	req = setup.CreateAuthRequest(http.MethodGet, "/api/servers/"+serverID, nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)
	detail := setup.ParseJSONResponse(t, resp)
	require.Equal(t, "Updated Server Name", detail["name"], "name should be persisted in the server detail response")

	t.Logf("Server name updated successfully")
	setup.LogTestPass(t, "TestUpdateName_Success")
}

func TestUpdateName_EmptyName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUpdateName_EmptyName")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "emptyname@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Empty Name Server", "emptyname", 1, false)

	setup.LogTestStep(t, "Testing Update Server Name with Empty Name")
	reqBody := []byte(`{"name":""}`)
	req := setup.CreateAuthRequest(http.MethodPut, "/api/servers/"+serverID+"/name", reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "required", "error message should mention name is required")

	t.Logf("Correctly rejected empty server name")
	setup.LogTestPass(t, "TestUpdateName_EmptyName")
}

func TestUpdateName_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUpdateName_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "unauthupdname@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Unauth Update Name Server", "unauthupdn", 1, false)

	setup.LogTestStep(t, "Testing Update Server Name Without Auth")
	reqBody := []byte(`{"name":"Updated Name"}`)
	req := setup.CreateAuthRequest(http.MethodPut, "/api/servers/"+serverID+"/name", reqBody, "")
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated update server name request")
	setup.LogTestPass(t, "TestUpdateName_Unauthorized")
}

func TestUpdateName_NotOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUpdateName_NotOwner")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token1 := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "ownerupdname@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token1, "Owner Update Name Server", "ownerupdn", 1, false)

	token2 := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "notownerupdname@example.com", "password123")

	setup.LogTestStep(t, "Testing Update Server Name as Non-Owner")
	reqBody := []byte(`{"name":"Hacked Name"}`)
	req := setup.CreateAuthRequest(http.MethodPut, "/api/servers/"+serverID+"/name", reqBody, token2)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "not the owner", "error message should mention not the owner")

	t.Logf("Correctly rejected server name update by non-owner: %s", errMsg)
	setup.LogTestPass(t, "TestUpdateName_NotOwner")
}

func TestUpdateShortName_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUpdateShortName_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "updateshortname@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Update Short Name Server", "updshort", 1, false)

	setup.LogTestStep(t, "Testing Update Server Short Name")
	reqBody := []byte(`{"shortName":"updshort"}`)
	req := setup.CreateAuthRequest(http.MethodPut, "/api/servers/"+serverID+"/shortName", reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "id", "response should contain server id")
	require.Contains(t, result, "updatedAt", "response should contain updatedAt timestamp")

	req = setup.CreateAuthRequest(http.MethodGet, "/api/servers/"+serverID, nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)
	detail := setup.ParseJSONResponse(t, resp)
	require.Equal(t, "updshort", detail["shortName"], "shortName should be persisted")

	t.Logf("Server short name updated successfully")
	setup.LogTestPass(t, "TestUpdateShortName_Success")
}

func TestUpdateShortName_TooLong(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUpdateShortName_TooLong")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "longshortname@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Long Short Name Server", "longshort", 1, false)

	setup.LogTestStep(t, "Testing Update Server Short Name with Too Long Short Name")
	reqBody := []byte(fmt.Sprintf(`{"shortName":"%s"}`, "1234567890123456789012345678901"))
	req := setup.CreateAuthRequest(http.MethodPut, "/api/servers/"+serverID+"/shortName", reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "10", "error message should mention character limit")

	t.Logf("Correctly rejected too long server short name")
	setup.LogTestPass(t, "TestUpdateShortName_TooLong")
}

func TestUpdateCategory_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUpdateCategory_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "updatecategory@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Update Category Server", "updcat", 1, false)

	setup.LogTestStep(t, "Testing Update Server Category")
	reqBody := []byte(`{"categoryId":2}`)
	req := setup.CreateAuthRequest(http.MethodPut, "/api/servers/"+serverID+"/category", reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "id", "response should contain server id")
	require.Contains(t, result, "updatedAt", "response should contain updatedAt timestamp")

	req = setup.CreateAuthRequest(http.MethodGet, "/api/servers/"+serverID, nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)
	detail := setup.ParseJSONResponse(t, resp)
	require.Equal(t, float64(2), detail["categoryId"], "categoryId should be persisted")

	t.Logf("Server category updated successfully")
	setup.LogTestPass(t, "TestUpdateCategory_Success")
}

func TestUpdateDescription_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUpdateDescription_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "updatedesc@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Update Description Server", "updatedesc", 1, false)

	setup.LogTestStep(t, "Testing Update Server Description")
	reqBody := []byte(`{"description":"This is an updated description"}`)
	req := setup.CreateAuthRequest(http.MethodPut, "/api/servers/"+serverID+"/description", reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "id", "response should contain server id")
	require.Contains(t, result, "updatedAt", "response should contain updatedAt timestamp")

	req = setup.CreateAuthRequest(http.MethodGet, "/api/servers/"+serverID, nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)
	detail := setup.ParseJSONResponse(t, resp)
	require.Equal(t, "This is an updated description", detail["description"], "description should be persisted")

	t.Logf("Server description updated successfully")
	setup.LogTestPass(t, "TestUpdateDescription_Success")
}

func TestUpdateSettings_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUpdateSettings_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "updatesettings@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Update Settings Server", "settings", 1, false)

	setup.LogTestStep(t, "Testing Update Server Settings")
	reqBody := []byte(`{"settings":{"isPrivate":true}}`)
	req := setup.CreateAuthRequest(http.MethodPut, "/api/servers/"+serverID+"/settings", reqBody, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)

	t.Logf("Server settings updated successfully")
	setup.LogTestPass(t, "TestUpdateSettings_Success")
}

func TestDelete_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestDelete_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "deleteserver@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Delete Server", "delsrv", 1, false)

	setup.LogTestStep(t, "Testing Delete Server")
	req := setup.CreateAuthRequest(http.MethodDelete, "/api/servers/"+serverID, nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)

	req = setup.CreateAuthRequest(http.MethodGet, "/api/servers/"+serverID, nil, token)
	resp, err = setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error after deletion")

	t.Logf("Server deleted successfully")
	setup.LogTestPass(t, "TestDelete_Success")
}

func TestDelete_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestDelete_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "unauthdelete@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Unauth Delete Server", "unauthdel", 1, false)

	setup.LogTestStep(t, "Testing Delete Server Without Auth")
	req := setup.CreateAuthRequest(http.MethodDelete, "/api/servers/"+serverID, nil, "")
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated delete server request")
	setup.LogTestPass(t, "TestDelete_Unauthorized")
}

func TestDelete_NotOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestDelete_NotOwner")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token1 := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "ownerdelete@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token1, "Owner Delete Server", "ownerdel", 1, false)

	token2 := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "notownerdelete@example.com", "password123")

	setup.LogTestStep(t, "Testing Delete Server as Non-Owner")
	req := setup.CreateAuthRequest(http.MethodDelete, "/api/servers/"+serverID, nil, token2)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "not the owner", "error message should mention not the owner")

	t.Logf("Correctly rejected server deletion by non-owner: %s", errMsg)
	setup.LogTestPass(t, "TestDelete_NotOwner")
}

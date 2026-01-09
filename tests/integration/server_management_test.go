package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// getFirstServerId is a helper to get the first server ID from user's servers
func getFirstServerId(t *testing.T, app *fiber.App, accessToken string) string {
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/me", nil, accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err, "get servers should succeed")

	apiResp := setup.ParseAPIResponse(t, resp)
	dataArray := setup.GetDataAsArray(t, apiResp)
	require.Greater(t, len(dataArray), 0, "should have at least one server")

	firstServer := dataArray[0].(map[string]interface{})
	serverId := firstServer["id"].(string)
	return serverId
}

// TestCreateServer tests the POST /servers/create endpoint
func TestCreateServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err, "infrastructure should start successfully")
	defer func() { _ = infra.Terminate(ctx, t) }()

	t.Log("=== Running Database Migrations ===")
	_ = setup.RunMigration(infra.PgURL, t)

	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Create test user
	t.Log("=== Setup: Creating Test User ===")
	accessToken := createTestUser(t, app, infra.MailhogURL, "serverowner@example.com", "serverowner", "pass123")

	// Test 1: Create server successfully
	t.Log("=== Test 1: Create Server Successfully ===")
	reqBody := []byte(`{"name":"Test Server","shortName":"testsvr","categoryId":1,"settings":{"isPrivate":false}}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err, "create server request should complete")
	require.Equal(t, 200, resp.StatusCode, "create server should return 200")

	result := setup.ParseJSONResponse(t, resp)

	// Verify response contains server data
	require.Contains(t, result, "id", "response should contain server id")
	require.Contains(t, result, "name", "response should contain name")
	require.Contains(t, result, "shortName", "response should contain shortName")

	serverName := result["name"].(string)
	require.Equal(t, "Test Server", serverName, "server name should match")

	shortName := result["shortName"].(string)
	require.Equal(t, "testsvr", shortName, "short name should match")

	t.Logf("✓ Server created successfully: id=%s, name=%s, shortName=%s", result["id"], serverName, shortName)

	// Test 2: Create server with name too short
	t.Log("=== Test 2: Create Server with Name Too Short ===")
	reqBody = []byte(`{"name":"Serv","shortName":"srv","categoryId":1,"settings":{"isPrivate":false}}`)
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param := setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "name", param, "error param should be 'name'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 3: Create server with name too long
	t.Log("=== Test 3: Create Server with Name Too Long ===")
	longName := "This server name is way too long and exceeds the maximum allowed length of forty characters"
	reqBody = []byte(fmt.Sprintf(`{"name":"%s","shortName":"test","categoryId":1,"settings":{"isPrivate":false}}`, longName))
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "name", param, "error param should be 'name'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 4: Create server with empty name
	t.Log("=== Test 4: Create Server with Empty Name ===")
	reqBody = []byte(`{"name":"","shortName":"test","categoryId":1,"settings":{"isPrivate":false}}`)
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "name", param, "error param should be 'name'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 5: Create server with short name too short
	t.Log("=== Test 5: Create Server with Short Name Too Short ===")
	reqBody = []byte(`{"name":"Test Server 2","shortName":"srv","categoryId":1,"settings":{"isPrivate":false}}`)
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "shortName", param, "error param should be 'shortName'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 6: Create server with short name too long
	t.Log("=== Test 6: Create Server with Short Name Too Long ===")
	reqBody = []byte(`{"name":"Test Server 2","shortName":"toolongname","categoryId":1,"settings":{"isPrivate":false}}`)
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "shortName", param, "error param should be 'shortName'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 7: Create server without authentication
	t.Log("=== Test 7: Create Server Without Authentication ===")
	reqBody = []byte(`{"name":"Unauthorized Server","shortName":"unauth","categoryId":1,"settings":{"isPrivate":false}}`)
	req = setup.CreateJSONRequest(http.MethodPost, "/api/servers/create", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, _ = setup.ParseErrorDetail(t, result)

	require.Equal(t, "UNAUTHORIEZED_ERROR", code, "error code should be UNAUTHORIEZED_ERROR")
	require.NotEmpty(t, message, "error message should not be empty")

	t.Logf("✓ Validation Error: Code=%s, Message=%s", code, message)

	t.Log("=== All Create Server Tests Passed ===")
}

// TestGetUserServers tests the GET /servers/me endpoint
func TestGetUserServers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err, "infrastructure should start successfully")
	defer func() { _ = infra.Terminate(ctx, t) }()

	t.Log("=== Running Database Migrations ===")
	_ = setup.RunMigration(infra.PgURL, t)

	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Create test user and server
	t.Log("=== Setup: Creating Test User and Server ===")
	accessToken := createTestUser(t, app, infra.MailhogURL, "serveruser@example.com", "serveruser", "pass123")

	// Create first server
	reqBody := []byte(`{"name":"Server 1","shortName":"server1","categoryId":1,"settings":{"isPrivate":false}}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err, "create server should succeed")

	if resp.StatusCode != 200 {
		result := setup.ParseJSONResponse(t, resp)
		t.Logf("ERROR: Create server returned %d. Response: %+v", resp.StatusCode, result)
	}

	require.Equal(t, 200, resp.StatusCode, "create server 1 should return 200")

	// Verify server 1 created successfully
	result1 := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result1, "id", "server 1 response should contain id")
	t.Logf("✓ Server 1 created: id=%s, name=%s", result1["id"], result1["name"])

	// Create second server
	reqBody = []byte(`{"name":"Server 2","shortName":"server2","categoryId":4,"settings":{"isPrivate":false}}`) // Technology category
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "create server should succeed")
	require.Equal(t, 200, resp.StatusCode, "create server 2 should return 200")

	// Verify server 2 created successfully
	result2 := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result2, "id", "server 2 response should contain id")
	t.Logf("✓ Server 2 created: id=%s, name=%s", result2["id"], result2["name"])

	// Test 1: Get user servers successfully
	t.Log("=== Test 1: Get User Servers Successfully ===")
	req = setup.CreateAuthRequest(http.MethodGet, "/api/servers/me", nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "get servers request should complete")
	require.Equal(t, 200, resp.StatusCode, "get servers should return 200")

	apiResp := setup.ParseAPIResponse(t, resp)

	dataArray := setup.GetDataAsArray(t, apiResp)
	require.GreaterOrEqual(t, len(dataArray), 2, "should have at least 2 servers")

	t.Logf("✓ Retrieved %d servers", len(dataArray))

	// Test 2: Get user servers with pagination
	t.Log("=== Test 2: Get User Servers with Pagination ===")
	req = setup.CreateAuthRequest(http.MethodGet, "/api/servers/me?limit=1", nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "get servers with limit should complete")

	apiResp = setup.ParseAPIResponse(t, resp)

	dataArray = setup.GetDataAsArray(t, apiResp)
	require.Len(t, dataArray, 1, "should return exactly 1 server with limit=1")

	nextCursor := setup.GetNextCursor(t, apiResp)
	require.NotEmpty(t, nextCursor, "should have next cursor for pagination")

	t.Logf("✓ Pagination works: limit=1, nextCursor=%s", nextCursor)

	// Test 3: Get user servers without authentication
	t.Log("=== Test 3: Get User Servers Without Authentication ===")
	req = setup.CreateJSONRequest(http.MethodGet, "/api/servers/me", nil)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result := setup.ParseJSONResponse(t, resp)
	code, message, _ := setup.ParseErrorDetail(t, result)

	require.Equal(t, "UNAUTHORIEZED_ERROR", code, "error code should be UNAUTHORIEZED_ERROR")
	require.NotEmpty(t, message, "error message should not be empty")

	t.Logf("✓ Validation Error: Code=%s, Message=%s", code, message)

	t.Log("=== All Get User Servers Tests Passed ===")
}

// TestUpdateServerName tests the PUT /servers/:id/name endpoint
func TestUpdateServerName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err, "infrastructure should start successfully")
	defer func() { _ = infra.Terminate(ctx, t) }()

	t.Log("=== Running Database Migrations ===")
	_ = setup.RunMigration(infra.PgURL, t)

	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Create test user and server
	t.Log("=== Setup: Creating Test User and Server ===")
	accessToken := createTestUser(t, app, infra.MailhogURL, "updater@example.com", "updater", "pass123")

	reqBody := []byte(`{"name":"Original Name","shortName":"original","categoryId":1,"settings":{"isPrivate":false}}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err, "create server should succeed")
	require.Equal(t, 200, resp.StatusCode, "create server should return 200")

	// Get serverId from user's servers list
	serverId := getFirstServerId(t, app, accessToken)

	// Test 1: Update server name successfully
	t.Log("=== Test 1: Update Server Name Successfully ===")
	reqBody = []byte(`{"name":"Updated Server Name"}`)
	req = setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/name", serverId), reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "update name request should complete")
	require.Equal(t, 200, resp.StatusCode, "update name should return 200")

	result := setup.ParseJSONResponse(t, resp)

	// Verify response contains server data
	require.Contains(t, result, "id", "response should contain server id")
	require.Contains(t, result, "name", "response should contain name")
	require.Contains(t, result, "shortName", "response should contain shortName")

	updatedName := result["name"].(string)
	require.Equal(t, "Updated Server Name", updatedName, "name should be updated")

	t.Logf("✓ Server name updated successfully: id=%s, name=%s", result["id"], updatedName)

	// Test 2: Update with name too short
	t.Log("=== Test 2: Update with Name Too Short ===")
	reqBody = []byte(`{"name":"Serv"}`)
	req = setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/name", serverId), reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param := setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "name", param, "error param should be 'name'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 3: Update with empty name
	t.Log("=== Test 3: Update with Empty Name ===")
	reqBody = []byte(`{"name":""}`)
	req = setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/name", serverId), reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "name", param, "error param should be 'name'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Create another user and try to update the server (should fail)
	t.Log("=== Test 4: Update Server by Non-Owner ===")
	otherAccessToken := createTestUser(t, app, infra.MailhogURL, "otheruser@example.com", "otheruser", "pass123")

	reqBody = []byte(`{"name":"Hacked Name"}`)
	req = setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/name", serverId), reqBody, otherAccessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, _ = setup.ParseErrorDetail(t, result)

	// Should get error - non-owner cannot update
	require.NotEmpty(t, code, "should return error for non-owner")

	t.Logf("✓ Non-owner correctly blocked: Code=%s, Message=%s", code, message)

	t.Log("=== All Update Server Name Tests Passed ===")
}

// TestUpdateServerShortName tests the PUT /servers/:id/shortName endpoint
func TestUpdateServerShortName(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err, "infrastructure should start successfully")
	defer func() { _ = infra.Terminate(ctx, t) }()

	t.Log("=== Running Database Migrations ===")
	_ = setup.RunMigration(infra.PgURL, t)

	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Create test user and server
	t.Log("=== Setup: Creating Test User and Server ===")
	accessToken := createTestUser(t, app, infra.MailhogURL, "shortname@example.com", "shortname", "pass123")

	reqBody := []byte(`{"name":"Test Server","shortName":"oldname","categoryId":1,"settings":{"isPrivate":false}}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, accessToken)
	_, err = app.Test(req)
	require.NoError(t, err, "create server should succeed")

	// Get serverId from user's servers list
	serverId := getFirstServerId(t, app, accessToken)

	// Test 1: Update short name successfully
	t.Log("=== Test 1: Update Short Name Successfully ===")
	reqBody = []byte(`{"shortName":"newname"}`)
	req = setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/shortName", serverId), reqBody, accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err, "update shortName request should complete")
	require.Equal(t, 200, resp.StatusCode, "update shortName should return 200")

	result := setup.ParseJSONResponse(t, resp)

	// Verify response contains server data
	require.Contains(t, result, "id", "response should contain server id")
	require.Contains(t, result, "shortName", "response should contain shortName")

	updatedShortName := result["shortName"].(string)
	require.Equal(t, "newname", updatedShortName, "shortName should be updated")

	t.Logf("✓ Short name updated successfully: id=%s, shortName=%s", result["id"], updatedShortName)

	// Test 2: Update with short name too short
	t.Log("=== Test 2: Update with Short Name Too Short ===")
	reqBody = []byte(`{"shortName":"name"}`)
	req = setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/shortName", serverId), reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param := setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "shortName", param, "error param should be 'shortName'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 3: Update with short name too long
	t.Log("=== Test 3: Update with Short Name Too Long ===")
	reqBody = []byte(`{"shortName":"toolongname"}`)
	req = setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/shortName", serverId), reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "shortName", param, "error param should be 'shortName'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	t.Log("=== All Update Server Short Name Tests Passed ===")
}

// TestUpdateServerCategory tests the PUT /servers/:id/category endpoint
func TestUpdateServerCategory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err, "infrastructure should start successfully")
	defer func() { _ = infra.Terminate(ctx, t) }()

	t.Log("=== Running Database Migrations ===")
	_ = setup.RunMigration(infra.PgURL, t)

	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Create test user and server
	t.Log("=== Setup: Creating Test User and Server ===")
	accessToken := createTestUser(t, app, infra.MailhogURL, "category@example.com", "category", "pass123")

	reqBody := []byte(`{"name":"Test Server","shortName":"testsvr","categoryId":1,"settings":{"isPrivate":false}}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, accessToken)
	_, err = app.Test(req)
	require.NoError(t, err, "create server should succeed")

	// Get serverId from user's servers list
	serverId := getFirstServerId(t, app, accessToken)

	// Test 1: Update category successfully
	t.Log("=== Test 1: Update Category Successfully ===")
	reqBody = []byte(`{"categoryId":4}`) // Technology category
	req = setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/category", serverId), reqBody, accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err, "update category request should complete")
	require.Equal(t, 200, resp.StatusCode, "update category should return 200")

	result := setup.ParseJSONResponse(t, resp)

	// Verify response contains server data
	require.Contains(t, result, "id", "response should contain server id")
	require.Contains(t, result, "categoryId", "response should contain categoryId")

	updatedCategoryId := result["categoryId"]
	require.Equal(t, float64(4), updatedCategoryId, "categoryId should be updated to 4")

	t.Logf("✓ Category updated successfully: id=%s, categoryId=%v", result["id"], updatedCategoryId)

	t.Log("=== All Update Server Category Tests Passed ===")
}

// TestUpdateServerDescription tests the PUT /servers/:id/description endpoint
func TestUpdateServerDescription(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err, "infrastructure should start successfully")
	defer func() { _ = infra.Terminate(ctx, t) }()

	t.Log("=== Running Database Migrations ===")
	_ = setup.RunMigration(infra.PgURL, t)

	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Create test user and server
	t.Log("=== Setup: Creating Test User and Server ===")
	accessToken := createTestUser(t, app, infra.MailhogURL, "desc@example.com", "desc", "pass123")

	reqBody := []byte(`{"name":"Test Server","shortName":"testsvr","categoryId":1,"settings":{"isPrivate":false}}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, accessToken)
	_, err = app.Test(req)
	require.NoError(t, err, "create server should succeed")

	// Get serverId from user's servers list
	serverId := getFirstServerId(t, app, accessToken)

	// Test 1: Update description successfully
	t.Log("=== Test 1: Update Description Successfully ===")
	descText := "This is a test server for integration testing"
	reqBody = []byte(fmt.Sprintf(`{"description":"%s"}`, descText))
	req = setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/description", serverId), reqBody, accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err, "update description request should complete")
	require.Equal(t, 200, resp.StatusCode, "update description should return 200")

	result := setup.ParseJSONResponse(t, resp)

	// Verify response contains server data
	require.Contains(t, result, "id", "response should contain server id")
	require.Contains(t, result, "description", "response should contain description")

	updatedDesc := result["description"].(string)
	require.Equal(t, descText, updatedDesc, "description should be updated")

	t.Logf("✓ Description updated successfully: id=%s, description=%s", result["id"], updatedDesc)

	// Test 2: Update description with empty string (should be allowed)
	t.Log("=== Test 2: Update Description with Empty String ===")
	reqBody = []byte(`{"description":""}`)
	req = setup.CreateAuthRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/description", serverId), reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "update description with empty string should succeed")
	require.Equal(t, 200, resp.StatusCode, "update description should return 200")

	result = setup.ParseJSONResponse(t, resp)

	// Verify response contains server data
	require.Contains(t, result, "id", "response should contain server id")
	require.Contains(t, result, "description", "response should contain description")

	updatedDesc, ok := result["description"].(string)
	if ok {
		require.Empty(t, updatedDesc, "description should be empty")
		t.Logf("✓ Description cleared successfully: id=%s, description=(empty)", result["id"])
	} else {
		// description might be null when empty
		t.Logf("✓ Description cleared successfully: id=%s, description=null", result["id"])
	}

	t.Log("=== All Update Server Description Tests Passed ===")
}

// TestDeleteServer tests the DELETE /servers/:id endpoint
func TestDeleteServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err, "infrastructure should start successfully")
	defer func() { _ = infra.Terminate(ctx, t) }()

	t.Log("=== Running Database Migrations ===")
	_ = setup.RunMigration(infra.PgURL, t)

	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Create test user and server
	t.Log("=== Setup: Creating Test User and Server ===")
	accessToken := createTestUser(t, app, infra.MailhogURL, "deleter@example.com", "deleter", "pass123")

	reqBody := []byte(`{"name":"Server To Delete","shortName":"delsvr","categoryId":1,"settings":{"isPrivate":false}}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err, "create server should succeed")
	require.Equal(t, 200, resp.StatusCode, "create server should return 200")

	// Get serverId from user's servers list
	serverId := getFirstServerId(t, app, accessToken)

	// Test 1: Delete server successfully
	t.Log("=== Test 1: Delete Server Successfully ===")
	req = setup.CreateAuthRequest(http.MethodDelete, fmt.Sprintf("/api/servers/%s", serverId), nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "delete server request should complete")
	require.Equal(t, 200, resp.StatusCode, "delete server should return 200")

	result := setup.ParseJSONResponse(t, resp)
	status, ok := result["status"].(string)
	require.True(t, ok, "response should have status field")
	require.Equal(t, "OK", status, "status should be 'OK'")

	t.Log("✓ Server deleted successfully")

	// Test 2: Try to access deleted server
	t.Log("=== Test 2: Access Deleted Server ===")
	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s", serverId), nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)

	// Server should not be accessible after deletion
	// Acceptable responses: 404 (Not Found), 405 (Method Not Allowed), or error response
	require.NotEqual(t, 200, resp.StatusCode, "deleted server should not return 200")

	if errorVal, hasError := result["error"]; hasError {
		t.Logf("✓ Deleted server correctly returns error: %+v", errorVal)
	} else {
		t.Logf("✓ Deleted server correctly returns non-200 status: %d", resp.StatusCode)
	}

	t.Log("=== All Delete Server Tests Passed ===")
}

package server

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestCreatePost_Success tests successful server creation
func TestCreatePost_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err)
	defer func() { _ = infra.Terminate(ctx, t) }()

	_ = setup.RunMigration(infra.PgURL, t)
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Setup: Create and login user
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "server@example.com", "password123")

	// Test: Create server
	t.Log("=== Testing Create Server ===")
	serverName := "Test Server " + setup.GenerateRandomString(6)
	shortName := "test" + setup.GenerateRandomString(4)

	reqBody := []byte(fmt.Sprintf(`{
		"name": "%s",
		"shortName": "%s",
		"categoryId": 1,
		"description": "This is a test server",
		"settings": {"isPrivate": false}
	}`, serverName, shortName))

	req := setup.CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, token)
	resp, err := app.Test(req)
	require.NoError(t, err, "create server request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "id", "response should contain server id")

	serverID := result["id"].(string)
	require.NotEmpty(t, serverID, "server id should not be empty")

	t.Logf("Server created successfully: %s", serverID)
}

// TestCreatePost_Validation tests server creation validation
func TestCreatePost_Validation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err)
	defer func() { _ = infra.Terminate(ctx, t) }()

	_ = setup.RunMigration(infra.PgURL, t)
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	token := setup.CreateTestUser(t, app, infra.MailhogURL, "validation@example.com", "password123")

	// Test 1: Empty name
	t.Log("=== Test 1: Empty Server Name ===")
	reqBody := []byte(`{"name":"","shortName":"test","categoryId":1,"settings":{"isPrivate":false}}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, token)
	resp, err := app.Test(req)
	require.NoError(t, err, "create server request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")

	// Test 2: Empty shortName
	t.Log("=== Test 2: Empty Short Name ===")
	reqBody = []byte(`{"name":"Test Server","shortName":"","categoryId":1,"settings":{"isPrivate":false}}`)
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, token)
	resp, err = app.Test(req)
	require.NoError(t, err, "create server request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")

	// Test 3: Invalid categoryId
	t.Log("=== Test 3: Invalid Category ID ===")
	reqBody = []byte(`{"name":"Test Server","shortName":"test","categoryId":9999,"settings":{"isPrivate":false}}`)
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, token)
	resp, err = app.Test(req)
	require.NoError(t, err, "create server request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")

	t.Logf("Validation tests completed")
}

// TestCreatePost_Unauthorized tests server creation without authentication
func TestCreatePost_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err)
	defer func() { _ = infra.Terminate(ctx, t) }()

	_ = setup.RunMigration(infra.PgURL, t)
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Test: Create server without token
	t.Log("=== Testing Create Server Without Auth ===")
	reqBody := []byte(`{"name":"Test Server","shortName":"test","categoryId":1,"settings":{"isPrivate":false}}`)
	req := setup.CreateJSONRequest(http.MethodPost, "/api/servers/create", reqBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "create server request should complete")

	// Should return unauthorized
	setup.RequireStatus(t, resp, 401)
	t.Logf("Correctly rejected unauthenticated request")
}

// TestCreatePost_PrivateServer tests creating a private server
func TestCreatePost_PrivateServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err)
	defer func() { _ = infra.Terminate(ctx, t) }()

	_ = setup.RunMigration(infra.PgURL, t)
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	token := setup.CreateTestUser(t, app, infra.MailhogURL, "private@example.com", "password123")

	// Test: Create private server
	t.Log("=== Testing Create Private Server ===")
	reqBody := []byte(`{
		"name": "Private Server",
		"shortName": "private",
		"categoryId": 1,
		"description": "This is a private server",
		"settings": {"isPrivate": true}
	}`)

	req := setup.CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, token)
	resp, err := app.Test(req)
	require.NoError(t, err, "create private server request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	serverID := result["id"].(string)
	t.Logf("Private server created successfully: %s", serverID)

	// Verify server is private by trying to access with another user
	token2 := setup.CreateTestUser(t, app, infra.MailhogURL, "other@example.com", "password123")
	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s", serverID), nil, token2)
	resp, err = app.Test(req)
	require.NoError(t, err, "get private server request should complete")

	result = setup.ParseJSONResponse(t, resp)
	// Should return error since user is not a member
	require.Contains(t, result, "error", "should not be able to access private server")

	t.Logf("Private server correctly inaccessible to non-members")
}

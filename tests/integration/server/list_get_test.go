package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestListGet_Success(t *testing.T) {
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

	token := setup.CreateTestUser(t, app, infra.MailhogURL, "list@example.com", "password123")
	_ = setup.CreateTestServer(t, app, infra.RedisURL, token, "Discovery Server", "disco", 1, false)

	t.Log("=== Testing Get Discovery Servers ===")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/", nil, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get servers request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "data", "response should contain data")
	require.Contains(t, result, "page", "response should contain pagination info")

	t.Logf("Server list retrieved successfully")
}

func TestList_Get_WithCategory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestList_Get_WithCategory")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "category@example.com", "password123")
	_ = setup.CreateTestServer(t, app, setup.GetGlobalInfra().RedisURL, token, "Category Server", "cat", 1, false)

	setup.LogTestStep(t, "Testing Get Discovery Servers by Category")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/?categoryId=1", nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get servers by category request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "data", "response should contain data")

	t.Logf("Server list by category retrieved successfully")
	setup.LogTestPass(t, "TestList_Get_WithCategory")
}

func TestList_Get_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestList_Get_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	setup.LogTestStep(t, "Testing Get Servers Without Auth")
	req := setup.CreateJSONRequest(http.MethodGet, "/api/servers/", nil)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "get servers request should complete")

	if resp.StatusCode != 200 && resp.StatusCode != 401 && resp.StatusCode != 404 {
		t.Logf("Got unexpected status: %d", resp.StatusCode)
	}
	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated request with status: %d", resp.StatusCode)

	setup.LogTestPass(t, "TestList_Get_Unauthorized")
}

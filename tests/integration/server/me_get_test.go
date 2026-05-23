package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestMeGet_Success tests successful get user servers
func TestMeGet_Success(t *testing.T) {
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

	// Setup: Create user and servers
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "meserver@example.com", "password123")
	_ = setup.CreateTestServer(t, app, infra.RedisURL, token, "Server 1", "server1", 1, false)
	_ = setup.CreateTestServer(t, app, infra.RedisURL, token, "Server 2", "server2", 2, false)

	// Test: Get user servers
	t.Log("=== Testing Get User Servers ===")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/me", nil, token)
	resp, err := app.Test(req)
	require.NoError(t, err, "get user servers request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "data", "response should contain data")

	t.Logf("User servers retrieved successfully")
}

// TestMeGet_Unauthorized tests get user servers without authentication
func TestMeGet_Unauthorized(t *testing.T) {
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

	// Test: Get user servers without token
	t.Log("=== Testing Get User Servers Without Auth ===")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/me", nil, "")
	resp, err := app.Test(req)
	require.NoError(t, err, "get user servers request should complete")

	// Should return unauthorized (404 from auth middleware)
	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated get user servers request")
}

// TestMeGet_EmptyList tests get user servers when user has no servers
func TestMeGet_EmptyList(t *testing.T) {
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

	token := setup.CreateTestUser(t, app, infra.MailhogURL, "noserver@example.com", "password123")

	// Test: Get user servers (user has no servers)
	t.Log("=== Testing Get User Servers with No Servers ===")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/me", nil, token)
	resp, err := app.Test(req)
	require.NoError(t, err, "get user servers request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "data", "response should contain data")

	t.Logf("Empty server list retrieved successfully")
}

// TestMeGet_WithPagination tests get user servers with pagination
func TestMeGet_WithPagination(t *testing.T) {
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

	token := setup.CreateTestUser(t, app, infra.MailhogURL, "meserverpage@example.com", "password123")
	_ = setup.CreateTestServer(t, app, infra.RedisURL, token, "Server 1", "server1", 1, false)
	_ = setup.CreateTestServer(t, app, infra.RedisURL, token, "Server 2", "server2", 2, false)
	_ = setup.CreateTestServer(t, app, infra.RedisURL, token, "Server 3", "server3", 3, false)

	// Test: Get user servers with limit
	t.Log("=== Testing Get User Servers With Pagination ===")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/me?limit=2", nil, token)
	resp, err := app.Test(req)
	require.NoError(t, err, "get user servers request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "data", "response should contain data")

	t.Logf("User servers retrieved successfully with pagination")
}

package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

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

	token := setup.CreateTestUser(t, app, infra.MailhogURL, "meserver@example.com", "password123")
	_ = setup.CreateTestServer(t, app, infra.RedisURL, token, "Server 1", "server1", 1, false)
	_ = setup.CreateTestServer(t, app, infra.RedisURL, token, "Server 2", "server2", 2, false)

	t.Log("=== Testing Get User Servers ===")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/me", nil, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get user servers request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "data", "response should contain data")

	t.Logf("User servers retrieved successfully")
}

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

	t.Log("=== Testing Get User Servers Without Auth ===")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/me", nil, "")
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get user servers request should complete")

	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated get user servers request")
}

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

	t.Log("=== Testing Get User Servers with No Servers ===")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/me", nil, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get user servers request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "data", "response should contain data")

	t.Logf("Empty server list retrieved successfully")
}

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

	t.Log("=== Testing Get User Servers With Pagination ===")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/me?limit=2", nil, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get user servers request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "data", "response should contain data")

	t.Logf("User servers retrieved successfully with pagination")
}

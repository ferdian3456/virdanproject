package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestGetById_Success tests successful get server by ID
func TestGetById_Success(t *testing.T) {
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

	// Setup: Create user and server
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "getbyid@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, token, "Get By ID Server", "getbyid", 1, false)

	// Test: Get server by ID
	t.Log("=== Testing Get Server By ID ===")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/"+serverID, nil, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get server by ID request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "id", "response should contain server id")

	t.Logf("Server retrieved successfully by ID")
}

// TestGetById_Unauthorized tests get server by ID without authentication
func TestGetById_Unauthorized(t *testing.T) {
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

	// Setup: Create user and server
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "unauthget@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, token, "Unauth Get Server", "unauthget", 1, false)

	// Test: Get server by ID without token
	t.Log("=== Testing Get Server By ID Without Auth ===")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/"+serverID, nil, "")
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get server by ID request should complete")

	// Should return unauthorized (404 from auth middleware)
	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated get server by ID request")
}

// TestGetById_InvalidServerId tests get server by ID with invalid server ID
func TestGetById_InvalidServerId(t *testing.T) {
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

	token := setup.CreateTestUser(t, app, infra.MailhogURL, "invalidget@example.com", "password123")

	// Test: Get server by ID with invalid server ID
	t.Log("=== Testing Get Server By ID with Invalid Server ID ===")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/00000000-0000-0000-0000-000000000000", nil, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get server by ID request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error")
	errMsg := setup.ParseErrorMessage(t, result)
	t.Logf("Error message for invalid server ID: %s", errMsg)
}

// TestGetById_NotAMember tests get server by ID when user is not a member
func TestGetById_NotAMember(t *testing.T) {
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

	// Setup: Create user and server (user is NOT a member)
	token1 := setup.CreateTestUser(t, app, infra.MailhogURL, "ownerget@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, token1, "Owner Get Server", "ownerget", 1, false)

	// Create another user (not a member of the server)
	token2 := setup.CreateTestUser(t, app, infra.MailhogURL, "nonmemberget@example.com", "password123")

	// Test: Get server by ID as non-member. The API does NOT block reads for
	// non-members today (see TD-002); it instead surfaces an `isMember:false`
	// flag in the detail response. Assert on that flag rather than expecting
	// an error response.
	t.Log("=== Testing Get Server By ID as Non-Member ===")
	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/"+serverID, nil, token2)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "get server by ID request should complete")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Equal(t, false, result["isMember"], "non-member should see isMember=false")
	t.Logf("Non-member sees isMember=false (membership check is not enforced server-side yet)")
}

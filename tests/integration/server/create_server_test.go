package server

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// baseCreateServerFields returns the minimum multipart fields the
// POST /api/servers/create endpoint accepts in the multi-identity flow.
// Callers override the entries they want to mutate.
func baseCreateServerFields() map[string]string {
	return map[string]string{
		"name":        "Test Server",
		"shortName":   "testsrv",
		"categoryId":  "1",
		"isPrivate":   "false",
		"nickname":    "Owner",
		"username":    "owner",
		"description": "Test server description",
	}
}

// TestCreateServer_Success tests successful server creation.
func TestCreateServer_Success(t *testing.T) {
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

	// Setup: create user.
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "server@example.com", "password123")

	// Test: create server.
	t.Log("=== Testing Create Server ===")
	fields := baseCreateServerFields()
	fields["name"] = "Test Server " + setup.GenerateRandomString(6)
	fields["shortName"] = "test" + setup.GenerateRandomString(4)
	fields["username"] = "owner" + setup.GenerateRandomString(4)

	body, contentType := setup.CreateMultipartTextOnly(t, fields)
	req := setup.CreateAuthMultipartRequest(http.MethodPost, "/api/servers/create", body, contentType, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "create server request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "server", "response should contain server object")
	require.Contains(t, result, "identity", "response should contain identity object (per-server profile)")

	serverObj, ok := result["server"].(map[string]interface{})
	require.True(t, ok, "server field should be an object")
	serverID, ok := serverObj["id"].(string)
	require.True(t, ok, "server.id should be a string")
	require.NotEmpty(t, serverID, "server id should not be empty")

	identity, ok := result["identity"].(map[string]interface{})
	require.True(t, ok, "identity field should be an object")
	require.NotEmpty(t, identity["profileId"], "identity.profileId should not be empty")
	require.Equal(t, fields["nickname"], identity["nickname"], "identity.nickname should match request")
	require.Equal(t, fields["username"], identity["username"], "identity.username should match request")

	t.Logf("Server created successfully: %s", serverID)
}

// TestCreateServer_Validation tests server creation validation.
func TestCreateServer_Validation(t *testing.T) {
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

	cases := []struct {
		name      string
		mutate    func(map[string]string)
		errSubstr string
	}{
		{
			name:      "EmptyName",
			mutate:    func(f map[string]string) { f["name"] = "" },
			errSubstr: "name",
		},
		{
			name:      "EmptyShortName",
			mutate:    func(f map[string]string) { f["shortName"] = "" },
			errSubstr: "shortName",
		},
		{
			name:      "EmptyNickname",
			mutate:    func(f map[string]string) { f["nickname"] = "" },
			errSubstr: "nickname",
		},
		{
			name:      "EmptyUsername",
			mutate:    func(f map[string]string) { f["username"] = "" },
			errSubstr: "username",
		},
		{
			name:      "InvalidCategoryId",
			mutate:    func(f map[string]string) { f["categoryId"] = strconv.Itoa(9999) },
			errSubstr: "Category",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			fields := baseCreateServerFields()
			tc.mutate(fields)

			body, contentType := setup.CreateMultipartTextOnly(t, fields)
			req := setup.CreateAuthMultipartRequest(http.MethodPost, "/api/servers/create", body, contentType, token)
			resp, err := setup.AppTest(t, app, req)
			require.NoError(t, err, "create server request should complete")

			result := setup.ParseJSONResponse(t, resp)
			require.Contains(t, result, "error", "response should contain error")
			errMsg := setup.ParseErrorMessage(t, result)
			require.Contains(t, errMsg, tc.errSubstr, "error message should mention %q", tc.errSubstr)
		})
	}
}

// TestCreateServer_Unauthorized tests server creation without authentication.
func TestCreateServer_Unauthorized(t *testing.T) {
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

	// Test: create server without token.
	t.Log("=== Testing Create Server Without Auth ===")
	body, contentType := setup.CreateMultipartTextOnly(t, baseCreateServerFields())
	req := setup.CreateAuthMultipartRequest(http.MethodPost, "/api/servers/create", body, contentType, "")
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "create server request should complete")

	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated request")
}

// TestCreateServer_PrivateServer tests creating a private server.
func TestCreateServer_PrivateServer(t *testing.T) {
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

	// Test: create private server.
	t.Log("=== Testing Create Private Server ===")
	fields := baseCreateServerFields()
	fields["name"] = "Private Server"
	fields["shortName"] = "private"
	fields["isPrivate"] = "true"
	fields["nickname"] = "PrivateOwner"
	fields["username"] = "privateowner"
	fields["description"] = "This is a private server"

	body, contentType := setup.CreateMultipartTextOnly(t, fields)
	req := setup.CreateAuthMultipartRequest(http.MethodPost, "/api/servers/create", body, contentType, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "create private server request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	serverObj, ok := result["server"].(map[string]interface{})
	require.True(t, ok, "server field should be an object")
	serverID, ok := serverObj["id"].(string)
	require.True(t, ok, "server.id should be a string")
	t.Logf("Private server created successfully: %s", serverID)

	// Verify server is private by trying to access it as a non-member.
	token2 := setup.CreateTestUser(t, app, infra.MailhogURL, "other@example.com", "password123")
	req = setup.CreateAuthRequest(http.MethodGet, "/api/servers/"+serverID, nil, token2)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "get private server request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "should not be able to access private server")
	t.Logf("Private server correctly inaccessible to non-members")
}

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestSignupStart tests the first step of signup flow
func TestSignupStart(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// 1. Start infrastructure
	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err)

	t.Cleanup(func() {
		t.Log("=== Cleaning Up Test Infrastructure ===")
	defer func() { _ = infra.Terminate(ctx, t) }()
	})

	// 2. Run migrations
	t.Log("=== Running Database Migrations ===")
	_ = setup.RunMigration(infra.PgURL, t)
	require.NoError(t, err)

	// 3. Setup test app
	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)

	// Cleanup database after test
	t.Cleanup(func() {
		t.Log("=== Cleaning Up Database ===")
		setup.TruncateAllTables(t, db, ctx)
	})

	// 4. Test: Signup start with valid email
	t.Log("=== Testing Signup Start ===")
	reqBody := []byte(`{"email":"test@example.com"}`)
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)

	resp, err := app.Test(req)
	require.NoError(t, err, "request should succeed")
	require.Equal(t, 200, resp.StatusCode, "status code should be 200")

	// Note: For now we just test that the endpoint doesn't error
	// In a real test, we would:
	// 1. Parse response to get sessionId
	// 2. Mock/bypass SMTP to get OTP
	// 3. Call /api/auth/signup/otp
	// 4. Call /api/auth/signup/username
	// 5. Call /api/auth/signup/password
	// 6. Login and get token
}

// TestHealthCheck tests the health check endpoint
func TestHealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Log("Starting health check test")

	ctx := context.Background()

	// 1. Start infrastructure
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err)

	t.Cleanup(func() {
	defer func() { _ = infra.Terminate(ctx, t) }()
	})

	// 2. Setup test app
	app, _, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogURL)

	// 3. Test: Health check
	req := setup.CreateJSONRequest(http.MethodGet, "/api/health", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	t.Log("Health check test passed")
}

// TestCompleteSignupFlow tests the entire signup flow including OTP verification
func TestCompleteSignupFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// 1. Start infrastructure
	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err)

	t.Cleanup(func() {
		t.Log("=== Cleaning Up Test Infrastructure ===")
	defer func() { _ = infra.Terminate(ctx, t) }()
	})

	// 2. Run migrations
	t.Log("=== Running Database Migrations ===")
	_ = setup.RunMigration(infra.PgURL, t)
	require.NoError(t, err)

	// 3. Setup test app
	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)

	// Cleanup database after test
	t.Cleanup(func() {
		t.Log("=== Cleaning Up Database ===")
		setup.TruncateAllTables(t, db, ctx)
	})

	// 4. Test: Complete signup flow
	testEmail := "completesignup@example.com"

	// Step 1: Signup start
	t.Log("=== Step 1: Signup Start ===")
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)

	resp, err := app.Test(req)
	require.NoError(t, err, "signup start request should succeed")
	require.Equal(t, 200, resp.StatusCode, "signup start should return 200")

	// Parse response untuk dapat sessionId
	result := setup.ParseJSONResponse(t, resp)
	t.Logf("Signup start response: %+v", result)

	// Response directly has sessionId and otpExpiresAt (no "data" wrapper)
	sessionId, ok := result["sessionId"].(string)
	require.True(t, ok, "response should have sessionId")
	require.NotEmpty(t, sessionId, "sessionId should not be empty")

	t.Logf("Session created: %s", sessionId)

	// Step 2: Get OTP from MailHog
	t.Log("=== Step 2: Fetch OTP from MailHog ===")
	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	t.Logf("OTP obtained: %s", otp)

	// Step 3: Verify OTP
	t.Log("=== Step 3: Verify OTP ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)

	resp, err = app.Test(req)
	require.NoError(t, err, "OTP verification should succeed")
	require.Equal(t, 200, resp.StatusCode, "OTP verification should return 200")

	// Step 4: Set username
	t.Log("=== Step 4: Set Username ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":"testuser%s"}`, sessionId, setup.GenerateRandomString(6)))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)

	resp, err = app.Test(req)
	require.NoError(t, err, "set username should succeed")
	require.Equal(t, 200, resp.StatusCode, "set username should return 200")

	// Step 5: Set password
	t.Log("=== Step 5: Set Password ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":"Password123"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)

	resp, err = app.Test(req)
	require.NoError(t, err, "set password should succeed")
	require.Equal(t, 200, resp.StatusCode, "set password should return 200")

	// Step 6: Parse token response
	result = setup.ParseJSONResponse(t, resp)
	t.Logf("Set password response: %+v", result)

	// Response directly has accessToken (no "data" wrapper)
	accessToken, ok := result["accessToken"].(string)
	require.True(t, ok, "response should have accessToken")
	require.NotEmpty(t, accessToken, "accessToken should not be empty")

	t.Logf("Signup completed successfully! Access token received")

	// Optional: Verify user actually created in database
	// (Tambahkan validasi database kalau perlu)
}

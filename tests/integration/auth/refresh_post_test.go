package auth

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func loginAndReturnTokens(t *testing.T, app *fiber.App, mailhogURL, email, password string) (accessToken, refreshToken string) {
	t.Helper()
	startBody := []byte(fmt.Sprintf(`{"email":%q}`, email))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", startBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "signup start should succeed")
	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	otp := setup.GetOTPFromMailhog(t, mailhogURL, email)
	otpBody := []byte(fmt.Sprintf(`{"sessionId":%q,"otp":%q}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", otpBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "verify OTP should succeed")
	setup.RequireStatus(t, resp, 200)

	pwBody := []byte(fmt.Sprintf(`{"sessionId":%q,"password":%q}`, sessionId, password))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", pwBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "set password should succeed")

	result = setup.ParseJSONResponse(t, resp)
	accessToken = result["accessToken"].(string)
	refreshToken = result["refreshToken"].(string)
	return accessToken, refreshToken
}

func TestRefresh_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestRefresh_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	access, refresh := loginAndReturnTokens(t, app, infra.MailhogURL, "refresh@example.com", "password123")
	require.NotEmpty(t, access, "initial access token should not be empty")
	require.NotEmpty(t, refresh, "initial refresh token should not be empty")

	reqBody := []byte(fmt.Sprintf(`{"refreshToken":%q}`, refresh))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/refresh", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "refresh request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "accessToken", "response should contain new accessToken")
	require.Contains(t, result, "refreshToken", "response should contain new refreshToken")
	require.NotEqual(t, refresh, result["refreshToken"], "refreshToken should be rotated")
	setup.LogTestPass(t, "TestRefresh_Success")
}

func TestRefresh_ReusedToken(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestRefresh_ReusedToken")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	_, refresh := loginAndReturnTokens(t, app, infra.MailhogURL, "refreshreuse@example.com", "password123")

	reqBody := []byte(fmt.Sprintf(`{"refreshToken":%q}`, refresh))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/refresh", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "first refresh should succeed")
	setup.RequireStatus(t, resp, 200)

	reqBody = []byte(fmt.Sprintf(`{"refreshToken":%q}`, refresh))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/refresh", reqBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "second refresh request should complete")
	require.NotEqual(t, 200, resp.StatusCode, "reusing a refresh token must not succeed")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error after reuse")
	setup.LogTestPass(t, "TestRefresh_ReusedToken")
}

func TestRefresh_InvalidToken(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestRefresh_InvalidToken")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	reqBody := []byte(`{"refreshToken":"not-a-real-refresh-token"}`)
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/refresh", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "refresh request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error for invalid token")
	setup.LogTestPass(t, "TestRefresh_InvalidToken")
}

func TestRefresh_EmptyRefreshToken(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestRefresh_EmptyRefreshToken")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/refresh", []byte(`{"refreshToken":""}`))
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "refresh request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "empty refresh token should be rejected")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "required", "validator should mark refreshToken as required")
	setup.LogTestPass(t, "TestRefresh_EmptyRefreshToken")
}

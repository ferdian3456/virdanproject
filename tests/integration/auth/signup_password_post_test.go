package auth

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestSignupPasswordPost_Success(t *testing.T) {
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

	testEmail := "password@example.com"

	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	_, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "OTP verification should succeed")

	t.Log("=== Testing Password Setting with Valid Password ===")
	validPassword := "ValidPass123"
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":"%s"}`, sessionId, validPassword))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "set password should succeed")
	setup.RequireStatus(t, resp, 200)

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "accessToken", "response should contain access token")
	require.Contains(t, result, "refreshToken", "response should contain refresh token")

	accessToken := result["accessToken"].(string)
	require.NotEmpty(t, accessToken, "access token should not be empty")

	t.Logf("Signup completed successfully, token received")
}

func TestSignupPasswordPost_EmptyPassword(t *testing.T) {
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

	testEmail := "emptypass@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	_, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "OTP verification should succeed")

	t.Log("=== Testing Password Setting with Empty Password ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":""}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "set password request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "required", "error message should mention password is required")
}

func TestSignupPasswordPost_TooShort(t *testing.T) {
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

	testEmail := "shortpass@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	_, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "OTP verification should succeed")

	t.Log("=== Testing Password Setting with Less Than 5 Characters ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":"1234"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "set password request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "at least 5 characters", "error message should mention minimum length")
}

func TestSignupPasswordPost_TooLong(t *testing.T) {
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

	testEmail := "longpass@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	_, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "OTP verification should succeed")

	t.Log("=== Testing Password Setting with More Than 20 Characters ===")
	longPassword := "thispasswordiswaytoolongtobevalid123"
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":"%s"}`, sessionId, longPassword))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "set password request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "at most 20 characters", "error message should mention maximum length")
}

func TestSignupPasswordPost_WrongStep(t *testing.T) {
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

	testEmail := "wrongstep@example.com"
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	t.Log("=== Testing Password Setting Before OTP Verification ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":"password123"}`, sessionId))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "set password request should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "response should contain error field")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "Invalid signup step", "error message should mention wrong signup step")

	t.Logf("Correctly rejected password before OTP verification")
}

func TestSignupPasswordPost_CreatesUserAndReturnsTokens(t *testing.T) {
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

	testEmail := "fullsignup@example.com"
	testPassword := "FullPass123"

	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, testEmail))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "signup start should succeed")

	result := setup.ParseJSONResponse(t, resp)
	sessionId := result["sessionId"].(string)

	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, testEmail)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	_, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "OTP verification should succeed")

	t.Log("=== Testing Complete Signup Flow ===")
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":"%s"}`, sessionId, testPassword))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "set password should succeed")
	setup.RequireStatus(t, resp, 200)

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "accessToken", "response should contain access token")
	require.Contains(t, result, "refreshToken", "response should contain refresh token")
	require.Contains(t, result, "tokenType", "response should contain token type")

	accessToken := result["accessToken"].(string)
	refreshToken := result["refreshToken"].(string)
	tokenType := result["tokenType"].(string)

	require.NotEmpty(t, accessToken, "access token should not be empty")
	require.NotEmpty(t, refreshToken, "refresh token should not be empty")
	require.Equal(t, "Bearer", tokenType, "token type should be Bearer")

	t.Logf("Signup completed successfully")
	t.Logf("Access Token: %s...", accessToken[:20])
	t.Logf("Token Type: %s", tokenType)

	t.Log("=== Verifying User Can Login ===")
	reqBody = []byte(fmt.Sprintf(`{"email":"%s","password":"%s"}`, testEmail, testPassword))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/login", reqBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "login should succeed")
	setup.RequireStatus(t, resp, 200)

	t.Logf("User successfully logged in after signup")
}

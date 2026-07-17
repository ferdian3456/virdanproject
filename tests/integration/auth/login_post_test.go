package auth

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestLogin_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestLogin_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	email := "login@example.com"
	password := "password123"

	_ = setup.CreateTestUser(t, app, infra.MailhogURL, email, password)

	setup.LogTestStep(t, "Testing Login with valid email/password")
	reqBody := []byte(fmt.Sprintf(`{"email":"%s","password":"%s"}`, email, password))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/login", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "login request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "accessToken", "response should contain accessToken")
	require.Contains(t, result, "refreshToken", "response should contain refreshToken")
	require.Contains(t, result, "tokenType", "response should contain tokenType")
	require.Equal(t, "Bearer", result["tokenType"], "tokenType should be Bearer")

	setup.LogTestPass(t, "TestLogin_Success")
}

func TestLogin_WrongPassword(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestLogin_WrongPassword")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	email := "loginwrong@example.com"
	_ = setup.CreateTestUser(t, app, infra.MailhogURL, email, "password123")

	reqBody := []byte(fmt.Sprintf(`{"email":"%s","password":"wrong-password"}`, email))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/login", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "login request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "wrong password should produce an error")
	setup.LogTestPass(t, "TestLogin_WrongPassword")
}

func TestLogin_UnknownEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestLogin_UnknownEmail")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	reqBody := []byte(`{"email":"nobody@example.com","password":"password123"}`)
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/login", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "login request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "unknown email should produce an error")
	setup.LogTestPass(t, "TestLogin_UnknownEmail")
}

func TestLogin_ValidationErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestLogin_ValidationErrors")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	cases := []struct {
		name string
		body string
	}{
		{name: "EmptyEmail", body: `{"email":"","password":"password123"}`},
		{name: "InvalidEmail", body: `{"email":"not-an-email","password":"password123"}`},
		{name: "EmptyPassword", body: `{"email":"user@example.com","password":""}`},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/login", []byte(tc.body))
			resp, err := setup.AppTest(t, app, req)
			require.NoError(t, err, "login request should complete")

			result := setup.ParseJSONResponse(t, resp)
			require.Contains(t, result, "error", "%s should fail validation", tc.name)
		})
	}
	setup.LogTestPass(t, "TestLogin_ValidationErrors")
}

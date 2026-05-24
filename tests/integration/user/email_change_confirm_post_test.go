package user

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestEmailChangeConfirm_Success completes the email change end-to-end:
// request → fetch OTP from MailHog (delivered to the OLD address as per the
// usecase) → confirm. The user then logs in with the new email to validate
// the swap landed in Postgres.
func TestEmailChangeConfirm_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestEmailChangeConfirm_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	oldEmail := "emailconfirm-old@example.com"
	newEmail := "emailconfirm-new@example.com"
	token := setup.CreateTestUser(t, app, infra.MailhogURL, oldEmail, "password123")

	// 1. Request the change.
	reqBody := []byte(fmt.Sprintf(`{"newEmail":"%s"}`, newEmail))
	req := setup.CreateAuthRequest(http.MethodPost, "/api/users/email/change/request", reqBody, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request email change should succeed")
	setup.RequireStatus(t, resp, 200)

	// 2. OTP is sent to the CURRENT email (oldEmail) for security.
	otp := setup.GetOTPFromMailhog(t, infra.MailhogURL, oldEmail)
	require.NotEmpty(t, otp, "expected an OTP delivered to the current email")

	// 3. Confirm.
	reqBody = []byte(fmt.Sprintf(`{"otp":"%s"}`, otp))
	req = setup.CreateAuthRequest(http.MethodPost, "/api/users/email/change/confirm", reqBody, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "confirm email change should succeed")
	setup.RequireStatus(t, resp, 200)

	// 4. Logging in with the new email should now work.
	loginBody := []byte(fmt.Sprintf(`{"email":"%s","password":"password123"}`, newEmail))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/login", loginBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "login with new email should complete")
	setup.RequireStatus(t, resp, 200)

	setup.LogTestPass(t, "TestEmailChangeConfirm_Success")
}

// TestEmailChangeConfirm_WrongOTP rejects an invalid OTP and increments the
// attempt counter (we don't assert the counter here, just the error path).
func TestEmailChangeConfirm_WrongOTP(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestEmailChangeConfirm_WrongOTP")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "emailconfirmbad@example.com", "password123")

	// Open a pending session so the confirm endpoint has a session to inspect.
	reqBody := []byte(`{"newEmail":"emailconfirmbad-new@example.com"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/users/email/change/request", reqBody, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request email change should succeed")
	setup.RequireStatus(t, resp, 200)

	reqBody = []byte(`{"otp":"000000"}`)
	req = setup.CreateAuthRequest(http.MethodPost, "/api/users/email/change/confirm", reqBody, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "confirm email change request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "wrong OTP should produce an error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "Invalid code", "error message should call out the invalid OTP")
	setup.LogTestPass(t, "TestEmailChangeConfirm_WrongOTP")
}

// TestEmailChangeConfirm_NoPendingSession rejects a confirm call when there is
// no in-flight email change.
func TestEmailChangeConfirm_NoPendingSession(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestEmailChangeConfirm_NoPendingSession")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "emailconfirmnone@example.com", "password123")

	reqBody := []byte(`{"otp":"123456"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/users/email/change/confirm", reqBody, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "confirm email change request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "missing pending session should produce an error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "No pending email change", "error message should explain there is no pending session")
	setup.LogTestPass(t, "TestEmailChangeConfirm_NoPendingSession")
}

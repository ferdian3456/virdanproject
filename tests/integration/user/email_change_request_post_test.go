package user

import (
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestEmailChangeRequest_Success kicks off the email change flow: the API
// sends an OTP to the current email and stashes a pending session in Redis.
func TestEmailChangeRequest_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestEmailChangeRequest_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "emailchg@example.com", "password123")

	reqBody := []byte(`{"newEmail":"emailchg-new@example.com"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/users/email/change/request", reqBody, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request email change should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "otpExpiresAt", "response should expose otpExpiresAt")
	setup.LogTestPass(t, "TestEmailChangeRequest_Success")
}

// TestEmailChangeRequest_SameAsCurrent rejects a request that reuses the
// caller's current email.
func TestEmailChangeRequest_SameAsCurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestEmailChangeRequest_SameAsCurrent")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	email := "emailchgsame@example.com"
	token := setup.CreateTestUser(t, app, infra.MailhogURL, email, "password123")

	reqBody := []byte(`{"newEmail":"` + email + `"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/users/email/change/request", reqBody, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request email change should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "same-email request should fail")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "differ", "error message should explain the new email must differ")
	setup.LogTestPass(t, "TestEmailChangeRequest_SameAsCurrent")
}

// TestEmailChangeRequest_AlreadyTaken rejects a new email that maps to an
// existing account.
func TestEmailChangeRequest_AlreadyTaken(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestEmailChangeRequest_AlreadyTaken")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	// First user owns the target email already.
	_ = setup.CreateTestUser(t, app, infra.MailhogURL, "emailtaken-target@example.com", "password123")
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "emailtaken-source@example.com", "password123")

	reqBody := []byte(`{"newEmail":"emailtaken-target@example.com"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/users/email/change/request", reqBody, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request email change should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "taken email should fail")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "already registered", "error message should mention email already registered")
	setup.LogTestPass(t, "TestEmailChangeRequest_AlreadyTaken")
}

// TestEmailChangeRequest_Unauthorized rejects unauthenticated callers.
func TestEmailChangeRequest_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestEmailChangeRequest_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	reqBody := []byte(`{"newEmail":"whoever@example.com"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/users/email/change/request", reqBody, "")
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "request email change should complete")
	require.NotEqual(t, 200, resp.StatusCode, "unauthenticated request must not succeed")
	setup.LogTestPass(t, "TestEmailChangeRequest_Unauthorized")
}

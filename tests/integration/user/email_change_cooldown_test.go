package user

import (
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestEmailChangeRequest_Cooldown exercises the rate-limit branch in
// RequestEmailChange. The usecase refuses a second request while more than
// (TTL - cooldown) of the previous OTP is still alive — i.e. when less than
// 60s have passed since the last request. Firing two requests back to back
// should trigger that branch and surface the "Please wait Ns" message.
func TestEmailChangeRequest_Cooldown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestEmailChangeRequest_Cooldown")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "ecreqcd@example.com", "password123")

	// First request opens the pending email-change session.
	reqBody := []byte(`{"newEmail":"ecreqcd-new1@example.com"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/users/email/change/request", reqBody, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "first email change request should succeed")
	setup.RequireStatus(t, resp, 200)

	// Second request fired immediately should hit the cooldown branch.
	reqBody = []byte(`{"newEmail":"ecreqcd-new2@example.com"}`)
	req = setup.CreateAuthRequest(http.MethodPost, "/api/users/email/change/request", reqBody, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "second email change request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "second request should be rejected")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "Please wait", "error message should mention the cooldown wait")
	setup.LogTestPass(t, "TestEmailChangeRequest_Cooldown")
}

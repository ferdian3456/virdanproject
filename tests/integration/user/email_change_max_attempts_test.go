package user

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestEmailChangeConfirm_MaxAttempts exercises the "too many attempts" branch
// in ConfirmEmailChange. The usecase increments the attempt counter on every
// mismatch and then checks `attempts >= emailChangeMaxTries` BEFORE the OTP
// comparison on the next call. With emailChangeMaxTries=5 that means the
// limit fires on the 6th attempt (after 5 increments). The 6th call wipes the
// session, so a 7th call should report no pending session.
func TestEmailChangeConfirm_MaxAttempts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestEmailChangeConfirm_MaxAttempts")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "ecmax@example.com", "password123")

	// Open a pending session.
	reqBody := []byte(`{"newEmail":"ecmax-new@example.com"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/users/email/change/request", reqBody, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "open email-change session should succeed")
	setup.RequireStatus(t, resp, 200)

	// Burn through 5 wrong OTPs. Each must report "Invalid code" and bump the
	// attempts counter from 0 → 5.
	for i := 1; i <= 5; i++ {
		wrong := fmt.Sprintf("00000%d", i)
		reqBody = []byte(fmt.Sprintf(`{"otp":"%s"}`, wrong))
		req = setup.CreateAuthRequest(http.MethodPost, "/api/users/email/change/confirm", reqBody, token)
		resp, err = setup.AppTest(t, app, req)
		require.NoError(t, err, "wrong OTP confirm attempt %d should complete", i)

		result := setup.ParseJSONResponse(t, resp)
		require.Contains(t, result, "error", "attempt %d should produce an error", i)
		errMsg := setup.ParseErrorMessage(t, result)
		require.Contains(t, errMsg, "Invalid code", "attempt %d should hit the OTP-mismatch branch", i)
	}

	// 6th attempt: attempts=5 ≥ emailChangeMaxTries → limit branch fires and
	// the pending session is wiped.
	reqBody = []byte(`{"otp":"000006"}`)
	req = setup.CreateAuthRequest(http.MethodPost, "/api/users/email/change/confirm", reqBody, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "6th confirm attempt should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "6th attempt should produce an error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "Too many attempts", "6th attempt should hit the max-tries branch")

	// 7th attempt: session was cleared, so the no-pending branch fires.
	reqBody = []byte(`{"otp":"000007"}`)
	req = setup.CreateAuthRequest(http.MethodPost, "/api/users/email/change/confirm", reqBody, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "post-cap confirm should complete")

	result = setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "post-cap attempt should produce an error")
	errMsg = setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "No pending email change", "session must be cleared after max attempts")
	setup.LogTestPass(t, "TestEmailChangeConfirm_MaxAttempts")
}

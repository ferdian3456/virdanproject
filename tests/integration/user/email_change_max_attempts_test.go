package user

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

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

	reqBody := []byte(`{"newEmail":"ecmax-new@example.com"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/users/email/change/request", reqBody, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "open email-change session should succeed")
	setup.RequireStatus(t, resp, 200)

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

	reqBody = []byte(`{"otp":"000006"}`)
	req = setup.CreateAuthRequest(http.MethodPost, "/api/users/email/change/confirm", reqBody, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "6th confirm attempt should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "6th attempt should produce an error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "Too many attempts", "6th attempt should hit the max-tries branch")

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

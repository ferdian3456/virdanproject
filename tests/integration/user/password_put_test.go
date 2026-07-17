package user

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestPasswordPut_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestPasswordPut_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	email := "pwchange@example.com"
	token := setup.CreateTestUser(t, app, infra.MailhogURL, email, "password123")

	reqBody := []byte(`{"currentPassword":"password123","newPassword":"NewPassword123"}`)
	req := setup.CreateAuthRequest(http.MethodPut, "/api/users/password", reqBody, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "change password should succeed")
	setup.RequireStatus(t, resp, 200)

	loginBody := []byte(fmt.Sprintf(`{"email":"%s","password":"NewPassword123"}`, email))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/login", loginBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "login with new password should complete")
	setup.RequireStatus(t, resp, 200)
	setup.LogTestPass(t, "TestPasswordPut_Success")
}

func TestPasswordPut_WrongCurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestPasswordPut_WrongCurrent")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "pwchangebad@example.com", "password123")

	reqBody := []byte(`{"currentPassword":"WRONG","newPassword":"NewPassword123"}`)
	req := setup.CreateAuthRequest(http.MethodPut, "/api/users/password", reqBody, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "change password request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "wrong current password should produce an error")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "Current password is incorrect", "error should call out the bad current password")
	setup.LogTestPass(t, "TestPasswordPut_WrongCurrent")
}

func TestPasswordPut_NewEqualsCurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestPasswordPut_NewEqualsCurrent")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "pwsame@example.com", "password123")

	reqBody := []byte(`{"currentPassword":"password123","newPassword":"password123"}`)
	req := setup.CreateAuthRequest(http.MethodPut, "/api/users/password", reqBody, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "change password request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "same-password change should produce an error")
	setup.LogTestPass(t, "TestPasswordPut_NewEqualsCurrent")
}

func TestPasswordPut_ValidationErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestPasswordPut_ValidationErrors")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "pwvalid@example.com", "password123")

	cases := []struct {
		name string
		body string
	}{
		{name: "EmptyCurrent", body: `{"currentPassword":"","newPassword":"NewPassword123"}`},
		{name: "ShortNew", body: `{"currentPassword":"password123","newPassword":"short"}`},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := setup.CreateAuthRequest(http.MethodPut, "/api/users/password", []byte(tc.body), token)
			resp, err := setup.AppTest(t, app, req)
			require.NoError(t, err, "change password request should complete")

			result := setup.ParseJSONResponse(t, resp)
			require.Contains(t, result, "error", "%s should fail validation", tc.name)
		})
	}
	setup.LogTestPass(t, "TestPasswordPut_ValidationErrors")
}

package notification

import (
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestRegisterDevice_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestRegisterDevice_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "device-register@example.com", "password123")

	body := []byte(`{"token":"fcm-token-abc123","platform":"android"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/devices/", body, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "register device request should complete")
	setup.RequireStatus(t, resp, 200)
	setup.LogTestPass(t, "TestRegisterDevice_Success")
}

func TestRegisterDevice_InvalidPlatform(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestRegisterDevice_InvalidPlatform")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "device-badplatform@example.com", "password123")

	body := []byte(`{"token":"fcm-token-xyz","platform":"web"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/devices/", body, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "register device request should complete")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "unsupported platform must be rejected")
	setup.LogTestPass(t, "TestRegisterDevice_InvalidPlatform")
}

func TestRegisterDevice_MissingToken(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestRegisterDevice_MissingToken")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "device-missingtoken@example.com", "password123")

	body := []byte(`{"token":"","platform":"android"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/devices/", body, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "register device request should complete")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "empty token must be rejected")
	setup.LogTestPass(t, "TestRegisterDevice_MissingToken")
}

func TestRegisterDevice_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestRegisterDevice_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	body := []byte(`{"token":"fcm-token-noauth","platform":"android"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/devices/", body, "")
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "register device request should complete")
	require.NotEqual(t, 200, resp.StatusCode, "unauthenticated register must not succeed")
	setup.LogTestPass(t, "TestRegisterDevice_Unauthorized")
}

func TestUnregisterDevice_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUnregisterDevice_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "device-unregister@example.com", "password123")

	registerBody := []byte(`{"token":"fcm-token-unreg","platform":"ios"}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/devices/", registerBody, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "register device should succeed")
	setup.RequireStatus(t, resp, 200)

	unregisterBody := []byte(`{"token":"fcm-token-unreg"}`)
	req = setup.CreateAuthRequest(http.MethodDelete, "/api/devices/", unregisterBody, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "unregister device request should complete")
	setup.RequireStatus(t, resp, 200)
	setup.LogTestPass(t, "TestUnregisterDevice_Success")
}

func TestUnregisterDevice_NonExistentToken(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUnregisterDevice_NonExistentToken")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "device-unreg-none@example.com", "password123")

	unregisterBody := []byte(`{"token":"never-registered-token"}`)
	req := setup.CreateAuthRequest(http.MethodDelete, "/api/devices/", unregisterBody, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "unregister device request should complete")
	setup.RequireStatus(t, resp, 200)
	setup.LogTestPass(t, "TestUnregisterDevice_NonExistentToken")
}

func TestUnregisterDevice_MissingToken(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestUnregisterDevice_MissingToken")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "device-unreg-missing@example.com", "password123")

	unregisterBody := []byte(`{"token":""}`)
	req := setup.CreateAuthRequest(http.MethodDelete, "/api/devices/", unregisterBody, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "unregister device request should complete")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode, "empty token must be rejected")
	setup.LogTestPass(t, "TestUnregisterDevice_MissingToken")
}

// TestSend only touches the FCM client when a device is registered; since the
// test app wires a nil FCM client, we only exercise the safe "no device
// registered" path here, which never reaches the FCM SDK.
func TestTestSend_NoDeviceRegistered(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestTestSend_NoDeviceRegistered")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "testsend-nodevice@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodPost, "/api/notifications/test-send", nil, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "test-send request should complete")
	require.Equal(t, http.StatusNotFound, resp.StatusCode, "test-send without a registered device must 404")
	setup.LogTestPass(t, "TestTestSend_NoDeviceRegistered")
}

func TestTestSend_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestTestSend_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	req := setup.CreateAuthRequest(http.MethodPost, "/api/notifications/test-send", nil, "")
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "test-send request should complete")
	require.NotEqual(t, 200, resp.StatusCode, "unauthenticated test-send must not succeed")
	setup.LogTestPass(t, "TestTestSend_Unauthorized")
}

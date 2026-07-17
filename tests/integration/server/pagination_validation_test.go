package server

import (
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestCategoriesGet_LimitNegative(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestCategoriesGet_LimitNegative")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "neglimit@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/categories?limit=-1", nil, token)
	result := setup.RequireJSONWithLog(t, app, req, 200)
	require.Contains(t, result, "data", "response should contain data array")
	setup.LogTestPass(t, "TestCategoriesGet_LimitNegative")
}

func TestCategoriesGet_LimitExceeded(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestCategoriesGet_LimitExceeded")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "maxlimit@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/categories?limit=101", nil, token)
	result := setup.RequireJSONWithLog(t, app, req, 200)
	require.Contains(t, result, "data", "response should contain data array")
	setup.LogTestPass(t, "TestCategoriesGet_LimitExceeded")
}

func TestMeGet_LimitNegative(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestMeGet_LimitNegative")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "neglimitme@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/me?limit=-1", nil, token)
	result := setup.RequireJSONWithLog(t, app, req, 200)
	require.Contains(t, result, "data", "response should contain data array")
	setup.LogTestPass(t, "TestMeGet_LimitNegative")
}

func TestMeGet_LimitExceeded(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestMeGet_LimitExceeded")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	token := setup.CreateTestUser(t, app, setup.GetGlobalInfra().MailhogURL, "maxlimitme@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodGet, "/api/servers/me?limit=51", nil, token)
	result := setup.RequireJSONWithLog(t, app, req, 200)
	require.Contains(t, result, "data", "response should contain data array")
	setup.LogTestPass(t, "TestMeGet_LimitExceeded")
}

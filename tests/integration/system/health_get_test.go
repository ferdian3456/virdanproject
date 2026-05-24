package system

import (
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestHealthGet_Success verifies the public /api/health endpoint reports
// Postgres, Redis, and MinIO as up when the singleton infrastructure is
// healthy. The endpoint requires no authentication.
func TestHealthGet_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestHealthGet_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	req := setup.CreateJSONRequest(http.MethodGet, "/api/health", nil)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "health request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Equal(t, "ok", result["status"], "overall status should be ok")

	checks, ok := result["checks"].(map[string]interface{})
	require.True(t, ok, "checks should be an object")
	require.Equal(t, "up", checks["postgres"], "postgres should be up")
	require.Equal(t, "up", checks["redis"], "redis should be up")
	require.Equal(t, "up", checks["minio"], "minio should be up")
	setup.LogTestPass(t, "TestHealthGet_Success")
}

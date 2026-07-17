package auth

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

func TestRefresh_ChainRotation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestRefresh_ChainRotation")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	_, refresh1 := loginAndReturnTokens(t, app, infra.MailhogURL, "refreshchain@example.com", "password123")
	require.NotEmpty(t, refresh1, "initial refresh token must not be empty")

	reqBody := []byte(fmt.Sprintf(`{"refreshToken":%q}`, refresh1))
	req := setup.CreateJSONRequest(http.MethodPost, "/api/auth/refresh", reqBody)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "first rotation should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	refresh2, ok := result["refreshToken"].(string)
	require.True(t, ok, "first rotation should return a string refreshToken")
	require.NotEqual(t, refresh1, refresh2, "refresh token must be rotated")

	reqBody = []byte(fmt.Sprintf(`{"refreshToken":%q}`, refresh2))
	req = setup.CreateJSONRequest(http.MethodPost, "/api/auth/refresh", reqBody)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "second rotation should succeed")
	setup.RequireStatus(t, resp, 200)

	result = setup.ParseJSONResponse(t, resp)
	refresh3, ok := result["refreshToken"].(string)
	require.True(t, ok, "second rotation should return a string refreshToken")
	require.NotEqual(t, refresh2, refresh3, "third refresh token must differ from the second")
	require.NotEqual(t, refresh1, refresh3, "third refresh token must differ from the first")
	setup.LogTestPass(t, "TestRefresh_ChainRotation")
}

package plus

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestListMyOrders_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestListMyOrders_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "listorders@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "List Orders Server", "listorders", 1, false)
	userID := setup.GetUserId(t, app, token)

	orderID := uuid.New().String()
	referenceID := "virdan-plus-" + orderID
	now := time.Now().UTC()
	expires := now.AddDate(0, 0, 30)
	_, execErr := db.Exec(t.Context(),
		`INSERT INTO server_plus_orders
		 (id, server_id, user_id, reference_id, xendit_payment_id, base_idr, tax_idr, total_idr, status, paid_at, plus_expires_at, created_at, updated_at, created_by, updated_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'PAID',$9,$10,$9,$9,$3,$3)`,
		orderID, serverID, userID, referenceID, "py-"+setup.GenerateRandomString(8), 50000, 5500, 55500, now, expires)
	require.NoError(t, execErr, "insert paid order")

	req := setup.CreateAuthRequest(http.MethodGet, "/api/me/plus-orders", nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	data, ok := result["data"].([]interface{})
	require.True(t, ok, "response should contain data array, got: %v", result)
	require.Len(t, data, 1, "should list the one order created for this user")

	item, ok := data[0].(map[string]interface{})
	require.True(t, ok, "order item should be an object")
	require.Equal(t, orderID, item["id"])
	require.Equal(t, serverID, item["serverId"])
	require.Equal(t, "PAID", item["status"])
	require.EqualValues(t, 55500, item["totalIdr"])
	setup.LogTestPass(t, "TestListMyOrders_Success")
}

func TestListMyOrders_EmptyWhenNoOrders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestListMyOrders_EmptyWhenNoOrders")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "listordersempty@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodGet, "/api/me/plus-orders", nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	data, ok := result["data"].([]interface{})
	require.True(t, ok, "response should contain data array, got: %v", result)
	require.Len(t, data, 0, "user with no orders should get an empty list")
	setup.LogTestPass(t, "TestListMyOrders_EmptyWhenNoOrders")
}

func TestListMyOrders_OnlyOwnOrders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestListMyOrders_OnlyOwnOrders")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()

	ownerToken := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "listordersowner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, ownerToken, "List Orders Owner Server", "listordersown", 1, false)
	ownerID := setup.GetUserId(t, app, ownerToken)

	otherToken := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "listordersother@example.com", "password123")

	orderID := uuid.New().String()
	now := time.Now().UTC()
	_, execErr := db.Exec(t.Context(),
		`INSERT INTO server_plus_orders
		 (id, server_id, user_id, reference_id, base_idr, tax_idr, total_idr, status, created_at, updated_at, created_by, updated_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,'PENDING',$8,$8,$3,$3)`,
		orderID, serverID, ownerID, fmt.Sprintf("virdan-plus-%s", orderID), 50000, 5500, 55500, now)
	require.NoError(t, execErr)

	req := setup.CreateAuthRequest(http.MethodGet, "/api/me/plus-orders", nil, otherToken)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	data, ok := result["data"].([]interface{})
	require.True(t, ok, "response should contain data array, got: %v", result)
	require.Len(t, data, 0, "user must not see other users' orders")
	setup.LogTestPass(t, "TestListMyOrders_OnlyOwnOrders")
}

func TestListMyOrders_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestListMyOrders_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	req := setup.CreateAuthRequest(http.MethodGet, "/api/me/plus-orders", nil, "")
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	require.NotEqual(t, 200, resp.StatusCode, "unauthenticated list must not succeed")
	setup.LogTestPass(t, "TestListMyOrders_Unauthorized")
}

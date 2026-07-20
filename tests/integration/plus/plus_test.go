package plus

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const testWebhookToken = "test_webhook_token_12345"

func TestPlusStatus_DefaultFree(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestPlusStatus_DefaultFree")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "plusfree@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Plus Free", "plusfree", 1, false)

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/plus", serverID), nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Equal(t, false, result["active"], "new server should not be plus")
	require.EqualValues(t, 30, result["durationDays"])
	price := result["price"].(map[string]interface{})
	require.EqualValues(t, 50000, price["baseIdr"])
	require.EqualValues(t, 5500, price["taxIdr"])
	require.EqualValues(t, 55500, price["totalIdr"])
	setup.LogTestPass(t, "TestPlusStatus_DefaultFree")
}

func TestPlusStatus_NotAMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestPlusStatus_NotAMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()

	owner := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "plusowner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, owner, "Plus Guard", "plusguard", 1, false)
	outsider := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "plusoutsider@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/plus", serverID), nil, outsider)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 403)
	setup.LogTestPass(t, "TestPlusStatus_NotAMember")
}

func TestWebhook_InvalidToken(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestWebhook_InvalidToken")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	payload := map[string]interface{}{
		"event": "payment.capture",
		"data":  map[string]interface{}{"payment_id": "py-" + setup.GenerateRandomString(8), "status": "SUCCEEDED", "reference_id": "virdan-plus-x"},
	}
	body, _ := json.Marshal(payload)
	req := setup.CreateJSONRequest(http.MethodPost, "/api/webhooks/xendit", body)
	req.Header.Set("x-callback-token", "wrong_token")

	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 401)
	setup.LogTestPass(t, "TestWebhook_InvalidToken")
}

func TestWebhook_GrantsPlus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestWebhook_GrantsPlus")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "plusgrant@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Plus Grant", "plusgrant", 1, false)
	userID := setup.GetUserId(t, app, token)

	orderID := uuid.New().String()
	referenceID := "virdan-plus-" + orderID
	now := time.Now().UTC()
	_, execErr := db.Exec(t.Context(),
		`INSERT INTO server_plus_orders
		 (id, server_id, user_id, reference_id, base_idr, tax_idr, total_idr, status, created_at, updated_at, created_by, updated_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,'PENDING',$8,$8,$3,$3)`,
		orderID, serverID, userID, referenceID, 50000, 5500, 55500, now)
	require.NoError(t, execErr, "insert pending order")

	paymentID := "py-" + setup.GenerateRandomString(12)
	payload := map[string]interface{}{
		"event": "payment.capture",
		"data": map[string]interface{}{
			"payment_id":   paymentID,
			"status":       "SUCCEEDED",
			"reference_id": referenceID,
		},
	}
	body, _ := json.Marshal(payload)
	req := setup.CreateJSONRequest(http.MethodPost, "/api/webhooks/xendit", body)
	req.Header.Set("x-callback-token", testWebhookToken)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)

	var status string
	for i := 0; i < 50; i++ {
		row := db.QueryRow(t.Context(), `SELECT status FROM server_plus_orders WHERE id = $1`, orderID)
		require.NoError(t, row.Scan(&status))
		if status == "PAID" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.Equal(t, "PAID", status, "order should be PAID after webhook")

	statusReq := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/plus", serverID), nil, token)
	statusResp, err := setup.TestRequestWithLogging(t, app, statusReq)
	require.NoError(t, err)
	setup.RequireStatus(t, statusResp, 200)
	statusResult := setup.ParseJSONResponse(t, statusResp)
	require.Equal(t, true, statusResult["active"], "server should be plus active after grant")
	setup.LogTestPass(t, "TestWebhook_GrantsPlus")
}

func TestWebhook_Idempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestWebhook_Idempotent")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "plusidem@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Plus Idem", "plusidem", 1, false)
	userID := setup.GetUserId(t, app, token)

	orderID := uuid.New().String()
	referenceID := "virdan-plus-" + orderID
	now := time.Now().UTC()
	_, execErr := db.Exec(t.Context(),
		`INSERT INTO server_plus_orders
		 (id, server_id, user_id, reference_id, base_idr, tax_idr, total_idr, status, created_at, updated_at, created_by, updated_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,'PENDING',$8,$8,$3,$3)`,
		orderID, serverID, userID, referenceID, 50000, 5500, 55500, now)
	require.NoError(t, execErr)

	paymentID := "py-" + setup.GenerateRandomString(12)
	payload := map[string]interface{}{
		"event": "payment.capture",
		"data":  map[string]interface{}{"payment_id": paymentID, "status": "SUCCEEDED", "reference_id": referenceID},
	}
	body, _ := json.Marshal(payload)

	send := func() {
		req := setup.CreateJSONRequest(http.MethodPost, "/api/webhooks/xendit", body)
		req.Header.Set("x-callback-token", testWebhookToken)
		resp, err := setup.TestRequestWithLogging(t, app, req)
		require.NoError(t, err)
		setup.RequireStatus(t, resp, 200)
	}
	send()
	time.Sleep(1 * time.Second)
	send()

	var count int
	eventID := "payment.capture:" + paymentID
	require.NoError(t, db.QueryRow(t.Context(), `SELECT COUNT(*) FROM xendit_webhook_events WHERE event_id = $1`, eventID).Scan(&count))
	require.Equal(t, 1, count, "duplicate webhook must not insert twice")
	setup.LogTestPass(t, "TestWebhook_Idempotent")
}

func TestCheckout_RejectWhenActive(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestCheckout_RejectWhenActive")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "plusactive@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Plus Active", "plusactive", 1, false)
	userID := setup.GetUserId(t, app, token)

	orderID := uuid.New().String()
	now := time.Now().UTC()
	expires := now.AddDate(0, 0, 30)
	_, execErr := db.Exec(t.Context(),
		`INSERT INTO server_plus_orders
		 (id, server_id, user_id, reference_id, xendit_payment_id, base_idr, tax_idr, total_idr, status, paid_at, plus_expires_at, created_at, updated_at, created_by, updated_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'PAID',$9,$10,$9,$9,$3,$3)`,
		orderID, serverID, userID, "virdan-plus-"+orderID, "py-"+setup.GenerateRandomString(8), 50000, 5500, 55500, now, expires)
	require.NoError(t, execErr)

	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/plus/checkout", serverID), nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 409)
	setup.LogTestPass(t, "TestCheckout_RejectWhenActive")
}

func TestOrderDetail_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestOrderDetail_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "plusdetail@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Plus Detail", "plusdetail", 1, false)
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
	require.NoError(t, execErr)

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/me/plus-orders/%s", orderID), nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Equal(t, orderID, result["id"])
	require.Equal(t, serverID, result["serverId"])
	require.Equal(t, referenceID, result["referenceId"])
	require.EqualValues(t, 50000, result["baseIdr"])
	require.EqualValues(t, 5500, result["taxIdr"])
	require.EqualValues(t, 55500, result["totalIdr"])
	require.Equal(t, "PAID", result["status"])
	setup.LogTestPass(t, "TestOrderDetail_Success")
}

func TestOrderDetail_NotOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestOrderDetail_NotOwner")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()

	owner := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "plusdetailowner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, owner, "Plus Detail Owner", "pldetown", 1, false)
	ownerID := setup.GetUserId(t, app, owner)
	outsider := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "plusdetailoutsider@example.com", "password123")

	orderID := uuid.New().String()
	now := time.Now().UTC()
	_, execErr := db.Exec(t.Context(),
		`INSERT INTO server_plus_orders
		 (id, server_id, user_id, reference_id, base_idr, tax_idr, total_idr, status, created_at, updated_at, created_by, updated_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,'PENDING',$8,$8,$3,$3)`,
		orderID, serverID, ownerID, "virdan-plus-"+orderID, 50000, 5500, 55500, now)
	require.NoError(t, execErr)

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/me/plus-orders/%s", orderID), nil, outsider)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 404)
	setup.LogTestPass(t, "TestOrderDetail_NotOwner")
}

func TestCreatePost_WithActivePlus_Succeeds(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()
	setup.LogTestStart(t, "TestCreatePost_WithActivePlus_Succeeds")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()
	globalInfra := setup.GetGlobalInfra()

	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "pluspost@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Plus Post", "pluspost", 1, false)
	userID := setup.GetUserId(t, app, token)

	orderID := uuid.New().String()
	now := time.Now().UTC()
	expires := now.AddDate(0, 0, 30)
	_, execErr := db.Exec(t.Context(),
		`INSERT INTO server_plus_orders
		 (id, server_id, user_id, reference_id, xendit_payment_id, base_idr, tax_idr, total_idr, status, paid_at, plus_expires_at, created_at, updated_at, created_by, updated_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'PAID',$9,$10,$9,$9,$3,$3)`,
		orderID, serverID, userID, "virdan-plus-"+orderID, "py-"+setup.GenerateRandomString(8), 50000, 5500, 55500, now, expires)
	require.NoError(t, execErr)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": "post with plus active",
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err)
	setup.RequireStatus(t, resp, 200)
	setup.LogTestPass(t, "TestCreatePost_WithActivePlus_Succeeds")
}

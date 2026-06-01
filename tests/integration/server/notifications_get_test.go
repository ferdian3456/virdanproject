package server

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestNotificationsFeed_Member: a member can read the per-server feed (empty on
// a fresh server) — verifies the nested route + member-guard pass.
func TestNotificationsFeed_Member(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestNotificationsFeed_Member")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "notif-mem@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, token, "Notif Member Server", "notifm", 1, false)

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/notifications", serverID), nil, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "feed request should complete")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "data", "feed should contain a data array")
	setup.LogTestPass(t, "TestNotificationsFeed_Member")
}

// TestNotificationsFeed_NotMember: a non-member is forbidden (no roster enumeration).
func TestNotificationsFeed_NotMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestNotificationsFeed_NotMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "notif-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Notif Guard Server", "notifg", 1, false)
	strangerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "notif-stranger@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/notifications", serverID), nil, strangerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "feed request should complete")
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "non-member must be forbidden")
	setup.LogTestPass(t, "TestNotificationsFeed_NotMember")
}

// TestNotificationsUnreadCount_Member: a member gets a count (0 on a fresh server).
func TestNotificationsUnreadCount_Member(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestNotificationsUnreadCount_Member")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "notif-cnt@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, token, "Notif Count Server", "notifc", 1, false)

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/notifications/unread-count", serverID), nil, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "unread-count request should complete")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "count", "response should contain count")
	require.EqualValues(t, 0, result["count"], "fresh server has no unread notifications")
	setup.LogTestPass(t, "TestNotificationsUnreadCount_Member")
}

// TestNotificationsUnreadCount_NotMember: non-member forbidden.
func TestNotificationsUnreadCount_NotMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestNotificationsUnreadCount_NotMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "notif-cnt-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Notif Count Guard", "notifcg", 1, false)
	strangerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "notif-cnt-stranger@example.com", "password123")

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/notifications/unread-count", serverID), nil, strangerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "unread-count request should complete")
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "non-member must be forbidden")
	setup.LogTestPass(t, "TestNotificationsUnreadCount_NotMember")
}

package server

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestJoinFromInvitePost_AvatarImageIdNotOwned exercises the avatarImageId
// branch in JoinServerFromInvite. Passing an avatarImageId that does not
// belong to the caller (here: a random unused UUID) must surface the
// "Avatar image is not owned by you" ForbiddenError.
func TestJoinFromInvitePost_AvatarImageIdNotOwned(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestJoinFromInvitePost_AvatarImageIdNotOwned")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "joinavatar-owner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, ownerToken, "Join Avatar Server", "joinavtr", 1, false)

	// Owner mints an invite.
	reqBody := []byte(`{"maxUses":5}`)
	req := setup.CreateAuthRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/invites", serverID), reqBody, ownerToken)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "create invite should succeed")
	setup.RequireStatus(t, resp, 200)
	inviteCode := setup.ParseJSONResponse(t, resp)["code"].(string)

	// Second user tries to join referencing an avatarImageId they do not own.
	joinerToken := setup.CreateTestUser(t, app, infra.MailhogURL, "joinavatar-joiner@example.com", "password123")
	reqBody = []byte(fmt.Sprintf(`{"inviteCode":"%s","nickname":"Joiner","username":"joiner","avatarImageId":"%s"}`,
		inviteCode, "11111111-1111-1111-1111-111111111111"))
	req = setup.CreateAuthRequest(http.MethodPost, "/api/servers/join", reqBody, joinerToken)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "join request should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "unowned avatarImageId should be rejected")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "not owned by you", "error should call out the ownership mismatch")
	setup.LogTestPass(t, "TestJoinFromInvitePost_AvatarImageIdNotOwned")
}

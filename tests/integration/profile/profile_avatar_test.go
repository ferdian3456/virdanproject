package profile

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestServerProfilePut_UploadFile covers the profileAvatar file branch of
// PUT /api/servers/:serverId/profile. The upload should land in MinIO,
// register a profile_avatar_images row, and surface a populated avatarUrl
// on the follow-up GET.
func TestServerProfilePut_UploadFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestServerProfilePut_UploadFile")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "pavtupload@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, token, "Profile Avatar Upload", "pavtup", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := buildProfileAvatarBody(t, imageData, map[string]string{
		"nickname": "AvatarUploader",
		"username": "avtuploader",
		"bio":      "Uploaded avatar via profile PUT",
	})
	req := setup.CreateAuthMultipartRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/profile", serverID), body, contentType, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "profile avatar upload should succeed")
	setup.RequireStatus(t, resp, 200)

	// Re-fetch profile and check avatarUrl + avatarImageId are set.
	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/profile/me", serverID), nil, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "follow-up GET should succeed")
	setup.RequireStatus(t, resp, 200)

	profile := setup.ParseJSONResponse(t, resp)
	require.NotEmpty(t, profile["avatarImageId"], "avatarImageId should be populated after upload")
	avatarURL, ok := profile["avatarUrl"].(string)
	require.True(t, ok, "avatarUrl should be a string after upload")
	require.Contains(t, avatarURL, "/profile/avatar/", "avatarUrl should target the profile/avatar bucket path")
	setup.LogTestPass(t, "TestServerProfilePut_UploadFile")
}

// TestServerProfilePut_ReuseAvatarImageId uploads an avatar to one server
// then reuses the resulting avatarImageId on a second server's profile.
// ResolveProfileAvatar verifies ownership against profile_avatar_images, so
// the reuse only works when the caller actually owns the row.
func TestServerProfilePut_ReuseAvatarImageId(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestServerProfilePut_ReuseAvatarImageId")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "pavtreuse@example.com", "password123")
	serverA := setup.CreateTestServer(t, app, infra.RedisURL, token, "Avatar Source", "pavtsrc", 1, false)
	serverB := setup.CreateTestServer(t, app, infra.RedisURL, token, "Avatar Reuse", "pavtdst", 1, false)

	// Upload an avatar through serverA.
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := buildProfileAvatarBody(t, imageData, map[string]string{
		"nickname": "ReuseOwner",
		"username": "reuseowner",
	})
	req := setup.CreateAuthMultipartRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/profile", serverA), body, contentType, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "upload on serverA should succeed")
	setup.RequireStatus(t, resp, 200)

	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/profile/me", serverA), nil, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "GET serverA profile should succeed")
	setup.RequireStatus(t, resp, 200)
	avatarImageId, ok := setup.ParseJSONResponse(t, resp)["avatarImageId"].(string)
	require.True(t, ok, "avatarImageId from serverA must be a string")
	require.NotEmpty(t, avatarImageId, "avatarImageId must be set after upload")

	// Reuse the same avatarImageId on serverB.
	body, contentType = buildProfileFieldsBody(t, map[string]string{
		"nickname":      "ReuseTarget",
		"username":      "reusetarget",
		"avatarImageId": avatarImageId,
	})
	req = setup.CreateAuthMultipartRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/profile", serverB), body, contentType, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "reuse on serverB should succeed")
	setup.RequireStatus(t, resp, 200)

	// Verify serverB profile now points at the same avatarImageId.
	req = setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/profile/me", serverB), nil, token)
	resp, err = setup.AppTest(t, app, req)
	require.NoError(t, err, "GET serverB profile should succeed")
	setup.RequireStatus(t, resp, 200)

	profileB := setup.ParseJSONResponse(t, resp)
	require.Equal(t, avatarImageId, profileB["avatarImageId"], "serverB should reuse the same avatarImageId")
	setup.LogTestPass(t, "TestServerProfilePut_ReuseAvatarImageId")
}

// TestServerProfilePut_AvatarFileAndIdConflict supplies BOTH profileAvatar
// and avatarImageId in the same request. ResolveProfileAvatar enforces
// mutual exclusion and must reject the call.
func TestServerProfilePut_AvatarFileAndIdConflict(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestServerProfilePut_AvatarFileAndIdConflict")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "pavtconflict@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, infra.RedisURL, token, "Conflict Avatar Server", "pavtcft", 1, false)

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := buildProfileAvatarBodyWithFields(t, imageData, map[string]string{
		"nickname":      "Conflict",
		"username":      "conflictusr",
		"avatarImageId": "22222222-2222-2222-2222-222222222222",
	})
	req := setup.CreateAuthMultipartRequest(http.MethodPut, fmt.Sprintf("/api/servers/%s/profile", serverID), body, contentType, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "conflicting profile update should complete")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "error", "supplying both profileAvatar and avatarImageId should fail")
	errMsg := setup.ParseErrorMessage(t, result)
	require.Contains(t, errMsg, "not both", "error message should call out the mutual exclusion")
	setup.LogTestPass(t, "TestServerProfilePut_AvatarFileAndIdConflict")
}

// buildProfileAvatarBody attaches a profileAvatar file plus text fields.
func buildProfileAvatarBody(t *testing.T, fileData []byte, fields map[string]string) (*bytes.Buffer, string) {
	return buildProfileAvatarBodyWithFields(t, fileData, fields)
}

// buildProfileAvatarBodyWithFields exists so the conflict test can attach
// both the file part and the avatarImageId field in one body.
func buildProfileAvatarBodyWithFields(t *testing.T, fileData []byte, fields map[string]string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="profileAvatar"; filename="avatar.webp"`)
	h.Set("Content-Type", "image/webp")
	part, err := writer.CreatePart(h)
	require.NoError(t, err, "create profileAvatar part")
	_, err = part.Write(fileData)
	require.NoError(t, err, "write profileAvatar data")

	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value), "write field %s", key)
	}
	require.NoError(t, writer.Close(), "close multipart writer")
	return body, writer.FormDataContentType()
}

// buildProfileFieldsBody builds a text-only multipart body for the profile
// PUT endpoint. The avatarImageId field doubles as the "reuse existing
// avatar" trigger.
func buildProfileFieldsBody(t *testing.T, fields map[string]string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value), "write field %s", key)
	}
	require.NoError(t, writer.Close(), "close multipart writer")
	return body, writer.FormDataContentType()
}

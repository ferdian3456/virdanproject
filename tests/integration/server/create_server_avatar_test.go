package server

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestCreateServer_WithAvatarFile drives the optional serverAvatar branch in
// CreateServer. The endpoint accepts multipart text fields plus an optional
// serverAvatar image; the response's server.avatarUrl pointer must be
// populated when a file was attached.
func TestCreateServer_WithAvatarFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestCreateServer_WithAvatarFile")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "createsrvavt@example.com", "password123")

	imageData := setup.CreateTestWebPImage(t)
	body, contentType := buildMultipartWithFile(t, "serverAvatar", "avatar.webp", imageData, map[string]string{
		"name":        "Server With Avatar",
		"shortName":   "savatar",
		"categoryId":  "1",
		"isPrivate":   "false",
		"nickname":    "OwnerSA",
		"username":    "ownersa",
		"description": "Server avatar branch test",
	})

	req := setup.CreateAuthMultipartRequest(http.MethodPost, "/api/servers/create", body, contentType, token)
	resp, err := setup.AppTest(t, app, req)
	require.NoError(t, err, "create server with avatar should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	server, ok := result["server"].(map[string]interface{})
	require.True(t, ok, "server field should be an object")
	require.NotEmpty(t, server["id"], "server.id should not be empty")

	avatarURL, ok := server["avatarUrl"].(string)
	require.True(t, ok, "avatarUrl should be populated when a serverAvatar file is attached")
	require.NotEmpty(t, avatarURL, "avatarUrl should not be empty")
	require.Contains(t, avatarURL, "/server/avatar/", "avatarUrl should target the server/avatar bucket path")
	setup.LogTestPass(t, "TestCreateServer_WithAvatarFile")
}

// buildMultipartWithFile is a local helper that builds a multipart body with
// one file part plus the supplied text fields. It mirrors
// setup.CreateMultipartFormData but does not impose a specific extension to
// content-type mapping (the test re-uses the webp test fixture).
func buildMultipartWithFile(t *testing.T, fieldName, fileName string, fileData []byte, fields map[string]string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="`+fieldName+`"; filename="`+fileName+`"`)
	h.Set("Content-Type", "image/webp")
	part, err := writer.CreatePart(h)
	require.NoError(t, err, "create multipart file part")
	_, err = part.Write(fileData)
	require.NoError(t, err, "write multipart file data")

	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value), "write field %s", key)
	}
	require.NoError(t, writer.Close(), "close multipart writer")
	return body, writer.FormDataContentType()
}

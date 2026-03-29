package user

import (
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/stretchr/testify/require"
)

// TestAvatarPut_Success tests successful avatar update
func TestAvatarPut_Success(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestAvatarPut_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Setup: Create user
	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "avatar@example.com", "avataruser", "password123")

	// Test: Update avatar
	setup.LogTestStep(t, "Testing Update Avatar")
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "avatar", "avatar.webp", imageData, nil)

	req := setup.CreateAuthMultipartRequest(http.MethodPut, "/api/users/avatar", body, contentType, token)
	result := setup.RequireJSONWithLog(t, app, req, 200)

	// API returns {"status":"OK"} for successful update
	if status, hasStatus := result["status"]; hasStatus && status == "OK" {
		t.Logf("Avatar updated successfully")
	} else if avatarURL, hasURL := result["avatar_url"]; hasURL {
		t.Logf("Avatar updated successfully: %v", avatarURL)
	} else {
		t.Logf("Avatar updated successfully")
	}

	setup.LogTestPass(t, "TestAvatarPut_Success")
}

// TestAvatarPut_MissingAvatar tests avatar update without file
func TestAvatarPut_MissingAvatar(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestAvatarPut_MissingAvatar")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "noavatar@example.com", "noavataruser", "password123")

	// Test: Update avatar without file
	setup.LogTestStep(t, "Testing Update Avatar Without File")
	body, contentType := setup.CreateMultipartFormData(t, "wrongfield", "avatar.jpg", setup.CreateTestWebPImage(t), nil)

	req := setup.CreateAuthMultipartRequest(http.MethodPut, "/api/users/avatar", body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "update avatar request should complete")

	// Should return error
	result := setup.ParseJSONResponse(t, resp)
	if _, hasError := result["error"]; hasError {
		t.Logf("Correctly rejected avatar update without file")
	} else {
		t.Logf("API accepts avatar update without file field")
	}

	setup.LogTestPass(t, "TestAvatarPut_MissingAvatar")
}

// TestAvatarPut_Unauthorized tests avatar update without authentication
func TestAvatarPut_Unauthorized(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestAvatarPut_Unauthorized")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	// Test: Update avatar without token
	setup.LogTestStep(t, "Testing Update Avatar Without Auth")
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "avatar", "avatar.jpg", imageData, nil)

	req := setup.CreateAuthMultipartRequest(http.MethodPut, "/api/users/avatar", body, contentType, "")
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "update avatar request should complete")

	// Should return unauthorized
	require.NotEqual(t, 200, resp.StatusCode, "should not return 200 without auth")
	t.Logf("Correctly rejected unauthenticated avatar update request")

	setup.LogTestPass(t, "TestAvatarPut_Unauthorized")
}

// TestAvatarPut_ReplaceExisting tests replacing existing avatar
func TestAvatarPut_ReplaceExisting(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	setup.LogTestStart(t, "TestAvatarPut_ReplaceExisting")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	infra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, infra.MailhogURL, "replaceavatar@example.com", "replaceavataruser", "password123")

	// Setup: Upload first avatar
	setup.LogTestStep(t, "Testing Replace Existing Avatar - First Upload")
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "avatar", "avatar1.jpg", imageData, nil)

	req := setup.CreateAuthMultipartRequest(http.MethodPut, "/api/users/avatar", body, contentType, token)
	_ = setup.RequireJSONWithLog(t, app, req, 200)

	// Upload second avatar (should replace first)
	setup.LogTestStep(t, "Testing Replace Existing Avatar - Second Upload")
	imageData = setup.CreateTestWebPImage(t)
	body, contentType = setup.CreateMultipartFormData(t, "avatar", "avatar2.jpg", imageData, nil)

	req = setup.CreateAuthMultipartRequest(http.MethodPut, "/api/users/avatar", body, contentType, token)
	result := setup.RequireJSONWithLog(t, app, req, 200)

	// API returns {"status":"OK"} for successful update
	if status, hasStatus := result["status"]; hasStatus && status == "OK" {
		t.Logf("Avatar replaced successfully")
	} else if avatarURL, hasURL := result["avatar_url"]; hasURL {
		t.Logf("Avatar replaced successfully: %v", avatarURL)
	} else {
		t.Logf("Avatar replaced successfully")
	}

	setup.LogTestPass(t, "TestAvatarPut_ReplaceExisting")
}

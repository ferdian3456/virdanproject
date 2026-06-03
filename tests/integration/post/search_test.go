package post

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// helper: create a post with a given caption, returns post id
func createPostWithCaption(t *testing.T, app *fiber.App, token, serverID, caption string) string {
	t.Helper()
	imageData := setup.CreateTestWebPImage(t)
	body, contentType := setup.CreateMultipartFormData(t, "image", "test.webp", imageData, map[string]string{
		"caption": caption,
	})
	req := setup.CreateAuthMultipartRequest("POST", fmt.Sprintf("/api/servers/%s/posts", serverID), body, contentType, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "create post should succeed")
	result := setup.ParseJSONResponse(t, resp)
	return result["id"].(string)
}

// TestSearchPosts_Success finds posts whose caption matches the query
func TestSearchPosts_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestSearchPosts_Success")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "searchpost@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Search Server", "search", 1, false)

	createPostWithCaption(t, app, token, serverID, "morning coffee run")
	createPostWithCaption(t, app, token, serverID, "late night code")

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/posts/search?q=coffee", serverID), nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "search request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	data, ok := result["data"].([]interface{})
	require.True(t, ok, "response should contain a data array")
	require.Len(t, data, 1, "only the coffee post should match")
	first := data[0].(map[string]interface{})
	require.Equal(t, "morning coffee run", first["caption"], "matched post caption")

	setup.LogTestPass(t, "TestSearchPosts_Success")
}

// TestSearchPosts_NoMatch returns an empty data array (not an error)
func TestSearchPosts_NoMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestSearchPosts_NoMatch")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "searchnomatch@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "NoMatch Server", "nomatch", 1, false)
	createPostWithCaption(t, app, token, serverID, "hello world")

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/posts/search?q=zzzznope", serverID), nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "search request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	data, ok := result["data"].([]interface{})
	require.True(t, ok, "response should contain a data array")
	require.Len(t, data, 0, "no posts should match")

	setup.LogTestPass(t, "TestSearchPosts_NoMatch")
}

// TestSearchPosts_QueryTooShort rejects queries shorter than 2 chars
func TestSearchPosts_QueryTooShort(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestSearchPosts_QueryTooShort")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "searchshort@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Short Server", "short", 1, false)

	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/posts/search?q=c", serverID), nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "request should complete")
	setup.RequireStatus(t, resp, 400)

	setup.LogTestPass(t, "TestSearchPosts_QueryTooShort")
}

// TestSearchPosts_NotAMember rejects a non-member requester
func TestSearchPosts_NotAMember(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestSearchPosts_NotAMember")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()
	ownerToken := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "searchowner@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, ownerToken, "Private Search", "psearch", 1, false)
	createPostWithCaption(t, app, ownerToken, serverID, "secret coffee")

	outsiderToken := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "searchoutsider@example.com", "password123")
	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/posts/search?q=coffee", serverID), nil, outsiderToken)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "request should complete")
	setup.RequireStatus(t, resp, 403)

	setup.LogTestPass(t, "TestSearchPosts_NotAMember")
}

// TestSearchPosts_EscapesWildcards treats ILIKE metacharacters in the query as
// literals: "a_c" must not match "abc"/"axc" via the underscore wildcard.
func TestSearchPosts_EscapesWildcards(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	t.Parallel()

	setup.LogTestStart(t, "TestSearchPosts_EscapesWildcards")
	app, db, _, _ := setup.SetupParallelTest(t)
	defer db.Close()

	globalInfra := setup.GetGlobalInfra()
	token := setup.CreateTestUser(t, app, globalInfra.MailhogURL, "searchwild@example.com", "password123")
	serverID := setup.CreateTestServer(t, app, globalInfra.RedisURL, token, "Wildcard Server", "wild", 1, false)

	createPostWithCaption(t, app, token, serverID, "abc")
	createPostWithCaption(t, app, token, serverID, "axc")

	// Underscore is an ILIKE wildcard; once escaped, "a_c" matches only a literal "a_c".
	req := setup.CreateAuthRequest(http.MethodGet, fmt.Sprintf("/api/servers/%s/posts/search?q=a_c", serverID), nil, token)
	resp, err := setup.TestRequestWithLogging(t, app, req)
	require.NoError(t, err, "search request should succeed")
	setup.RequireStatus(t, resp, 200)

	result := setup.ParseJSONResponse(t, resp)
	data, ok := result["data"].([]interface{})
	require.True(t, ok, "response should contain a data array")
	require.Len(t, data, 0, "underscore must be escaped, not treated as a wildcard")

	setup.LogTestPass(t, "TestSearchPosts_EscapesWildcards")
}

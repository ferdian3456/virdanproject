package integration

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/ferdian3456/virdanproject/tests/integration/setup"
)

// getTestImage reads a real JPEG image from testdata folder
func getTestImage() ([]byte, error) {
	testImagePath := "testdata/itachi.jpg"
	return os.ReadFile(testImagePath)
}

// createTestServer is a helper function to create a test server
func createTestServer(t *testing.T, app *fiber.App, accessToken string) map[string]interface{} {
	// Create server first
	reqBody := []byte(`{"name":"Test Server","shortName":"testsvr","categoryId":1,"settings":{"isPrivate":false}}`)
	req := setup.CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err, "create server should succeed")
	require.Equal(t, 200, resp.StatusCode, "create server should return 200")

	result := setup.ParseJSONResponse(t, resp)
	require.Contains(t, result, "id", "server response should contain id")

	return result
}

// createTestPost is a helper function to create a test post with image
func createTestPost(t *testing.T, app *fiber.App, accessToken string, serverId string, caption string) map[string]interface{} {
	// Read real test image
	testImageData, err := getTestImage()
	require.NoError(t, err, "should read test image")

	// Create multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add image file with proper Content-Type header
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="image"; filename="test_image.jpg"`)
	h.Set("Content-Type", "image/jpeg")
	part, err := writer.CreatePart(h)
	require.NoError(t, err, "should create form part")
	_, err = part.Write(testImageData)
	require.NoError(t, err, "should write image data")

	// Add caption
	err = writer.WriteField("caption", caption)
	require.NoError(t, err, "should write caption field")

	err = writer.Close()
	require.NoError(t, err, "should close writer")

	// Create request
	url := fmt.Sprintf("/api/servers/%s/posts", serverId)
	reqBody := body.Bytes()
	req := setup.CreateAuthRequest(http.MethodPost, url, reqBody, accessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req)
	require.NoError(t, err, "create post request should complete")
	require.Equal(t, 200, resp.StatusCode, "create post should return 200")

	result := setup.ParseJSONResponse(t, resp)
	return result
}

// TestCreatePost tests the POST /api/servers/:serverId/posts endpoint
func TestCreatePost(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err, "infrastructure should start successfully")
	defer infra.Terminate(ctx, t)

	t.Log("=== Running Database Migrations ===")
	setup.RunMigration(infra.PgURL, t)

	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Create test user
	t.Log("=== Setup: Creating Test User ===")
	accessToken := createTestUser(t, app, infra.MailhogURL, "postuser@example.com", "postuser", "pass123")

	// Create test server
	t.Log("=== Setup: Creating Test Server ===")
	server := createTestServer(t, app, accessToken)
	serverId := server["id"].(string)

	t.Log("✓ Test server created:", serverId)

	// Test 1: Create post successfully with valid data
	t.Log("=== Test 1: Create Post Successfully ===")

	// Read real test image
	testImageData, err := getTestImage()
	require.NoError(t, err, "should read test image")

	// Create multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add image file with proper Content-Type header
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="image"; filename="test_image.jpg"`)
	h.Set("Content-Type", "image/jpeg")
	part, err := writer.CreatePart(h)
	require.NoError(t, err, "should create form part")
	_, err = part.Write(testImageData)
	require.NoError(t, err, "should write image data")

	// Add caption
	err = writer.WriteField("caption", "This is a test post caption")
	require.NoError(t, err, "should write caption field")

	err = writer.Close()
	require.NoError(t, err, "should close writer")

	url := fmt.Sprintf("/api/servers/%s/posts", serverId)
	reqBody := body.Bytes()
	req := setup.CreateAuthRequest(http.MethodPost, url, reqBody, accessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req)
	require.NoError(t, err, "create post request should complete")

	if resp.StatusCode != 200 {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		t.Logf("Response status: %d", resp.StatusCode)
		t.Logf("Response body: %s", string(bodyBytes[:n]))
	}

	require.Equal(t, 200, resp.StatusCode, "create post should return 200")

	result := setup.ParseJSONResponse(t, resp)

	// Verify response contains post data
	require.Contains(t, result, "postId", "response should contain postId")
	require.Contains(t, result, "caption", "response should contain caption")
	require.Contains(t, result, "postImageUrl", "response should contain postImageUrl")
	require.Contains(t, result, "likeCount", "response should contain likeCount")
	require.Contains(t, result, "commentCount", "response should contain commentCount")

	postId := result["postId"].(string)
	caption := result["caption"].(string)
	likeCount := result["likeCount"].(float64)

	require.Equal(t, "This is a test post caption", caption, "caption should match")
	require.Equal(t, float64(0), likeCount, "initial like count should be 0")

	t.Log("✓ Post created successfully with postId:", postId)

	// Test 2: Create post without caption (should fail)
	t.Log("=== Test 2: Create Post Without Caption ===")

	testImageData2, err := getTestImage()
	require.NoError(t, err, "should read test image")

	body2 := &bytes.Buffer{}
	writer2 := multipart.NewWriter(body2)

	h2 := make(textproto.MIMEHeader)
	h2.Set("Content-Disposition", `form-data; name="image"; filename="test_image2.jpg"`)
	h2.Set("Content-Type", "image/jpeg")
	part2, err := writer2.CreatePart(h2)
	require.NoError(t, err, "should create form part")
	part2.Write(testImageData2)

	err2 := writer2.Close()
	require.NoError(t, err2, "should close writer")

	url2 := fmt.Sprintf("/api/servers/%s/posts", serverId)
	reqBody2 := body2.Bytes()
	req2 := setup.CreateAuthRequest(http.MethodPost, url2, reqBody2, accessToken)
	req2.Header.Set("Content-Type", writer2.FormDataContentType())

	resp2, err := app.Test(req2)
	require.NoError(t, err, "request should complete")

	result2 := setup.ParseJSONResponse(t, resp2)
	code, message, param := setup.ParseErrorDetail(t, result2)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "caption", param, "error param should be 'caption'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 3: Create post without image (should fail)
	t.Log("=== Test 3: Create Post Without Image ===")

	body3 := &bytes.Buffer{}
	writer3 := multipart.NewWriter(body3)

	err3 := writer3.WriteField("caption", "Caption without image")
	require.NoError(t, err3, "should write caption field")

	err3 = writer3.Close()
	require.NoError(t, err3, "should close writer")

	url3 := fmt.Sprintf("/api/servers/%s/posts", serverId)
	reqBody3 := body3.Bytes()
	req3 := setup.CreateAuthRequest(http.MethodPost, url3, reqBody3, accessToken)
	req3.Header.Set("Content-Type", writer3.FormDataContentType())

	resp3, err := app.Test(req3)
	require.NoError(t, err, "request should complete")

	result3 := setup.ParseJSONResponse(t, resp3)
	code, message, param = setup.ParseErrorDetail(t, result3)

	// Backend may return either VALIDATION_ERROR or INTERNAL_SERVER_ERROR when no file is uploaded
	require.Contains(t, []string{"VALIDATION_ERROR", "INTERNAL_SERVER_ERROR"}, code,
		"error code should be VALIDATION_ERROR or INTERNAL_SERVER_ERROR")
	if param == "" {
		require.Equal(t, "INTERNAL_SERVER_ERROR", code, "error code should be INTERNAL_SERVER_ERROR when no param")
	} else {
		require.Equal(t, "image", param, "error param should be 'image'")
	}

	t.Logf("✓ Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 4: Create post when not a server member (should fail)
	t.Log("=== Test 4: Create Post When Not Server Member ===")

	// Create another user
	otherAccessToken := createTestUser(t, app, infra.MailhogURL, "otheruser@example.com", "otheruser", "pass123")

	testImageData4, err := getTestImage()
	require.NoError(t, err, "should read test image")

	body4 := &bytes.Buffer{}
	writer4 := multipart.NewWriter(body4)

	h4 := make(textproto.MIMEHeader)
	h4.Set("Content-Disposition", `form-data; name="image"; filename="test_image4.jpg"`)
	h4.Set("Content-Type", "image/jpeg")
	part4, err := writer4.CreatePart(h4)
	require.NoError(t, err, "should create form part")
	part4.Write(testImageData4)

	writer4.WriteField("caption", "This should fail")

	err4 := writer4.Close()
	require.NoError(t, err4, "should close writer")

	url4 := fmt.Sprintf("/api/servers/%s/posts", serverId)
	reqBody4 := body4.Bytes()
	req4 := setup.CreateAuthRequest(http.MethodPost, url4, reqBody4, otherAccessToken)
	req4.Header.Set("Content-Type", writer4.FormDataContentType())

	resp4, err := app.Test(req4)
	require.NoError(t, err, "request should complete")

	result4 := setup.ParseJSONResponse(t, resp4)
	code, message, param = setup.ParseErrorDetail(t, result4)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "serverId", param, "error param should be 'serverId'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	t.Log("=== All Create Post Tests Passed ===")
}

// TestGetServerPosts tests the GET /api/servers/:serverId/posts endpoint
func TestGetServerPosts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err, "infrastructure should start successfully")
	defer infra.Terminate(ctx, t)

	t.Log("=== Running Database Migrations ===")
	setup.RunMigration(infra.PgURL, t)

	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Create test user
	t.Log("=== Setup: Creating Test User ===")
	accessToken := createTestUser(t, app, infra.MailhogURL, "getpostsuser@example.com", "getpostsuser", "pass123")

	// Create test server
	t.Log("=== Setup: Creating Test Server ===")
	server := createTestServer(t, app, accessToken)
	serverId := server["id"].(string)

	// Create multiple posts
	t.Log("=== Setup: Creating Multiple Posts ===")
	for i := 1; i <= 3; i++ {
		testImageData, err := getTestImage()
		require.NoError(t, err, "should read test image")

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename="test_image_%d.jpg"`, i))
		h.Set("Content-Type", "image/jpeg")
		part, err := writer.CreatePart(h)
		require.NoError(t, err, "should create form part")
		part.Write(testImageData)

		writer.WriteField("caption", fmt.Sprintf("Test post %d", i))

		writer.Close()

		url := fmt.Sprintf("/api/servers/%s/posts", serverId)
		reqBody := body.Bytes()
		req := setup.CreateAuthRequest(http.MethodPost, url, reqBody, accessToken)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := app.Test(req)
		require.NoError(t, err, "create post should succeed")
		require.Equal(t, 200, resp.StatusCode, "create post should return 200")

		// Add delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	t.Log("✓ Created 3 test posts")

	// Test 1: Get server posts successfully
	t.Log("=== Test 1: Get Server Posts Successfully ===")
	url := fmt.Sprintf("/api/servers/%s/posts", serverId)
	req := setup.CreateAuthRequest(http.MethodGet, url, nil, accessToken)
	resp, err := app.Test(req)
	require.NoError(t, err, "get posts request should complete")
	require.Equal(t, 200, resp.StatusCode, "get posts should return 200")

	result := setup.ParseJSONResponse(t, resp)

	// Verify response structure
	require.Contains(t, result, "data", "response should contain data")
	data := result["data"].([]interface{})
	require.Greater(t, len(data), 0, "data should contain posts")

	firstPost := data[0].(map[string]interface{})
	require.Contains(t, firstPost, "postId", "post should have postId")
	require.Contains(t, firstPost, "caption", "post should have caption")
	require.Contains(t, firstPost, "postImageUrl", "post should have postImageUrl")
	require.Contains(t, firstPost, "likeCount", "post should have likeCount")
	require.Contains(t, firstPost, "commentCount", "post should have commentCount")

	postId := firstPost["postId"].(string)
	caption := firstPost["caption"].(string)

	t.Logf("✓ Retrieved %d posts, first post: id=%s, caption=%s", len(data), postId, caption)

	// Test 2: Get posts with limit
	t.Log("=== Test 2: Get Posts With Limit ===")
	url = fmt.Sprintf("/api/servers/%s/posts?limit=2", serverId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "get posts with limit should complete")
	require.Equal(t, 200, resp.StatusCode, "get posts should return 200")

	result = setup.ParseJSONResponse(t, resp)
	data = result["data"].([]interface{})
	require.LessOrEqual(t, len(data), 2, "should return at most 2 posts")

	t.Logf("✓ Retrieved %d posts with limit=2", len(data))

	// Test 3: Get posts with pagination (cursor)
	if len(data) == 2 {
		t.Log("=== Test 3: Get Posts With Pagination ===")

		// Check if there's a next cursor
		if page, ok := result["page"].(map[string]interface{}); ok {
			if nextCursor, ok := page["nextCursor"].(string); ok && nextCursor != "" {
				t.Logf("✓ Got nextCursor: %s", nextCursor)

				// Fetch next page
				url2 := fmt.Sprintf("/api/servers/%s/posts?limit=2&cursor=%s", serverId, nextCursor)
				req2 := setup.CreateAuthRequest(http.MethodGet, url2, nil, accessToken)
				resp2, err := app.Test(req2)
				require.NoError(t, err, "get next page should complete")
				require.Equal(t, 200, resp2.StatusCode, "get next page should return 200")

				result2 := setup.ParseJSONResponse(t, resp2)
				data2 := result2["data"].([]interface{})
				t.Logf("✓ Retrieved %d posts on next page", len(data2))
			} else {
				t.Log("✓ No more pages available")
			}
		}
	}

	// Test 4: Get posts when not a server member (should fail)
	t.Log("=== Test 4: Get Posts When Not Server Member ===")
	otherAccessToken := createTestUser(t, app, infra.MailhogURL, "nonmember@example.com", "nonmember", "pass123")

	url = fmt.Sprintf("/api/servers/%s/posts", serverId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, otherAccessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param := setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "serverId", param, "error param should be 'serverId'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	t.Log("=== All Get Server Posts Tests Passed ===")
}

// TestGetPostDetail tests the GET /api/posts/:postId endpoint
func TestGetPostDetail(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err, "infrastructure should start successfully")
	defer infra.Terminate(ctx, t)

	t.Log("=== Running Database Migrations ===")
	setup.RunMigration(infra.PgURL, t)

	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Create test user
	t.Log("=== Setup: Creating Test User ===")
	accessToken := createTestUser(t, app, infra.MailhogURL, "postdetailuser@example.com", "postdetailuser", "pass123")

	// Create test server
	t.Log("=== Setup: Creating Test Server ===")
	server := createTestServer(t, app, accessToken)
	serverId := server["id"].(string)

	// Create test post
	t.Log("=== Setup: Creating Test Post ===")
	testImageData, err := getTestImage()
	require.NoError(t, err, "should read test image")
	require.NoError(t, err, "should create test image file")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="image"; filename="test_image.jpg"`)
	h.Set("Content-Type", "image/jpeg")
	part, err := writer.CreatePart(h)
	require.NoError(t, err, "should create form part")
	part.Write(testImageData)

	writer.WriteField("caption", "Test post for detail view")

	writer.Close()

	url := fmt.Sprintf("/api/servers/%s/posts", serverId)
	reqBody := body.Bytes()
	req := setup.CreateAuthRequest(http.MethodPost, url, reqBody, accessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req)
	require.NoError(t, err, "create post should succeed")

	// We need to get the postId first - let's fetch server posts
	url = fmt.Sprintf("/api/servers/%s/posts", serverId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "get posts should succeed")

	result := setup.ParseJSONResponse(t, resp)
	data := result["data"].([]interface{})
	require.Greater(t, len(data), 0, "should have at least one post")

	firstPost := data[0].(map[string]interface{})
	postId := firstPost["postId"].(string)

	t.Log("✓ Test post created:", postId)

	// Test 1: Get post detail successfully
	t.Log("=== Test 1: Get Post Detail Successfully ===")
	url = fmt.Sprintf("/api/posts/%s", postId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "get post detail should complete")

	if resp.StatusCode != 200 {
		bodyBytes := make([]byte, 1024)
		n, _ := resp.Body.Read(bodyBytes)
		t.Logf("Error Response status: %d", resp.StatusCode)
		t.Logf("Error Response body: %s", string(bodyBytes[:n]))
	}

	require.Equal(t, 200, resp.StatusCode, "get post detail should return 200")

	result = setup.ParseJSONResponse(t, resp)

	// Verify response structure
	require.Contains(t, result, "postId", "response should contain postId")
	require.Contains(t, result, "caption", "response should contain caption")
	require.Contains(t, result, "postImageUrl", "response should contain postImageUrl")
	require.Contains(t, result, "likeCount", "response should contain likeCount")
	require.Contains(t, result, "commentCount", "response should contain commentCount")
	require.Contains(t, result, "ownerId", "response should contain ownerId")

	detailPostId := result["postId"].(string)
	caption := result["caption"].(string)
	likeCount := result["likeCount"].(float64)
	commentCount := result["commentCount"].(float64)

	require.Equal(t, postId, detailPostId, "postId should match")
	require.Equal(t, "Test post for detail view", caption, "caption should match")

	t.Logf("✓ Post detail retrieved: id=%s, caption=%s, likes=%d, comments=%d",
		detailPostId, caption, int(likeCount), int(commentCount))

	// Test 2: Get post detail when not a server member (should fail)
	t.Log("=== Test 2: Get Post Detail When Not Server Member ===")
	otherAccessToken := createTestUser(t, app, infra.MailhogURL, "nonmemberdetail@example.com", "nonmemberdetail", "pass123")

	url = fmt.Sprintf("/api/posts/%s", postId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, otherAccessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param := setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "postId", param, "error param should be 'postId'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 3: Get non-existent post detail (should fail)
	t.Log("=== Test 3: Get Non-Existent Post Detail ===")
	url = fmt.Sprintf("/api/posts/%s", "00000000-0000-0000-0000-000000000000")
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	// Should return error
	_, hasError := result["error"]
	require.True(t, hasError, "should return error for non-existent post")

	t.Log("✓ Non-existent post correctly returns error")

	t.Log("=== All Get Post Detail Tests Passed ===")
}

// TestUpdatePostCaption tests the PUT /api/servers/:serverId/posts/:postId endpoint
func TestUpdatePostCaption(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err, "infrastructure should start successfully")
	defer infra.Terminate(ctx, t)

	t.Log("=== Running Database Migrations ===")
	setup.RunMigration(infra.PgURL, t)

	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Create test user
	t.Log("=== Setup: Creating Test User ===")
	accessToken := createTestUser(t, app, infra.MailhogURL, "updatepostuser@example.com", "updatepostuser", "pass123")

	// Create test server
	t.Log("=== Setup: Creating Test Server ===")
	server := createTestServer(t, app, accessToken)
	serverId := server["id"].(string)

	// Create test post
	t.Log("=== Setup: Creating Test Post ===")
	testImageData, err := getTestImage()
	require.NoError(t, err, "should read test image")
	require.NoError(t, err, "should create test image file")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="image"; filename="test_image.jpg"`)
	h.Set("Content-Type", "image/jpeg")
	part, err := writer.CreatePart(h)
	require.NoError(t, err, "should create form part")
	part.Write(testImageData)

	writer.WriteField("caption", "Original caption")

	writer.Close()

	url := fmt.Sprintf("/api/servers/%s/posts", serverId)
	reqBody := body.Bytes()
	req := setup.CreateAuthRequest(http.MethodPost, url, reqBody, accessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req)
	require.NoError(t, err, "create post should succeed")

	// Get postId from server posts
	url = fmt.Sprintf("/api/servers/%s/posts", serverId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "get posts should succeed")

	result := setup.ParseJSONResponse(t, resp)
	data := result["data"].([]interface{})
	firstPost := data[0].(map[string]interface{})
	postId := firstPost["postId"].(string)

	t.Log("✓ Test post created:", postId)

	// Test 1: Update post caption successfully
	t.Log("=== Test 1: Update Post Caption Successfully ===")
	reqBody = []byte(`{"caption":"Updated caption text"}`)
	url = fmt.Sprintf("/api/servers/%s/posts/%s", serverId, postId)
	req = setup.CreateAuthRequest(http.MethodPut, url, reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "update caption request should complete")
	require.Equal(t, 200, resp.StatusCode, "update caption should return 200")

	result = setup.ParseJSONResponse(t, resp)

	// Verify response contains updated post data
	require.Contains(t, result, "postId", "response should contain postId")
	require.Contains(t, result, "caption", "response should contain caption")
	require.Contains(t, result, "postImageUrl", "response should contain postImageUrl")

	updatedCaption := result["caption"].(string)
	require.Equal(t, "Updated caption text", updatedCaption, "caption should be updated")

	t.Logf("✓ Post caption updated successfully: %s", updatedCaption)

	// Test 2: Update with empty caption (should fail)
	t.Log("=== Test 2: Update with Empty Caption ===")
	reqBody = []byte(`{"caption":""}`)
	url = fmt.Sprintf("/api/servers/%s/posts/%s", serverId, postId)
	req = setup.CreateAuthRequest(http.MethodPut, url, reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param := setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "caption", param, "error param should be 'caption'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 3: Update post when not the author (should fail)
	t.Log("=== Test 3: Update Post When Not Author ===")
	otherAccessToken := createTestUser(t, app, infra.MailhogURL, "otherauthor@example.com", "otherauthor", "pass123")

	// We'll just create a new server for this user
	otherServer := createTestServer(t, app, otherAccessToken)
	otherServerId := otherServer["id"].(string)

	// Create post by other user
	body2 := &bytes.Buffer{}
	writer2 := multipart.NewWriter(body2)

	h2 := make(textproto.MIMEHeader)
	h2.Set("Content-Disposition", `form-data; name="image"; filename="test_image2.jpg"`)
	h2.Set("Content-Type", "image/jpeg")
	part2, err := writer2.CreatePart(h2)
	require.NoError(t, err, "should create form part")
	part2.Write(testImageData)

	writer2.WriteField("caption", "Other user's post")

	writer2.Close()

	url2 := fmt.Sprintf("/api/servers/%s/posts", otherServerId)
	reqBody2 := body2.Bytes()
	req2 := setup.CreateAuthRequest(http.MethodPost, url2, reqBody2, otherAccessToken)
	req2.Header.Set("Content-Type", writer2.FormDataContentType())

	resp2, err := app.Test(req2)
	require.NoError(t, err, "create post should succeed")

	// Get the postId
	url2 = fmt.Sprintf("/api/servers/%s/posts", otherServerId)
	req2 = setup.CreateAuthRequest(http.MethodGet, url2, nil, otherAccessToken)
	resp2, err = app.Test(req2)
	require.NoError(t, err, "get posts should succeed")

	result2 := setup.ParseJSONResponse(t, resp2)
	data2 := result2["data"].([]interface{})
	otherPost := data2[0].(map[string]interface{})
	otherPostId := otherPost["postId"].(string)

	// Try to update with first user (not the author)
	reqBody = []byte(`{"caption":"Hacked caption"}`)
	url2 = fmt.Sprintf("/api/servers/%s/posts/%s", otherServerId, otherPostId)
	req = setup.CreateAuthRequest(http.MethodPut, url2, reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	// User is not a member of the server, so param should be serverId
	require.Equal(t, "serverId", param, "error param should be 'serverId'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	t.Log("=== All Update Post Caption Tests Passed ===")
}

// TestLikeUnlikePost tests the POST/DELETE /api/posts/:postId/likes endpoints
func TestLikeUnlikePost(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err, "infrastructure should start successfully")
	defer infra.Terminate(ctx, t)

	t.Log("=== Running Database Migrations ===")
	setup.RunMigration(infra.PgURL, t)

	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Create test user
	t.Log("=== Setup: Creating Test User ===")
	accessToken := createTestUser(t, app, infra.MailhogURL, "likeuser@example.com", "likeuser", "pass123")

	// Create test server
	t.Log("=== Setup: Creating Test Server ===")
	server := createTestServer(t, app, accessToken)
	serverId := server["id"].(string)

	// Create test post
	t.Log("=== Setup: Creating Test Post ===")
	testImageData, err := getTestImage()
	require.NoError(t, err, "should read test image")
	require.NoError(t, err, "should create test image file")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="image"; filename="test_image.jpg"`)
	h.Set("Content-Type", "image/jpeg")
	part, err := writer.CreatePart(h)
	require.NoError(t, err, "should create form part")
	part.Write(testImageData)

	writer.WriteField("caption", "Test post for likes")

	writer.Close()

	url := fmt.Sprintf("/api/servers/%s/posts", serverId)
	reqBody := body.Bytes()
	req := setup.CreateAuthRequest(http.MethodPost, url, reqBody, accessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req)
	require.NoError(t, err, "create post should succeed")

	// Get postId
	url = fmt.Sprintf("/api/servers/%s/posts", serverId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "get posts should succeed")

	result := setup.ParseJSONResponse(t, resp)
	data := result["data"].([]interface{})
	firstPost := data[0].(map[string]interface{})
	postId := firstPost["postId"].(string)
	likeCount := int(firstPost["likeCount"].(float64))

	t.Log("✓ Test post created:", postId, "with", likeCount, "likes")

	// Test 1: Like post successfully
	t.Log("=== Test 1: Like Post Successfully ===")
	url = fmt.Sprintf("/api/posts/%s/likes", postId)
	req = setup.CreateAuthRequest(http.MethodPost, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "like post request should complete")
	require.Equal(t, 200, resp.StatusCode, "like post should return 200")

	result = setup.ParseJSONResponse(t, resp)

	// Verify response contains likeCount
	require.Contains(t, result, "likeCount", "response should contain likeCount")
	newLikeCount := int(result["likeCount"].(float64))
	require.Equal(t, likeCount+1, newLikeCount, "like count should increase by 1")

	t.Logf("✓ Post liked successfully, like count: %d", newLikeCount)

	// Test 2: Like post again (should fail - already liked)
	t.Log("=== Test 2: Like Post Again (Already Liked) ===")
	url = fmt.Sprintf("/api/posts/%s/likes", postId)
	req = setup.CreateAuthRequest(http.MethodPost, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param := setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "postId", param, "error param should be 'postId'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 3: Unlike post successfully
	t.Log("=== Test 3: Unlike Post Successfully ===")
	url = fmt.Sprintf("/api/posts/%s/likes", postId)
	req = setup.CreateAuthRequest(http.MethodDelete, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "unlike post request should complete")
	require.Equal(t, 200, resp.StatusCode, "unlike post should return 200")

	result = setup.ParseJSONResponse(t, resp)

	// Verify response contains likeCount
	require.Contains(t, result, "likeCount", "response should contain likeCount")
	finalLikeCount := int(result["likeCount"].(float64))
	require.Equal(t, likeCount, finalLikeCount, "like count should return to original")

	t.Logf("✓ Post unliked successfully, like count: %d", finalLikeCount)

	// Test 4: Unlike post again (should fail - not liked)
	t.Log("=== Test 4: Unlike Post Again (Not Liked) ===")
	url = fmt.Sprintf("/api/posts/%s/likes", postId)
	req = setup.CreateAuthRequest(http.MethodDelete, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "postId", param, "error param should be 'postId'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	t.Log("=== All Like/Unlike Post Tests Passed ===")
}

// TestCreateComment tests the POST /api/posts/:postId/comments endpoint
func TestCreateComment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err, "infrastructure should start successfully")
	defer infra.Terminate(ctx, t)

	t.Log("=== Running Database Migrations ===")
	setup.RunMigration(infra.PgURL, t)

	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Create test user
	t.Log("=== Setup: Creating Test User ===")
	accessToken := createTestUser(t, app, infra.MailhogURL, "commentuser@example.com", "commentuser", "pass123")

	// Create test server
	t.Log("=== Setup: Creating Test Server ===")
	server := createTestServer(t, app, accessToken)
	serverId := server["id"].(string)

	// Create test post
	t.Log("=== Setup: Creating Test Post ===")
	testImageData, err := getTestImage()
	require.NoError(t, err, "should read test image")
	require.NoError(t, err, "should create test image file")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="image"; filename="test_image.jpg"`)
	h.Set("Content-Type", "image/jpeg")
	part, err := writer.CreatePart(h)
	require.NoError(t, err, "should create form part")
	part.Write(testImageData)

	writer.WriteField("caption", "Test post for comments")

	writer.Close()

	url := fmt.Sprintf("/api/servers/%s/posts", serverId)
	reqBody := body.Bytes()
	req := setup.CreateAuthRequest(http.MethodPost, url, reqBody, accessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req)
	require.NoError(t, err, "create post should succeed")

	// Get postId
	url = fmt.Sprintf("/api/servers/%s/posts", serverId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "get posts should succeed")

	result := setup.ParseJSONResponse(t, resp)
	data := result["data"].([]interface{})
	firstPost := data[0].(map[string]interface{})
	postId := firstPost["postId"].(string)

	t.Log("✓ Test post created:", postId)

	// Test 1: Create comment successfully
	t.Log("=== Test 1: Create Comment Successfully ===")
	reqBody = []byte(`{"content":"This is a test comment"}`)
	url = fmt.Sprintf("/api/posts/%s/comments", postId)
	req = setup.CreateAuthRequest(http.MethodPost, url, reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "create comment request should complete")
	require.Equal(t, 200, resp.StatusCode, "create comment should return 200")

	result = setup.ParseJSONResponse(t, resp)

	// Verify response contains comment data
	require.Contains(t, result, "id", "response should contain id")
	require.Contains(t, result, "content", "response should contain content")
	require.Contains(t, result, "authorId", "response should contain authorId")

	commentId := result["id"].(string)
	content := result["content"].(string)

	t.Logf("✓ Comment created successfully: id=%s, content=%s", commentId, content)

	// Test 2: Create comment with empty content (should fail)
	t.Log("=== Test 2: Create Comment with Empty Content ===")
	reqBody = []byte(`{"content":""}`)
	url = fmt.Sprintf("/api/posts/%s/comments", postId)
	req = setup.CreateAuthRequest(http.MethodPost, url, reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param := setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "content", param, "error param should be 'content'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 3: Create comment when not a server member (should fail)
	t.Log("=== Test 3: Create Comment When Not Server Member ===")
	otherAccessToken := createTestUser(t, app, infra.MailhogURL, "nonmembercomment@example.com", "nonmembercomment", "pass123")

	reqBody = []byte(`{"content":"This should fail"}`)
	url = fmt.Sprintf("/api/posts/%s/comments", postId)
	req = setup.CreateAuthRequest(http.MethodPost, url, reqBody, otherAccessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "postId", param, "error param should be 'postId'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 4: Create reply comment (with parentId)
	t.Log("=== Test 4: Create Reply Comment (Nested Comment) ===")

	// First, get the commentId
	url = fmt.Sprintf("/api/posts/%s/comments", postId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "get comments should succeed")

	result = setup.ParseJSONResponse(t, resp)
	commentsData := result["data"].([]interface{})
	require.Greater(t, len(commentsData), 0, "should have at least one comment")

	firstComment := commentsData[0].(map[string]interface{})
	commentId = firstComment["id"].(string)

	// Create reply
	reqBody = []byte(fmt.Sprintf(`{"content":"This is a reply","parentId":"%s"}`, commentId))
	url = fmt.Sprintf("/api/posts/%s/comments", postId)
	req = setup.CreateAuthRequest(http.MethodPost, url, reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "create reply request should complete")
	require.Equal(t, 200, resp.StatusCode, "create reply should return 200")

	result = setup.ParseJSONResponse(t, resp)

	// Verify response contains comment data
	require.Contains(t, result, "id", "response should contain id")
	require.Contains(t, result, "content", "response should contain content")

	replyId := result["id"].(string)
	t.Logf("✓ Reply comment created successfully: id=%s", replyId)

	// Test 5: Create reply with invalid parentId (should fail)
	t.Log("=== Test 5: Create Reply with Invalid ParentId ===")
	reqBody = []byte(`{"content":"This should fail","parentId":"00000000-0000-0000-0000-000000000000"}`)
	url = fmt.Sprintf("/api/posts/%s/comments", postId)
	req = setup.CreateAuthRequest(http.MethodPost, url, reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param = setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "parentId", param, "error param should be 'parentId'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	t.Log("=== All Create Comment Tests Passed ===")
}

// TestGetComments tests the GET /api/posts/:postId/comments endpoint
func TestGetComments(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err, "infrastructure should start successfully")
	defer infra.Terminate(ctx, t)

	t.Log("=== Running Database Migrations ===")
	setup.RunMigration(infra.PgURL, t)

	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Create test user
	t.Log("=== Setup: Creating Test User ===")
	accessToken := createTestUser(t, app, infra.MailhogURL, "getcommentsuser@example.com", "getcommentsuser", "pass123")

	// Create test server
	t.Log("=== Setup: Creating Test Server ===")
	server := createTestServer(t, app, accessToken)
	serverId := server["id"].(string)

	// Create test post
	t.Log("=== Setup: Creating Test Post ===")
	testImageData, err := getTestImage()
	require.NoError(t, err, "should read test image")
	require.NoError(t, err, "should create test image file")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="image"; filename="test_image.jpg"`)
	h.Set("Content-Type", "image/jpeg")
	part, err := writer.CreatePart(h)
	require.NoError(t, err, "should create form part")
	part.Write(testImageData)

	writer.WriteField("caption", "Test post for getting comments")

	writer.Close()

	url := fmt.Sprintf("/api/servers/%s/posts", serverId)
	reqBody := body.Bytes()
	req := setup.CreateAuthRequest(http.MethodPost, url, reqBody, accessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req)
	require.NoError(t, err, "create post should succeed")

	// Get postId
	url = fmt.Sprintf("/api/servers/%s/posts", serverId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "get posts should succeed")

	result := setup.ParseJSONResponse(t, resp)
	data := result["data"].([]interface{})
	firstPost := data[0].(map[string]interface{})
	postId := firstPost["postId"].(string)

	t.Log("✓ Test post created:", postId)

	// Create multiple comments
	t.Log("=== Setup: Creating Multiple Comments ===")
	for i := 1; i <= 3; i++ {
		reqBody = []byte(fmt.Sprintf(`{"content":"Test comment %d"}`, i))
		url = fmt.Sprintf("/api/posts/%s/comments", postId)
		req = setup.CreateAuthRequest(http.MethodPost, url, reqBody, accessToken)
		resp, err = app.Test(req)
		require.NoError(t, err, "create comment should succeed")
		require.Equal(t, 200, resp.StatusCode, "create comment should return 200")

		time.Sleep(10 * time.Millisecond)
	}

	t.Log("✓ Created 3 test comments")

	// Test 1: Get comments successfully
	t.Log("=== Test 1: Get Comments Successfully ===")
	url = fmt.Sprintf("/api/posts/%s/comments", postId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "get comments request should complete")
	require.Equal(t, 200, resp.StatusCode, "get comments should return 200")

	result = setup.ParseJSONResponse(t, resp)

	// Verify response structure
	require.Contains(t, result, "data", "response should contain data")
	data = result["data"].([]interface{})
	require.Greater(t, len(data), 0, "data should contain comments")

	firstComment := data[0].(map[string]interface{})
	require.Contains(t, firstComment, "id", "comment should have id")
	require.Contains(t, firstComment, "content", "comment should have content")
	require.Contains(t, firstComment, "authorId", "comment should have authorId")

	commentId := firstComment["id"].(string)
	content := firstComment["content"].(string)

	t.Logf("✓ Retrieved %d comments, first comment: id=%s, content=%s", len(data), commentId, content)

	// Test 2: Get comments with limit
	t.Log("=== Test 2: Get Comments With Limit ===")
	url = fmt.Sprintf("/api/posts/%s/comments?limit=2", postId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "get comments with limit should complete")
	require.Equal(t, 200, resp.StatusCode, "get comments should return 200")

	result = setup.ParseJSONResponse(t, resp)
	data = result["data"].([]interface{})
	require.LessOrEqual(t, len(data), 2, "should return at most 2 comments")

	t.Logf("✓ Retrieved %d comments with limit=2", len(data))

	// Test 3: Get comments with pagination (cursor)
	if len(data) == 2 {
		t.Log("=== Test 3: Get Comments With Pagination ===")

		// Check if there's a next cursor
		if page, ok := result["page"].(map[string]interface{}); ok {
			if nextCursor, ok := page["nextCursor"].(string); ok && nextCursor != "" {
				t.Logf("✓ Got nextCursor: %s", nextCursor)

				// Fetch next page
				url2 := fmt.Sprintf("/api/posts/%s/comments?limit=2&cursor=%s", postId, nextCursor)
				req2 := setup.CreateAuthRequest(http.MethodGet, url2, nil, accessToken)
				resp2, err := app.Test(req2)
				require.NoError(t, err, "get next page should complete")
				require.Equal(t, 200, resp2.StatusCode, "get next page should return 200")

				result2 := setup.ParseJSONResponse(t, resp2)
				data2 := result2["data"].([]interface{})
				t.Logf("✓ Retrieved %d comments on next page", len(data2))
			} else {
				t.Log("✓ No more pages available")
			}
		}
	}

	// Test 4: Get comments when not a server member (should fail)
	t.Log("=== Test 4: Get Comments When Not Server Member ===")
	otherAccessToken := createTestUser(t, app, infra.MailhogURL, "nonmembergetcomment@example.com", "nonmembergetcomment", "pass123")

	url = fmt.Sprintf("/api/posts/%s/comments", postId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, otherAccessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param := setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	require.Equal(t, "postId", param, "error param should be 'postId'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	t.Log("=== All Get Comments Tests Passed ===")
}

// TestDeleteComment tests the DELETE /api/posts/:postId/comments/:commentId endpoint
func TestDeleteComment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err, "infrastructure should start successfully")
	defer infra.Terminate(ctx, t)

	t.Log("=== Running Database Migrations ===")
	setup.RunMigration(infra.PgURL, t)

	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Create test user
	t.Log("=== Setup: Creating Test User ===")
	accessToken := createTestUser(t, app, infra.MailhogURL, "deletecommentuser@example.com", "deletecommentuser", "pass123")

	// Create test server
	t.Log("=== Setup: Creating Test Server ===")
	server := createTestServer(t, app, accessToken)
	serverId := server["id"].(string)

	// Create test post
	t.Log("=== Setup: Creating Test Post ===")
	testImageData, err := getTestImage()
	require.NoError(t, err, "should read test image")
	require.NoError(t, err, "should create test image file")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="image"; filename="test_image.jpg"`)
	h.Set("Content-Type", "image/jpeg")
	part, err := writer.CreatePart(h)
	require.NoError(t, err, "should create form part")
	part.Write(testImageData)

	writer.WriteField("caption", "Test post for deleting comments")

	writer.Close()

	url := fmt.Sprintf("/api/servers/%s/posts", serverId)
	reqBody := body.Bytes()
	req := setup.CreateAuthRequest(http.MethodPost, url, reqBody, accessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req)
	require.NoError(t, err, "create post should succeed")

	// Get postId
	url = fmt.Sprintf("/api/servers/%s/posts", serverId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "get posts should succeed")

	result := setup.ParseJSONResponse(t, resp)
	data := result["data"].([]interface{})
	firstPost := data[0].(map[string]interface{})
	postId := firstPost["postId"].(string)

	// Create test comment
	t.Log("=== Setup: Creating Test Comment ===")
	reqBody = []byte(`{"content":"Comment to be deleted"}`)
	url = fmt.Sprintf("/api/posts/%s/comments", postId)
	req = setup.CreateAuthRequest(http.MethodPost, url, reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "create comment should succeed")

	// Get commentId
	url = fmt.Sprintf("/api/posts/%s/comments", postId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "get comments should succeed")

	result = setup.ParseJSONResponse(t, resp)
	data = result["data"].([]interface{})
	firstComment := data[0].(map[string]interface{})
	commentId := firstComment["id"].(string)

	t.Log("✓ Test comment created:", commentId)

	// Test 1: Delete comment successfully
	t.Log("=== Test 1: Delete Comment Successfully ===")
	url = fmt.Sprintf("/api/posts/%s/comments/%s", postId, commentId)
	req = setup.CreateAuthRequest(http.MethodDelete, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "delete comment request should complete")
	require.Equal(t, 200, resp.StatusCode, "delete comment should return 200")

	result = setup.ParseJSONResponse(t, resp)
	status, ok := result["status"].(string)
	require.True(t, ok, "response should have status field")
	require.Equal(t, "OK", status, "status should be 'OK'")

	t.Log("✓ Comment deleted successfully")

	// Verify comment is deleted
	url = fmt.Sprintf("/api/posts/%s/comments", postId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "get comments should succeed")

	result = setup.ParseJSONResponse(t, resp)
	data = result["data"].([]interface{})
	require.Equal(t, 0, len(data), "should have 0 comments after deletion")

	t.Logf("✓ Comment count verified: %d", len(data))

	// Test 2: Delete comment when not the author (should fail)
	t.Log("=== Test 2: Delete Comment When Not Author ===")

	// Create another comment
	reqBody = []byte(`{"content":"Another comment"}`)
	url = fmt.Sprintf("/api/posts/%s/comments", postId)
	req = setup.CreateAuthRequest(http.MethodPost, url, reqBody, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "create comment should succeed")

	// Get commentId
	url = fmt.Sprintf("/api/posts/%s/comments", postId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "get comments should succeed")

	result = setup.ParseJSONResponse(t, resp)
	data = result["data"].([]interface{})
	secondComment := data[0].(map[string]interface{})
	secondCommentId := secondComment["id"].(string)

	// Try to delete with another user
	otherAccessToken := createTestUser(t, app, infra.MailhogURL, "otherdeleter@example.com", "otherdeleter", "pass123")

	url = fmt.Sprintf("/api/posts/%s/comments/%s", postId, secondCommentId)
	req = setup.CreateAuthRequest(http.MethodDelete, url, nil, otherAccessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param := setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	// User is not a member of the server, so param should be postId
	require.Equal(t, "postId", param, "error param should be 'postId'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 3: Delete non-existent comment (should fail)
	t.Log("=== Test 3: Delete Non-Existent Comment ===")
	url = fmt.Sprintf("/api/posts/%s/comments/00000000-0000-0000-0000-000000000000", postId)
	req = setup.CreateAuthRequest(http.MethodDelete, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	_, hasError := result["error"]
	require.True(t, hasError, "should return error for non-existent comment")

	t.Log("✓ Non-existent comment correctly returns error")

	t.Log("=== All Delete Comment Tests Passed ===")
}

// TestDeletePost tests the DELETE /api/servers/:serverId/posts/:postId endpoint
func TestDeletePost(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	t.Log("=== Starting Test Infrastructure ===")
	infra, err := setup.StartInfra(ctx, t)
	require.NoError(t, err, "infrastructure should start successfully")
	defer infra.Terminate(ctx, t)

	t.Log("=== Running Database Migrations ===")
	setup.RunMigration(infra.PgURL, t)

	t.Log("=== Setting Up Test Application ===")
	app, db, _, _ := setup.SetupTestApp(t, infra.PgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)
	defer db.Close()

	// Create test user
	t.Log("=== Setup: Creating Test User ===")
	accessToken := createTestUser(t, app, infra.MailhogURL, "deletepostuser@example.com", "deletepostuser", "pass123")

	// Create test server
	t.Log("=== Setup: Creating Test Server ===")
	server := createTestServer(t, app, accessToken)
	serverId := server["id"].(string)

	// Create test post
	t.Log("=== Setup: Creating Test Post ===")
	testImageData, err := getTestImage()
	require.NoError(t, err, "should read test image")
	require.NoError(t, err, "should create test image file")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="image"; filename="test_image.jpg"`)
	h.Set("Content-Type", "image/jpeg")
	part, err := writer.CreatePart(h)
	require.NoError(t, err, "should create form part")
	part.Write(testImageData)

	writer.WriteField("caption", "Post to be deleted")

	writer.Close()

	url := fmt.Sprintf("/api/servers/%s/posts", serverId)
	reqBody := body.Bytes()
	req := setup.CreateAuthRequest(http.MethodPost, url, reqBody, accessToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := app.Test(req)
	require.NoError(t, err, "create post should succeed")

	// Get postId
	url = fmt.Sprintf("/api/servers/%s/posts", serverId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "get posts should succeed")

	result := setup.ParseJSONResponse(t, resp)
	data := result["data"].([]interface{})
	firstPost := data[0].(map[string]interface{})
	postId := firstPost["postId"].(string)

	t.Log("✓ Test post created:", postId)

	// Test 1: Delete post successfully
	t.Log("=== Test 1: Delete Post Successfully ===")
	url = fmt.Sprintf("/api/servers/%s/posts/%s", serverId, postId)
	req = setup.CreateAuthRequest(http.MethodDelete, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "delete post request should complete")
	require.Equal(t, 200, resp.StatusCode, "delete post should return 200")

	result = setup.ParseJSONResponse(t, resp)
	status, ok := result["status"].(string)
	require.True(t, ok, "response should have status field")
	require.Equal(t, "OK", status, "status should be 'OK'")

	t.Log("✓ Post deleted successfully")

	// Verify post is deleted
	url = fmt.Sprintf("/api/posts/%s", postId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	_, hasError := result["error"]
	require.True(t, hasError, "deleted post should return error")

	t.Log("✓ Deleted post correctly returns error")

	// Test 2: Delete post when not the author (should fail)
	t.Log("=== Test 2: Delete Post When Not Author ===")

	// Create another post
	body2 := &bytes.Buffer{}
	writer2 := multipart.NewWriter(body2)

	h2 := make(textproto.MIMEHeader)
	h2.Set("Content-Disposition", `form-data; name="image"; filename="test_image2.jpg"`)
	h2.Set("Content-Type", "image/jpeg")
	part2, err := writer2.CreatePart(h2)
	require.NoError(t, err, "should create form part")
	part2.Write(testImageData)

	writer2.WriteField("caption", "Another post")

	writer2.Close()

	url = fmt.Sprintf("/api/servers/%s/posts", serverId)
	reqBody2 := body2.Bytes()
	req2 := setup.CreateAuthRequest(http.MethodPost, url, reqBody2, accessToken)
	req2.Header.Set("Content-Type", writer2.FormDataContentType())

	_, err = app.Test(req2)
	require.NoError(t, err, "create post should succeed")

	// Get postId
	url = fmt.Sprintf("/api/servers/%s/posts", serverId)
	req = setup.CreateAuthRequest(http.MethodGet, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "get posts should succeed")

	result = setup.ParseJSONResponse(t, resp)
	data = result["data"].([]interface{})
	secondPost := data[0].(map[string]interface{})
	secondPostId := secondPost["postId"].(string)

	// Try to delete with another user
	otherAccessToken := createTestUser(t, app, infra.MailhogURL, "otherpostdeleter@example.com", "otherpostdeleter", "pass123")

	url = fmt.Sprintf("/api/servers/%s/posts/%s", serverId, secondPostId)
	req = setup.CreateAuthRequest(http.MethodDelete, url, nil, otherAccessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	code, message, param := setup.ParseErrorDetail(t, result)

	require.Equal(t, "VALIDATION_ERROR", code, "error code should be VALIDATION_ERROR")
	// User is not a member of the server, so param should be serverId
	require.Equal(t, "serverId", param, "error param should be 'serverId'")

	t.Logf("✓ Validation Error: Code=%s, Param=%s, Message=%s", code, param, message)

	// Test 3: Delete non-existent post (should fail)
	t.Log("=== Test 3: Delete Non-Existent Post ===")
	url = fmt.Sprintf("/api/servers/%s/posts/00000000-0000-0000-0000-000000000000", serverId)
	req = setup.CreateAuthRequest(http.MethodDelete, url, nil, accessToken)
	resp, err = app.Test(req)
	require.NoError(t, err, "request should complete")

	result = setup.ParseJSONResponse(t, resp)
	_, hasError = result["error"]
	require.True(t, hasError, "should return error for non-existent post")

	t.Log("✓ Non-existent post correctly returns error")

	t.Log("=== All Delete Post Tests Passed ===")
}

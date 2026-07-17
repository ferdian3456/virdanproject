package setup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TruncateAllTables(t *testing.T, db *pgxpool.Pool, ctx context.Context) {
	t.Log("Truncating all database tables...")

	tables := []string{
		"server_post_likes",
		"server_post_comments",
		"server_posts",
		"server_post_images",
		"server_members",
		"server_invites",
		"server_roles",
		"server_banner_images",
		"server_avatar_images",
		"servers",
		"server_categories",
		"user_avatar_images",
		"users",
	}

	for _, table := range tables {
		_, err := db.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		require.NoError(t, err, "failed to truncate table %s", table)
	}

	t.Log("All database tables truncated successfully")
}

func CreateTestWebPImage(t *testing.T) []byte {
	_, filename, _, _ := runtime.Caller(0)
	currentDir := filepath.Dir(filename)
	imagePath := filepath.Join(currentDir, "..", "testdata", "itachi.jpg")

	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatalf("Failed to read test image: %v (path: %s)", err, imagePath)
	}

	return imageData
}

const appTestTimeout = 30 * time.Second

func AppTest(t *testing.T, app *fiber.App, req *http.Request) (*http.Response, error) {
	t.Helper()
	return app.Test(req, fiber.TestConfig{Timeout: appTestTimeout, FailOnTimeout: true})
}

func CreateMultipartTextOnly(t *testing.T, fields map[string]string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		err := writer.WriteField(key, value)
		require.NoError(t, err, "failed to write form field %s", key)
	}
	err := writer.Close()
	require.NoError(t, err, "failed to close multipart writer")
	return body, writer.FormDataContentType()
}

func CreateMultipartFormData(t *testing.T, fieldName, fileName string, fileData []byte, fields map[string]string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	fileContentType := "application/octet-stream"
	ext := filepath.Ext(fileName)
	switch ext {
	case ".jpg", ".jpeg":
		fileContentType = "image/jpeg"
	case ".png":
		fileContentType = "image/png"
	case ".gif":
		fileContentType = "image/gif"
	case ".webp":
		fileContentType = "image/webp"
	}

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fileName))
	h.Set("Content-Type", fileContentType)
	part, err := writer.CreatePart(h)
	require.NoError(t, err, "failed to create form file field")

	_, err = part.Write(fileData)
	require.NoError(t, err, "failed to write file data")

	for key, value := range fields {
		err = writer.WriteField(key, value)
		require.NoError(t, err, "failed to write form field %s", key)
	}

	err = writer.Close()
	require.NoError(t, err, "failed to close multipart writer")

	contentType := writer.FormDataContentType()
	return body, contentType
}

func CreateJSONRequest(method, url string, jsonBody []byte) *http.Request {
	req := httptest.NewRequest(method, url, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func CreateAuthRequest(method, url string, jsonBody []byte, token string) *http.Request {
	req := CreateJSONRequest(method, url, jsonBody)
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	return req
}

func CreateAuthMultipartRequest(method, url string, body *bytes.Buffer, contentType string, token string) *http.Request {
	req := httptest.NewRequest(method, url, body)
	req.Header.Set("Content-Type", contentType)
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	return req
}

func ParseJSONResponse(t *testing.T, resp *http.Response) map[string]interface{} {
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "failed to read response body")
	require.NotEmpty(t, body, "response body should not be empty")

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		t.Errorf("failed to parse JSON response: %v. Body: %s", err, string(body))
		t.FailNow()
	}

	return result
}

func RequireStatus(t *testing.T, resp *http.Response, expected int) {
	if resp.StatusCode != expected {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status %d, got %d. Body: %s", expected, resp.StatusCode, string(body))
		t.FailNow()
	}
}

func RequireJSONResponse(t *testing.T, resp *http.Response, expectedStatus int) map[string]interface{} {
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "failed to read response body")

	if resp.StatusCode != expectedStatus {
		t.Errorf("Expected status %d, got %d. Body: %s", expectedStatus, resp.StatusCode, string(body))
		t.FailNow()
	}

	require.NotEmpty(t, body, "response body should not be empty")

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		t.Errorf("failed to parse JSON response: %v. Body: %s", err, string(body))
		t.FailNow()
	}

	return result
}

func GetOTPFromMailhog(t *testing.T, mailhogURL, email string) string {
	t.Logf("Fetching OTP from MailHog for email: %s", email)

	apiURL := fmt.Sprintf("%s/api/v1/messages", mailhogURL)

	maxAttempts := 10
	var otp string

	for i := 0; i < maxAttempts; i++ {
		t.Logf("Attempt %d: Fetching messages from MailHog", i+1)

		resp, err := http.Get(apiURL)
		require.NoError(t, err, "failed to fetch messages from MailHog")
		defer func() { _ = resp.Body.Close() }()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "failed to read MailHog response")

		var rawMessages []map[string]interface{}
		err = json.Unmarshal(body, &rawMessages)
		require.NoError(t, err, "failed to parse MailHog JSON response")

		t.Logf("Found %d messages in MailHog", len(rawMessages))

		if len(rawMessages) > 0 {
			t.Logf("First message structure: %+v", rawMessages[0])
		}

		for _, rawMsg := range rawMessages {
			matchesTargetEmail := false

			if toField, ok := rawMsg["To"].([]interface{}); ok {
				for _, recipient := range toField {
					if recipientMap, ok := recipient.(map[string]interface{}); ok {
						if mailbox, ok := recipientMap["Mailbox"].(string); ok {
							if domain, ok := recipientMap["Domain"].(string); ok {
								fullEmail := mailbox + "@" + domain
								if fullEmail == email {
									matchesTargetEmail = true
									t.Logf("Found message for target email: %s (via top-level To)", email)
									break
								}
							}
						}
					}
				}
			}

			if !matchesTargetEmail {
				continue
			}

			var emailBody string

			if content, ok := rawMsg["Content"].(map[string]interface{}); ok {
				if body, ok := content["Body"].(string); ok {
					emailBody = body
				}
			}

			if emailBody != "" {
				t.Logf("Found email for %s with body length: %d", email, len(emailBody))

				patterns := []string{
					`Your OTP code is:\s*(\d{6})`,
					`OTP code is:\s*(\d{6})`,
					`(\d{6})`,
					`otp.*?(\d{6})`,
					`code.*?(\d{6})`,
				}

				for _, pattern := range patterns {
					re := regexp.MustCompile(`(?i)` + pattern)
					matches := re.FindStringSubmatch(emailBody)

					if len(matches) > 1 {
						otp = matches[1]
						t.Logf("OTP extracted successfully with pattern '%s': %s", pattern, otp)
						return otp
					}
				}

				t.Logf("Email found but OTP pattern not matched. Body (first 500 chars): %.500s", emailBody)
			}
		}

		if i < maxAttempts-1 {
			t.Logf("OTP not found yet, waiting 500ms before retry...")
			time.Sleep(500 * time.Millisecond)
		}
	}

	require.Fail(t, "OTP not found in email after %d attempts", maxAttempts)
	return ""
}

func GenerateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

type APIResponse struct {
	Status string         `json:"status,omitempty"`
	Data   interface{}    `json:"data,omitempty"`
	Page   *PageInfo      `json:"page,omitempty"`
	Error  *ErrorResponse `json:"error,omitempty"`
}

type PageInfo struct {
	NextCursor string `json:"nextCursor"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param,omitempty"`
}

func ParseErrorMessage(t *testing.T, result map[string]interface{}) string {
	errResp := ParseErrorResponse(t, result)
	return errResp.Message
}

func ParseErrorDetail(t *testing.T, result map[string]interface{}) (code, message, param string) {
	errResp := ParseErrorResponse(t, result)
	return errResp.Code, errResp.Message, errResp.Param
}

func ParseErrorResponse(t *testing.T, result map[string]interface{}) ErrorResponse {
	require.Contains(t, result, "error", "response should contain error field")

	errObj, ok := result["error"].(map[string]interface{})
	require.True(t, ok, "error field should be an object")

	errResp := ErrorResponse{}

	if code, ok := errObj["code"].(string); ok {
		errResp.Code = code
	}

	if message, ok := errObj["message"].(string); ok {
		errResp.Message = message
	}

	if param, ok := errObj["param"].(string); ok {
		errResp.Param = param
	}

	return errResp
}

func ParseAPIResponse(t *testing.T, resp *http.Response) APIResponse {
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "failed to read response body")
	require.NotEmpty(t, body, "response body should not be empty")

	var apiResp APIResponse
	err = json.Unmarshal(body, &apiResp)
	require.NoError(t, err, "failed to parse JSON response")

	return apiResp
}

func IsSuccess(t *testing.T, resp APIResponse) bool {
	return resp.Error == nil
}

func GetStatus(t *testing.T, resp APIResponse) string {
	require.NotEmpty(t, resp.Status, "response should have status field")
	return resp.Status
}

func GetDataAsMap(t *testing.T, resp APIResponse) map[string]interface{} {
	require.NotNil(t, resp.Data, "response should have data field")
	dataMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "data field should be an object/map")
	return dataMap
}

func GetDataAsArray(t *testing.T, resp APIResponse) []interface{} {
	require.NotNil(t, resp.Data, "response should have data field")
	dataArray, ok := resp.Data.([]interface{})
	require.True(t, ok, "data field should be an array")
	return dataArray
}

func GetNextCursor(t *testing.T, resp APIResponse) string {
	require.NotNil(t, resp.Page, "response should have page field")
	return resp.Page.NextCursor
}

func HasPagination(resp APIResponse) bool {
	return resp.Page != nil
}

func CreateTestUser(t *testing.T, app *fiber.App, mailhogURL, email, password string) string {
	reqBody := []byte(fmt.Sprintf(`{"email":%q}`, email))
	req := CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
	require.NoError(t, err, "signup start should succeed")

	result := RequireJSONResponse(t, resp, 200)
	sessionId, ok := result["sessionId"].(string)
	require.True(t, ok, "sessionId should be a string, got: %v", result)

	otp := GetOTPFromMailhog(t, mailhogURL, email)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":%q,"otp":%q}`, sessionId, otp))
	req = CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
	require.NoError(t, err, "verify OTP should succeed")
	RequireStatus(t, resp, 200)

	reqBody = []byte(fmt.Sprintf(`{"sessionId":%q,"password":%q}`, sessionId, password))
	req = CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
	require.NoError(t, err, "set password should succeed")

	result = RequireJSONResponse(t, resp, 200)
	accessToken, ok := result["accessToken"].(string)
	require.True(t, ok, "accessToken should be a string, got: %v", result)
	require.NotEmpty(t, accessToken, "accessToken should not be empty")

	return accessToken
}

func LoginTestUser(t *testing.T, app *fiber.App, email, password string) string {
	reqBody := []byte(fmt.Sprintf(`{"email":%q,"password":%q}`, email, password))
	req := CreateJSONRequest(http.MethodPost, "/api/auth/login", reqBody)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
	require.NoError(t, err, "login should succeed")

	result := RequireJSONResponse(t, resp, 200)
	accessToken, ok := result["accessToken"].(string)
	require.True(t, ok, "accessToken should be a string, got: %v", result)
	require.NotEmpty(t, accessToken, "accessToken should not be empty")

	return accessToken
}

func CreateTestServer(t *testing.T, app *fiber.App, redisURL, token, name, shortName string, categoryID int, isPrivate bool) string {
	_ = redisURL
	username := deriveUsernameFromShortName(shortName)

	fields := map[string]string{
		"name":        name,
		"shortName":   shortName,
		"categoryId":  strconv.Itoa(categoryID),
		"isPrivate":   strconv.FormatBool(isPrivate),
		"nickname":    name,
		"username":    username,
		"description": "Test server description",
	}
	body, contentType := CreateMultipartTextOnly(t, fields)
	req := CreateAuthMultipartRequest(http.MethodPost, "/api/servers/create", body, contentType, token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
	require.NoError(t, err, "create server should succeed")

	result := RequireJSONResponse(t, resp, 200)
	serverObj, ok := result["server"].(map[string]interface{})
	require.True(t, ok, "response should contain server object, got: %v", result)
	serverID, ok := serverObj["id"].(string)
	require.True(t, ok, "server.id should be a string")
	require.NotEmpty(t, serverID, "server id should not be empty")

	return serverID
}

func JoinTestServer(t *testing.T, app *fiber.App, token, serverID, nickname, username, bio string) {
	fields := map[string]string{
		"nickname": nickname,
		"username": username,
	}
	if bio != "" {
		fields["bio"] = bio
	}
	body, contentType := CreateMultipartTextOnly(t, fields)
	req := CreateAuthMultipartRequest(http.MethodPost, fmt.Sprintf("/api/servers/%s/join", serverID), body, contentType, token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
	require.NoError(t, err, "join server should succeed")
	RequireStatus(t, resp, 200)
}

func GetUserId(t *testing.T, app *fiber.App, token string) string {
	req := CreateAuthRequest(http.MethodGet, "/api/users/me", nil, token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
	require.NoError(t, err, "get users/me should succeed")
	result := RequireJSONResponse(t, resp, 200)
	id, ok := result["id"].(string)
	require.True(t, ok, "users/me should return id, got: %v", result)
	require.NotEmpty(t, id, "user id should not be empty")
	return id
}

func deriveUsernameFromShortName(shortName string) string {
	if len(shortName) >= 3 {
		return shortName
	}
	return shortName + "_u"
}

func SetupParallelTest(t *testing.T) (*fiber.App, *pgxpool.Pool, *redis.Client, *minio.Client) {
	t.Log("Setting up parallel test with singleton infrastructure...")

	if err := EnsureSingletonInitialized(); err != nil {
		t.Fatalf("Failed to initialize singleton infrastructure: %v", err)
	}

	infra := GetGlobalInfra()
	if infra == nil {
		t.Fatal("Global infrastructure not initialized after EnsureSingletonInitialized()")
	}

	testDBName := sanitizeTestName(t.Name())
	createTestDatabase(t, infra.PgURL, testDBName)

	testPgURL := replaceDBName(infra.PgURL, testDBName)

	app, db, redisClient, minioClient := SetupTestApp(t, testPgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)

	t.Cleanup(func() {
		if db != nil {
			db.Close()
		}
		dropTestDatabase(t, infra.PgURL, testDBName)
	})

	return app, db, redisClient, minioClient
}

func sanitizeTestName(testName string) string {
	name := testName
	name = strings.TrimPrefix(name, "/")
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		" ", "_",
		"-", "_",
		".", "_",
	)
	name = replacer.Replace(name)
	if len(name) > 40 {
		name = name[:40]
	}
	return "test_" + name
}

func replaceDBName(pgURL, newDBName string) string {
	parts := strings.Split(pgURL, "/")
	if len(parts) >= 5 {
		parts[len(parts)-1] = newDBName
		if idx := strings.Index(newDBName, "?"); idx > 0 {
			parts[len(parts)-1] = newDBName
		}
		return strings.Join(parts, "/")
	}
	lastSlash := strings.LastIndex(pgURL, "/")
	if lastSlash > 0 {
		queryStart := strings.Index(pgURL[lastSlash+1:], "?")
		if queryStart > 0 {
			return pgURL[:lastSlash+1] + newDBName + pgURL[lastSlash+1+queryStart:]
		}
		return pgURL[:lastSlash+1] + newDBName
	}
	return pgURL
}

func createTestDatabase(t *testing.T, pgURL, dbName string) {
	ctx := context.Background()
	adminURL := replaceDBName(pgURL, "postgres")
	pool, err := pgxpool.New(ctx, adminURL)
	require.NoError(t, err, "failed to connect to admin database")
	defer pool.Close()

	_, _ = pool.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName))

	_, err = pool.Exec(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, dbName))
	require.NoError(t, err, "failed to create test database %s", dbName)

	t.Logf("Created test database: %s", dbName)

	pool.Close()

	testPgURL := replaceDBName(pgURL, dbName)
	if err := RunMigration(testPgURL, t); err != nil {
		t.Fatalf("failed to run migrations on test database %s: %v", dbName, err)
	}
}

func dropTestDatabase(t *testing.T, pgURL, dbName string) {
	ctx := context.Background()
	adminURL := replaceDBName(pgURL, "postgres")
	pool, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Logf("Warning: failed to connect to admin database for cleanup: %v", err)
		return
	}
	defer pool.Close()

	_, _ = pool.Exec(ctx, fmt.Sprintf(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND pid <> pg_backend_pid()`, dbName))
	_, err = pool.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName))
	if err != nil {
		t.Logf("Warning: failed to drop test database %s: %v", dbName, err)
		return
	}

	t.Logf("Dropped test database: %s", dbName)
}

type LoggedRequest struct {
	Method string
	URL    string
	Header http.Header
	Body   string
	Token  string
}

type LoggedResponse struct {
	StatusCode int
	Header     http.Header
	Body       string
}

func LogHTTPRequest(t *testing.T, req *http.Request) {
	loggedReq := LoggedRequest{
		Method: req.Method,
		URL:    req.URL.String(),
		Header: req.Header,
		Token:  req.Header.Get("Authorization"),
	}

	if req.Body != nil && req.Body != http.NoBody {
		bodyBytes, _ := io.ReadAll(req.Body)
		loggedReq.Body = string(bodyBytes)
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	t.Logf(">>> HTTP REQUEST <<<")
	t.Logf("  Method: %s", loggedReq.Method)
	t.Logf("  URL:    %s", loggedReq.URL)

	t.Logf("  Headers:")
	for key, values := range loggedReq.Header {
		if key == "Authorization" && len(values) > 0 && len(values[0]) > 50 {
			t.Logf("    %s: Bearer *** (truncated)", key)
		} else {
			t.Logf("    %s: %s", key, values)
		}
	}

	if loggedReq.Body != "" {
		bodyPreview := loggedReq.Body
		if len(bodyPreview) > 500 {
			bodyPreview = bodyPreview[:500] + "... (truncated)"
		}
		t.Logf("  Body:   %s", bodyPreview)
	}
	t.Logf(">>> END REQUEST <<<")
}

func LogHTTPResponse(t *testing.T, resp *http.Response) {
	loggedResp := LoggedResponse{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err == nil {
		loggedResp.Body = string(bodyBytes)
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	t.Logf("<<< HTTP RESPONSE <<<")
	t.Logf("  Status: %d %s", loggedResp.StatusCode, resp.Status)

	t.Logf("  Headers:")
	for key, values := range loggedResp.Header {
		t.Logf("    %s: %s", key, values)
	}

	if loggedResp.Body != "" {
		bodyPreview := loggedResp.Body
		if len(bodyPreview) > 500 {
			bodyPreview = bodyPreview[:500] + "... (truncated)"
		}
		t.Logf("  Body:   %s", bodyPreview)
	}
	t.Logf("<<< END RESPONSE <<<")

	if loggedResp.StatusCode >= 400 {
		t.Logf("⚠️  ERROR RESPONSE: Status %d", loggedResp.StatusCode)
	}
}

func LogTestRequest(t *testing.T, testName string, req *http.Request, resp *http.Response, err error) {
	t.Logf("===== %s =====", testName)
	if err != nil {
		t.Logf("❌ REQUEST FAILED: %v", err)
	} else {
		LogHTTPRequest(t, req)
		LogHTTPResponse(t, resp)
	}
	t.Logf("==================")
}

func TestRequestWithLogging(t *testing.T, app *fiber.App, req *http.Request) (*http.Response, error) {
	t.Logf("--- Executing Request: %s %s ---", req.Method, req.URL.Path)
	LogHTTPRequest(t, req)

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})

	if err != nil {
		t.Logf("❌ Request Error: %v", err)
		return nil, err
	}

	LogHTTPResponse(t, resp)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		t.Logf("✅ Request Success: %d", resp.StatusCode)
	} else if resp.StatusCode >= 400 {
		t.Logf("⚠️  Request Error Response: %d", resp.StatusCode)
	}

	return resp, nil
}

func RequireStatusWithLog(t *testing.T, req *http.Request, resp *http.Response, expected int) {
	if resp.StatusCode != expected {
		t.Logf("❌ Status Code Mismatch!")
		t.Logf("   Expected: %d, Got: %d", expected, resp.StatusCode)

		LogHTTPRequest(t, req)

		bodyBytes, _ := io.ReadAll(resp.Body)
		if len(bodyBytes) > 0 {
			bodyPreview := string(bodyBytes)
			if len(bodyPreview) > 500 {
				bodyPreview = bodyPreview[:500] + "... (truncated)"
			}
			t.Logf("❌ Response Body:")
			t.Logf("   %s", bodyPreview)
		}

		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		t.Fatalf("Expected status %d, got %d", expected, resp.StatusCode)
	}
}

func RequireJSONWithLog(t *testing.T, app *fiber.App, req *http.Request, expectedStatus int) map[string]interface{} {
	t.Logf("--- Executing Request: %s %s ---", req.Method, req.URL.Path)
	LogHTTPRequest(t, req)

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
	if err != nil {
		t.Logf("❌ Request Failed: %v", err)
		t.Fatalf("Request failed: %v", err)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	if resp.StatusCode != expectedStatus {
		t.Logf("❌ Status Code Mismatch!")
		t.Logf("   Expected: %d, Got: %d", expectedStatus, resp.StatusCode)

		bodyPreview := string(bodyBytes)
		if len(bodyPreview) > 500 {
			bodyPreview = bodyPreview[:500] + "... (truncated)"
		}
		t.Logf("❌ Response Body:")
		t.Logf("   %s", bodyPreview)

		t.Fatalf("Expected status %d, got %d", expectedStatus, resp.StatusCode)
	}

	if len(bodyBytes) == 0 {
		t.Logf("❌ Empty response body")
		t.Fatalf("Response body is empty for status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		t.Logf("❌ Failed to parse JSON response")
		t.Logf("   Error: %v", err)
		t.Logf("   Raw Body: %s", string(bodyBytes))
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	t.Logf("✅ Request Successful: %d", resp.StatusCode)
	if len(bodyBytes) < 500 {
		t.Logf("   Response: %s", string(bodyBytes))
	} else {
		t.Logf("   Response: %s... (truncated)", string(bodyBytes[:500]))
	}

	return result
}

func AssertNoError(t *testing.T, err error, msg string, args ...interface{}) {
	if err != nil {
		t.Logf("❌ Error: %s", fmt.Sprintf(msg, args...))
		t.Logf("   Error details: %v", err)
		require.NoError(t, err, fmt.Sprintf(msg, args...))
	}
}

func LogTestStep(t *testing.T, step string, args ...interface{}) {
	t.Logf("📌 %s", fmt.Sprintf(step, args...))
}

func LogTestStart(t *testing.T, testName string) {
	t.Logf("========================================")
	t.Logf("🧪 Starting: %s", testName)
	t.Logf("========================================")
}

func LogTestPass(t *testing.T, testName string) {
	t.Logf("✅ %s PASSED", testName)
}

func LogTestFail(t *testing.T, testName, reason string, args ...interface{}) {
	t.Logf("❌ %s FAILED", testName)
	t.Logf("   Reason: %s", fmt.Sprintf(reason, args...))
}

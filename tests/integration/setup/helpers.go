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
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// TruncateAllTables truncates all tables in correct order (children first, then parents)
func TruncateAllTables(t *testing.T, db *pgxpool.Pool, ctx context.Context) {
	t.Log("Truncating all database tables...")

	tables := []string{
		// Post-related tables (children first)
		"server_post_likes",
		"server_post_comments",
		"server_posts",
		"server_post_images",
		// Server-related tables (children first)
		"server_members",
		"server_invites",
		"server_roles",
		"server_banner_images",
		"server_avatar_images",
		"servers",
		"server_categories",
		// User-related tables
		"user_avatar_images",
		"users",
	}

	for _, table := range tables {
		_, err := db.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		require.NoError(t, err, "failed to truncate table %s", table)
	}

	t.Log("All database tables truncated successfully")
}

// CreateTestWebPImage reads a real test image from testdata
// Uses itachi.jpg from tests/integration/testdata directory
func CreateTestWebPImage(t *testing.T) []byte {
	// Get the current file's directory (setup package)
	_, filename, _, _ := runtime.Caller(0)
	currentDir := filepath.Dir(filename)
	// Go up one level to integration directory, then into testdata
	imagePath := filepath.Join(currentDir, "..", "testdata", "itachi.jpg")

	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatalf("Failed to read test image: %v (path: %s)", err, imagePath)
	}

	return imageData
}

// CreateMultipartFormData creates multipart form data for file upload requests
// fieldName: form field name for the file (e.g., "image", "avatar")
// fileName: name of the file being uploaded
// fileData: binary content of the file
// fields: additional form fields (e.g., caption, content)
func CreateMultipartFormData(t *testing.T, fieldName, fileName string, fileData []byte, fields map[string]string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Determine content type based on file extension
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

	// Add file field with proper content type
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fileName))
	h.Set("Content-Type", fileContentType)
	part, err := writer.CreatePart(h)
	require.NoError(t, err, "failed to create form file field")

	_, err = part.Write(fileData)
	require.NoError(t, err, "failed to write file data")

	// Add additional text fields
	for key, value := range fields {
		err = writer.WriteField(key, value)
		require.NoError(t, err, "failed to write form field %s", key)
	}

	err = writer.Close()
	require.NoError(t, err, "failed to close multipart writer")

	contentType := writer.FormDataContentType()
	return body, contentType
}

// CreateJSONRequest creates a test request with JSON body
func CreateJSONRequest(method, url string, jsonBody []byte) *http.Request {
	req := httptest.NewRequest(method, url, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// CreateAuthRequest creates a test request with JSON body and Authorization header
func CreateAuthRequest(method, url string, jsonBody []byte, token string) *http.Request {
	req := CreateJSONRequest(method, url, jsonBody)
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	return req
}

// CreateAuthMultipartRequest creates a test request with multipart body and Authorization header
func CreateAuthMultipartRequest(method, url string, body *bytes.Buffer, contentType string, token string) *http.Request {
	req := httptest.NewRequest(method, url, body)
	req.Header.Set("Content-Type", contentType)
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	return req
}

// ParseJSONResponse helper to parse JSON response body
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

// RequireStatus checks response status code and logs body on failure
func RequireStatus(t *testing.T, resp *http.Response, expected int) {
	if resp.StatusCode != expected {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status %d, got %d. Body: %s", expected, resp.StatusCode, string(body))
		t.FailNow()
	}
}

// RequireJSONResponse checks status code and parses JSON body, logging body on failure
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

// GetAccessTokenFromResponse extracts access token from login/signup response
func GetAccessTokenFromResponse(t *testing.T, resp *http.Response) string {
	result := ParseJSONResponse(t, resp)

	data, ok := result["data"].(map[string]interface{})
	require.True(t, ok, "response data should be an object")

	accessToken, ok := data["accessToken"].(string)
	require.True(t, ok, "accessToken should be a string")
	require.NotEmpty(t, accessToken, "accessToken should not be empty")

	return accessToken
}

// GetOTPFromMailhog fetches OTP from MailHog API
// Polls MailHog API untuk email yang dikirim ke alamat tertentu
// Parse email body dan extract OTP menggunakan regex
func GetOTPFromMailhog(t *testing.T, mailhogURL, email string) string {
	t.Logf("Fetching OTP from MailHog for email: %s", email)

	// MailHog API endpoint
	apiURL := fmt.Sprintf("%s/api/v1/messages", mailhogURL)

	maxAttempts := 10 // Max 10 retries (5 seconds total)
	var otp string

	for i := 0; i < maxAttempts; i++ {
		t.Logf("Attempt %d: Fetching messages from MailHog", i+1)

		// HTTP GET ke MailHog API
		// #nosec G107 -- apiURL is a trusted localhost test server (MailHog)
		resp, err := http.Get(apiURL)
		require.NoError(t, err, "failed to fetch messages from MailHog")
		defer func() { _ = resp.Body.Close() }()

		// Parse JSON response
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "failed to read MailHog response")

		// First, let's see the raw structure
		var rawMessages []map[string]interface{}
		err = json.Unmarshal(body, &rawMessages)
		require.NoError(t, err, "failed to parse MailHog JSON response")

		t.Logf("Found %d messages in MailHog", len(rawMessages))

		// Debug: Print first message structure if available
		if len(rawMessages) > 0 {
			t.Logf("First message structure: %+v", rawMessages[0])
		}

		// Try to extract information from the raw structure
		for _, rawMsg := range rawMessages {
			// Check if this message is for the target email
			// MailHog stores the recipient info in the top-level "To" field
			matchesTargetEmail := false

			// Check top-level To field (this is where MailHog stores it)
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

			// Skip if this message is not for our target email
			if !matchesTargetEmail {
				continue
			}

			// Get email body
			var emailBody string

			// Check Content.Body
			if content, ok := rawMsg["Content"].(map[string]interface{}); ok {
				if body, ok := content["Body"].(string); ok {
					emailBody = body
				}
			}

			// If we have an email body, try to extract OTP
			if emailBody != "" {
				t.Logf("Found email for %s with body length: %d", email, len(emailBody))

				// Try multiple OTP patterns
				patterns := []string{
					`Your OTP code is:\s*(\d{6})`, // Original pattern
					`OTP code is:\s*(\d{6})`,      // Without "Your"
					`(\d{6})`,                     // Just 6 digits
					`otp.*?(\d{6})`,               // "otp" followed by 6 digits
					`code.*?(\d{6})`,              // "code" followed by 6 digits
				}

				for _, pattern := range patterns {
					re := regexp.MustCompile(`(?i)` + pattern) // Case-insensitive
					matches := re.FindStringSubmatch(emailBody)

					if len(matches) > 1 {
						otp = matches[1]
						t.Logf("OTP extracted successfully with pattern '%s': %s", pattern, otp)
						return otp
					}
				}

				// Print more body for debugging
				t.Logf("Email found but OTP pattern not matched. Body (first 500 chars): %.500s", emailBody)
			}
		}

		// Jika belum ketemu, tunggu 500ms sebelum retry
		if i < maxAttempts-1 {
			t.Logf("OTP not found yet, waiting 500ms before retry...")
			time.Sleep(500 * time.Millisecond)
		}
	}

	require.Fail(t, "OTP not found in email after %d attempts", maxAttempts)
	return ""
}

// GenerateRandomString generates a random string of specified length
// Uses lowercase letters and numbers for test data generation
func GenerateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		// #nosec G404 -- Weak randomness is acceptable for non-security test data
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// getKeys returns the keys of a map for debugging
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// APIResponse represents the standard API response structure
// Support 2 success formats:
// 1. Simple: {"status": "ok"}
// 2. With data: {"data": [...], "page": {"nextCursor": "..."}}
type APIResponse struct {
	Status string         `json:"status,omitempty"` // "ok" untuk simple success response
	Data   interface{}    `json:"data,omitempty"`   // bisa berupa array, object, atau nil
	Page   *PageInfo      `json:"page,omitempty"`   // pagination info (untuk list endpoints)
	Error  *ErrorResponse `json:"error,omitempty"`  // error info (untuk error response)
}

// PageInfo represents pagination information for list endpoints
type PageInfo struct {
	NextCursor string `json:"nextCursor"` // Cursor untuk page berikutnya
}

// ErrorResponse represents the standard error response structure
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param,omitempty"` // Param hanya ada untuk validation error
}

// ParseErrorMessage extracts error message from error response
func ParseErrorMessage(t *testing.T, result map[string]interface{}) string {
	errResp := ParseErrorResponse(t, result)
	return errResp.Message
}

// ParseErrorDetail extracts complete error details (code, message, param)
func ParseErrorDetail(t *testing.T, result map[string]interface{}) (code, message, param string) {
	errResp := ParseErrorResponse(t, result)
	return errResp.Code, errResp.Message, errResp.Param
}

// ParseErrorResponse parses error response into ErrorResponse struct
func ParseErrorResponse(t *testing.T, result map[string]interface{}) ErrorResponse {
	require.Contains(t, result, "error", "response should contain error field")

	errObj, ok := result["error"].(map[string]interface{})
	require.True(t, ok, "error field should be an object")

	errResp := ErrorResponse{}

	// Parse Code
	if code, ok := errObj["code"].(string); ok {
		errResp.Code = code
	}

	// Parse Message
	if message, ok := errObj["message"].(string); ok {
		errResp.Message = message
	}

	// Parse Param (opsional, hanya untuk validation error)
	if param, ok := errObj["param"].(string); ok {
		errResp.Param = param
	}

	return errResp
}

// ParseAPIResponse parses HTTP response into strongly-typed APIResponse struct
func ParseAPIResponse(t *testing.T, resp *http.Response) APIResponse {
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "failed to read response body")
	require.NotEmpty(t, body, "response body should not be empty")

	var apiResp APIResponse
	err = json.Unmarshal(body, &apiResp)
	require.NoError(t, err, "failed to parse JSON response")

	return apiResp
}

// IsSuccess checks if response is successful (no error field)
func IsSuccess(t *testing.T, resp APIResponse) bool {
	return resp.Error == nil
}

// GetStatus returns the status string (e.g., "ok")
func GetStatus(t *testing.T, resp APIResponse) string {
	require.NotEmpty(t, resp.Status, "response should have status field")
	return resp.Status
}

// GetDataAsMap extracts data field as map (for single object responses)
// Example: {"data": {"sessionId": "...", "otpExpiresAt": 123}}
func GetDataAsMap(t *testing.T, resp APIResponse) map[string]interface{} {
	require.NotNil(t, resp.Data, "response should have data field")
	dataMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "data field should be an object/map")
	return dataMap
}

// GetDataAsArray extracts data field as array (for list responses)
// Example: {"data": [{...}, {...}], "page": {"nextCursor": "..."}}
func GetDataAsArray(t *testing.T, resp APIResponse) []interface{} {
	require.NotNil(t, resp.Data, "response should have data field")
	dataArray, ok := resp.Data.([]interface{})
	require.True(t, ok, "data field should be an array")
	return dataArray
}

// GetNextCursor extracts pagination cursor from list responses
// Example: {"data": [], "page": {"nextCursor": "abc123"}}
func GetNextCursor(t *testing.T, resp APIResponse) string {
	require.NotNil(t, resp.Page, "response should have page field")
	return resp.Page.NextCursor
}

// HasPagination checks if response has pagination info
func HasPagination(resp APIResponse) bool {
	return resp.Page != nil
}

// ============================================================================
// COMMON TEST HELPERS
// These helpers are shared across all test modules
// ============================================================================

// CreateTestUser is a helper function to create a test user and return access token
// It performs the complete signup flow and then login to get the token
//
// Parameters:
//   - t: testing instance
//   - app: Fiber app instance
//   - mailhogURL: MailHog URL for fetching OTP
//   - email: user email for signup
//   - username: desired username
//   - password: desired password
//
// Returns:
//   - Access token string
//
// Example:
//
//	token := setup.CreateTestUser(t, app, mailhogURL, "test@example.com", "testuser", "password123")
func CreateTestUser(t *testing.T, app *fiber.App, mailhogURL, email, username, password string) string {
	// Start signup
	reqBody := []byte(fmt.Sprintf(`{"email":"%s"}`, email))
	req := CreateJSONRequest(http.MethodPost, "/api/auth/signup/start", reqBody)
	resp, err := app.Test(req)
	require.NoError(t, err, "signup start should succeed")

	result := RequireJSONResponse(t, resp, 200)
	sessionIDRaw, ok := result["sessionId"]
	require.True(t, ok, "sessionId should be in response, got: %v", result)
	sessionId, ok := sessionIDRaw.(string)
	require.True(t, ok, "sessionId should be a string, got: %T", sessionIDRaw)

	// Verify OTP
	otp := GetOTPFromMailhog(t, mailhogURL, email)
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","otp":"%s"}`, sessionId, otp))
	req = CreateJSONRequest(http.MethodPost, "/api/auth/signup/otp", reqBody)
	resp, err = app.Test(req)
	RequireStatus(t, resp, 200)

	// Set username
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","username":"%s"}`, sessionId, username))
	req = CreateJSONRequest(http.MethodPost, "/api/auth/signup/username", reqBody)
	resp, err = app.Test(req)
	RequireStatus(t, resp, 200)

	// Set password
	reqBody = []byte(fmt.Sprintf(`{"sessionId":"%s","password":"%s"}`, sessionId, password))
	req = CreateJSONRequest(http.MethodPost, "/api/auth/signup/password", reqBody)
	resp, err = app.Test(req)
	RequireStatus(t, resp, 200)

	// Login to get access token
	reqBody = []byte(fmt.Sprintf(`{"username":"%s","password":"%s"}`, username, password))
	req = CreateJSONRequest(http.MethodPost, "/api/auth/login", reqBody)
	resp, err = app.Test(req)
	require.NoError(t, err, "login should succeed")

	result = RequireJSONResponse(t, resp, 200)
	accessToken := result["accessToken"].(string)

	return accessToken
}

// CreateTestServer is a helper function to create a test server and return server ID
//
// Parameters:
//   - t: testing instance
//   - app: Fiber app instance
//   - redisURL: Redis URL (not directly used, kept for consistency)
//   - token: Access token for authentication
//   - name: Server name
//   - shortName: Server short name
//   - categoryID: Category ID
//   - isPrivate: Whether server is private
//
// Returns:
//   - Server ID string
//
// Example:
//
//	serverID := setup.CreateTestServer(t, app, redisURL, token, "My Server", "myserver", 1, false)
func CreateTestServer(t *testing.T, app *fiber.App, redisURL, token, name, shortName string, categoryID int, isPrivate bool) string {
	reqBody := []byte(fmt.Sprintf(`{
		"name": "%s",
		"shortName": "%s",
		"categoryId": %d,
		"description": "Test server description",
		"settings": {"isPrivate": %t}
	}`, name, shortName, categoryID, isPrivate))

	req := CreateAuthRequest(http.MethodPost, "/api/servers/create", reqBody, token)
	resp, err := app.Test(req)
	require.NoError(t, err, "create server should succeed")

	result := RequireJSONResponse(t, resp, 200)
	serverID := result["id"].(string)
	require.NotEmpty(t, serverID, "server id should not be empty")

	return serverID
}

// ============================================================================
// PARALLEL TEST SETUP WITH SINGLETON INFRASTRUCTURE
// These helpers use the global singleton infrastructure for parallel tests
// Each test gets its own database for complete isolation
// ============================================================================

// SetupParallelTest creates a test app using the global singleton infrastructure
// This is the main entry point for parallel tests - no container startup needed
// Each test gets its own database for complete isolation
func SetupParallelTest(t *testing.T) (*fiber.App, *pgxpool.Pool, *redis.Client, *minio.Client) {
 t.Log("Setting up parallel test with singleton infrastructure...")

 // Ensure singleton infrastructure is initialized
 // This uses sync.Once internally, so it's safe to call multiple times
 if err := EnsureSingletonInitialized(); err != nil {
  t.Fatalf("Failed to initialize singleton infrastructure: %v", err)
 }

 // Get global singleton infrastructure
 infra := GetGlobalInfra()
 if infra == nil {
  t.Fatal("Global infrastructure not initialized after EnsureSingletonInitialized()")
 }

 // Create a unique database for this test using test name
 // This provides isolation for parallel tests
 testDBName := sanitizeTestName(t.Name())
 createTestDatabase(t, infra.PgURL, testDBName)

 // Create connection string for the test database
 testPgURL := replaceDBName(infra.PgURL, testDBName)

 // Setup test app with isolated database
 app, db, redisClient, minioClient := SetupTestApp(t, testPgURL, infra.RedisURL, infra.MinioURL, infra.MailhogSMTP)

 // Cleanup: Drop test database after test completes
 t.Cleanup(func() {
  if db != nil {
   db.Close()
  }
  dropTestDatabase(t, infra.PgURL, testDBName)
 })

 return app, db, redisClient, minioClient
}

// sanitizeTestName converts a test name to a valid database name
func sanitizeTestName(testName string) string {
 // Replace special characters with underscores
 name := testName
 // Remove leading slash if present
 if strings.HasPrefix(name, "/") {
  name = name[1:]
 }
 // Replace slashes and other special chars
 replacer := strings.NewReplacer(
  "/", "_",
  "\\", "_",
  " ", "_",
  "-", "_",
  ".", "_",
 )
 name = replacer.Replace(name)
 // Limit length and add prefix
 if len(name) > 40 {
  name = name[:40]
 }
 return "test_" + name
}

// replaceDBName replaces the database name in a PostgreSQL connection string
func replaceDBName(pgURL, newDBName string) string {
 // Parse connection string and replace database name
 // Format: postgres://user:pass@host:port/dbname?options
 parts := strings.Split(pgURL, "/")
 if len(parts) >= 5 {
  parts[len(parts)-1] = newDBName
  // Add back query params if any
  if idx := strings.Index(newDBName, "?"); idx > 0 {
   parts[len(parts)-1] = newDBName
  }
  return strings.Join(parts, "/")
 }
 // Fallback: try to replace the last part
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

// createTestDatabase creates a new database for the test
func createTestDatabase(t *testing.T, pgURL, dbName string) {
 ctx := context.Background()
 // Connect to default postgres database to create new database
 adminURL := replaceDBName(pgURL, "postgres")
 pool, err := pgxpool.New(ctx, adminURL)
 require.NoError(t, err, "failed to connect to admin database")
 defer pool.Close()

 // First, drop database if exists (from previous failed test)
 _, _ = pool.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName))

 // Create database with proper quoting
 _, err = pool.Exec(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, dbName))
 require.NoError(t, err, "failed to create test database %s", dbName)

 t.Logf("Created test database: %s", dbName)

 // Close the admin pool to ensure connections are released
 pool.Close()

 // Run migrations on the new database
 testPgURL := replaceDBName(pgURL, dbName)
 if err := RunMigration(testPgURL, t); err != nil {
  t.Fatalf("failed to run migrations on test database %s: %v", dbName, err)
 }
}

// dropTestDatabase drops the test database
func dropTestDatabase(t *testing.T, pgURL, dbName string) {
 ctx := context.Background()
 // Connect to default postgres database to drop test database
 adminURL := replaceDBName(pgURL, "postgres")
 pool, err := pgxpool.New(ctx, adminURL)
 if err != nil {
  t.Logf("Warning: failed to connect to admin database for cleanup: %v", err)
  return
 }
 defer pool.Close()

 // Drop database with proper quoting
 // Also need to drop connections first
 _, _ = pool.Exec(ctx, fmt.Sprintf(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND pid <> pg_backend_pid()`, dbName))
 _, err = pool.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName))
 if err != nil {
  t.Logf("Warning: failed to drop test database %s: %v", dbName, err)
  return
 }

 t.Logf("Dropped test database: %s", dbName)
}

// mailhogSMTPSplit splits the mailhog SMTP string into host and port
func mailhogSMTPSplit(smtpSMTP string) struct{ host, port string } {
 parts := strings.Split(smtpSMTP, ":")
 host := parts[0]
 port := "1025"
 if len(parts) > 1 {
  port = parts[1]
 }
 return struct{ host, port string }{host: host, port: port}
}

// ============================================================================
// HTTP REQUEST/RESPONSE LOGGING
// These helpers provide detailed logging for debugging API calls
// ============================================================================

// LoggedRequest wraps an HTTP request with logging
type LoggedRequest struct {
 Method    string
 URL       string
 Header    http.Header
 Body      string
 Token     string
}

// LoggedResponse wraps an HTTP response with logging
type LoggedResponse struct {
 StatusCode int
 Header     http.Header
 Body       string
}

// LogHTTPRequest logs the details of an HTTP request
func LogHTTPRequest(t *testing.T, req *http.Request) {
 loggedReq := LoggedRequest{
  Method: req.Method,
  URL:    req.URL.String(),
  Header: req.Header,
  Token:  req.Header.Get("Authorization"),
 }

 // Read body if present
 if req.Body != nil && req.Body != http.NoBody {
  bodyBytes, _ := io.ReadAll(req.Body)
  loggedReq.Body = string(bodyBytes)
  // Restore body for subsequent reads
  req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
 }

 t.Logf(">>> HTTP REQUEST <<<")
 t.Logf("  Method: %s", loggedReq.Method)
 t.Logf("  URL:    %s", loggedReq.URL)

 // Log headers (excluding very long ones)
 t.Logf("  Headers:")
 for key, values := range loggedReq.Header {
  if key == "Authorization" && len(values) > 0 && len(values[0]) > 50 {
   t.Logf("    %s: Bearer *** (truncated)", key)
  } else {
   t.Logf("    %s: %s", key, values)
  }
 }

 if loggedReq.Body != "" {
  // Truncate body if too long
  bodyPreview := loggedReq.Body
  if len(bodyPreview) > 500 {
   bodyPreview = bodyPreview[:500] + "... (truncated)"
  }
  t.Logf("  Body:   %s", bodyPreview)
 }
 t.Logf(">>> END REQUEST <<<")
}

// LogHTTPResponse logs the details of an HTTP response
func LogHTTPResponse(t *testing.T, resp *http.Response) {
 loggedResp := LoggedResponse{
  StatusCode: resp.StatusCode,
  Header:     resp.Header,
 }

 // Read body
 bodyBytes, err := io.ReadAll(resp.Body)
 if err == nil {
  loggedResp.Body = string(bodyBytes)
  // Restore body for subsequent reads
  resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
 }

 t.Logf("<<< HTTP RESPONSE <<<")
 t.Logf("  Status: %d %s", loggedResp.StatusCode, resp.Status)

 // Log key headers
 t.Logf("  Headers:")
 for key, values := range loggedResp.Header {
  t.Logf("    %s: %s", key, values)
 }

 if loggedResp.Body != "" {
  // Truncate body if too long
  bodyPreview := loggedResp.Body
  if len(bodyPreview) > 500 {
   bodyPreview = bodyPreview[:500] + "... (truncated)"
  }
  t.Logf("  Body:   %s", bodyPreview)
 }
 t.Logf("<<< END RESPONSE <<<")

 // Also log error status
 if loggedResp.StatusCode >= 400 {
  t.Logf("⚠️  ERROR RESPONSE: Status %d", loggedResp.StatusCode)
 }
}

// LogTestRequest logs both request and response with test context
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

// TestRequestWithLogging executes an HTTP request with full logging
func TestRequestWithLogging(t *testing.T, app *fiber.App, req *http.Request) (*http.Response, error) {
 t.Logf("--- Executing Request: %s %s ---", req.Method, req.URL.Path)
 LogHTTPRequest(t, req)

 resp, err := app.Test(req)

 if err != nil {
  t.Logf("❌ Request Error: %v", err)
  return nil, err
 }

 LogHTTPResponse(t, resp)

 // Log success/error indication
 if resp.StatusCode >= 200 && resp.StatusCode < 300 {
  t.Logf("✅ Request Success: %d", resp.StatusCode)
 } else if resp.StatusCode >= 400 {
  t.Logf("⚠️  Request Error Response: %d", resp.StatusCode)
 }

 return resp, nil
}

// ============================================================================
// ENHANCED ASSERTION HELPERS WITH LOGGING
// These helpers provide better error messages with request/response logging
// ============================================================================

// RequireStatusWithLog checks response status and logs full request/response on failure
// This replaces RequireStatus for better debugging
func RequireStatusWithLog(t *testing.T, req *http.Request, resp *http.Response, expected int) {
 if resp.StatusCode != expected {
  // Log the request that caused the failure
  t.Logf("❌ Status Code Mismatch!")
  t.Logf("   Expected: %d, Got: %d", expected, resp.StatusCode)

  // Log full request details
  LogHTTPRequest(t, req)

  // Read and log response body
  bodyBytes, _ := io.ReadAll(resp.Body)
  if len(bodyBytes) > 0 {
   bodyPreview := string(bodyBytes)
   if len(bodyPreview) > 500 {
    bodyPreview = bodyPreview[:500] + "... (truncated)"
   }
   t.Logf("❌ Response Body:")
   t.Logf("   %s", bodyPreview)
  }

  // Restore body for subsequent reads
  resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

  t.Fatalf("Expected status %d, got %d", expected, resp.StatusCode)
 }
}

// RequireJSONWithLog executes a request, checks status, and parses JSON
// Returns parsed JSON and logs full details on any error
func RequireJSONWithLog(t *testing.T, app *fiber.App, req *http.Request, expectedStatus int) map[string]interface{} {
 t.Logf("--- Executing Request: %s %s ---", req.Method, req.URL.Path)
 LogHTTPRequest(t, req)

 resp, err := app.Test(req)
 if err != nil {
  t.Logf("❌ Request Failed: %v", err)
  t.Fatalf("Request failed: %v", err)
 }

 // Read body before checking status
 bodyBytes, _ := io.ReadAll(resp.Body)
 resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

 // Check status first
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

 // Parse JSON
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

 // Log success
 t.Logf("✅ Request Successful: %d", resp.StatusCode)
 if len(bodyBytes) < 500 {
  t.Logf("   Response: %s", string(bodyBytes))
 } else {
  t.Logf("   Response: %s... (truncated)", string(bodyBytes[:500]))
 }

 return result
}

// AssertNoError is a helper that logs details before calling require.NoError
func AssertNoError(t *testing.T, err error, msg string, args ...interface{}) {
 if err != nil {
  t.Logf("❌ Error: %s", fmt.Sprintf(msg, args...))
  t.Logf("   Error details: %v", err)
  require.NoError(t, err, fmt.Sprintf(msg, args...))
 }
}

// LogTestStep logs a test step for better traceability
func LogTestStep(t *testing.T, step string, args ...interface{}) {
 t.Logf("📌 %s", fmt.Sprintf(step, args...))
}

// LogTestStart marks the start of a test case
func LogTestStart(t *testing.T, testName string) {
 t.Logf("========================================")
 t.Logf("🧪 Starting: %s", testName)
 t.Logf("========================================")
}

// LogTestPass marks a test as passed
func LogTestPass(t *testing.T, testName string) {
 t.Logf("✅ %s PASSED", testName)
}

// LogTestFail marks a test as failed with reason
func LogTestFail(t *testing.T, testName, reason string, args ...interface{}) {
 t.Logf("❌ %s FAILED", testName)
 t.Logf("   Reason: %s", fmt.Sprintf(reason, args...))
}

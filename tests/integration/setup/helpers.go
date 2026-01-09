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
	"regexp"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

// CreateTestWebPImage creates a minimal valid WebP image for testing
// This is a 1x1 pixel transparent WebP image in VP8L format
func CreateTestWebPImage(t *testing.T) []byte {
	// Minimal valid WebP VP8L header for a 1x1 transparent image
	// RIFF + WEBP + VP8L chunk
	webpData := []byte{
		// "RIFF"
		0x52, 0x49, 0x46, 0x46,
		// File size (little endian)
		0x1A, 0x00, 0x00, 0x00,
		// "WEBP"
		0x57, 0x45, 0x42, 0x50,
		// "VP8L"
		0x56, 0x50, 0x38, 0x4C,
		// Chunk size (little endian)
		0x18, 0x00, 0x00, 0x00,
		// VP8L data: 1x1 image, no alpha, version 1
		0x2F, 0x07, 0x10, 0x58,
		// Rest of VP8L data (green pixel)
		0x58, 0x10, 0x00, 0x00,
	}

	return webpData
}

// CreateMultipartFormData creates multipart form data for file upload requests
// fieldName: form field name for the file (e.g., "image", "avatar")
// fileName: name of the file being uploaded
// fileData: binary content of the file
// fields: additional form fields (e.g., caption, content)
func CreateMultipartFormData(t *testing.T, fieldName, fileName string, fileData []byte, fields map[string]string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file field
	part, err := writer.CreateFormFile(fieldName, fileName)
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
	require.NoError(t, err, "failed to parse JSON response")

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
				t.Logf("Found email with body length: %d", len(emailBody))

				// Try multiple OTP patterns
				patterns := []string{
					`Your OTP code is:\s*(\d{6})`,                                    // Original pattern
					`OTP code is:\s*(\d{6})`,                                        // Without "Your"
					`(\d{6})`,                                                      // Just 6 digits
					`otp.*?(\d{6})`,                                                // "otp" followed by 6 digits
					`code.*?(\d{6})`,                                               // "code" followed by 6 digits
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

// APIResponse represents the standard API response structure
// Support 2 success formats:
// 1. Simple: {"status": "ok"}
// 2. With data: {"data": [...], "page": {"nextCursor": "..."}}
type APIResponse struct {
	Status string      `json:"status,omitempty"` // "ok" untuk simple success response
	Data   interface{} `json:"data,omitempty"`    // bisa berupa array, object, atau nil
	Page   *PageInfo    `json:"page,omitempty"`    // pagination info (untuk list endpoints)
	Error  *ErrorResponse `json:"error,omitempty"` // error info (untuk error response)
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

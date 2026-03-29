package setup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// APIClient is a fluent wrapper for Fiber app testing
// Provides a more intuitive and chainable API for testing HTTP endpoints
//
// Example:
//
//	client := setup.NewAPIClient(t, app)
//	resp := client.Post("/api/auth/signup/start", map[string]any{"email": "test@example.com"})
//	resp.AssertStatus(200)
//	sessionId := resp.GetData().GetString("sessionId")
type APIClient struct {
	t        *testing.T
	app      *fiber.App
	token    string
	baseURL  string
}

// NewAPIClient creates a new API client for testing
func NewAPIClient(t *testing.T, app *fiber.App) *APIClient {
	return &APIClient{
		t:       t,
		app:     app,
		baseURL: "/api",
	}
}

// WithAuth sets the authorization token for subsequent requests
// Returns the client for chaining
func (c *APIClient) WithAuth(token string) *APIClient {
	c.token = token
	return c
}

// Post sends a POST request with JSON body
// Returns a TestResponse for assertion chaining
func (c *APIClient) Post(path string, body any) *TestResponse {
	jsonBody := mustMarshalJSON(c.t, body)
	req := c.createRequest(http.MethodPost, path, jsonBody)
	return c.executeRequest(req)
}

// Get sends a GET request
func (c *APIClient) Get(path string) *TestResponse {
	req := c.createRequest(http.MethodGet, path, nil)
	return c.executeRequest(req)
}

// Put sends a PUT request with JSON body
func (c *APIClient) Put(path string, body any) *TestResponse {
	jsonBody := mustMarshalJSON(c.t, body)
	req := c.createRequest(http.MethodPut, path, jsonBody)
	return c.executeRequest(req)
}

// Delete sends a DELETE request
func (c *APIClient) Delete(path string) *TestResponse {
	req := c.createRequest(http.MethodDelete, path, nil)
	return c.executeRequest(req)
}

// PutMultipart sends a PUT request with multipart form data (for file uploads)
func (c *APIClient) PutMultipart(path string, body *bytes.Buffer, contentType string) *TestResponse {
	req := httptest.NewRequest(http.MethodPut, c.baseURL+path, body)
	req.Header.Set("Content-Type", contentType)
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}
	return c.executeRequest(req)
}

// createRequest creates an HTTP request with proper headers
func (c *APIClient) createRequest(method, path string, body []byte) *http.Request {
	url := c.baseURL + path
	req := httptest.NewRequest(method, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}
	return req
}

// executeRequest executes the request and returns a TestResponse
func (c *APIClient) executeRequest(req *http.Request) *TestResponse {
	resp, err := c.app.Test(req)
	require.NoError(c.t, err, "request should execute successfully")
	return NewTestResponse(c.t, resp)
}

// mustMarshalJSON marshals any value to JSON, failing the test on error
func mustMarshalJSON(t *testing.T, v any) []byte {
	body, err := json.Marshal(v)
	require.NoError(t, err, "failed to marshal JSON body")
	return body
}

// TestResponse provides a fluent interface for asserting HTTP responses
//
// Example:
//
//	resp := client.Post("/api/auth/login", credentials)
//	resp.AssertStatus(200).
//	    AssertHasData().
//	    AssertDataContains("accessToken")
type TestResponse struct {
	t           *testing.T
	httpResp    *http.Response
	parsedBody  map[string]any
	parsed      bool
}

// NewTestResponse creates a new TestResponse from an HTTP response
func NewTestResponse(t *testing.T, resp *http.Response) *TestResponse {
	return &TestResponse{
		t:        t,
		httpResp: resp,
		parsed:   false,
	}
}

// AssertStatus asserts the HTTP status code
// Returns the response for chaining
func (r *TestResponse) AssertStatus(expected int) *TestResponse {
	require.Equal(r.t, expected, r.httpResp.StatusCode, "status code mismatch")
	return r
}

// AssertError asserts that the response contains an error
func (r *TestResponse) AssertError() *TestResponse {
	r.ensureParsed()
	require.Contains(r.t, r.parsedBody, "error", "response should contain error field")
	return r
}

// AssertNoError asserts that the response does NOT contain an error
func (r *TestResponse) AssertNoError() *TestResponse {
	r.ensureParsed()
	require.NotContains(r.t, r.parsedBody, "error", "response should not contain error field")
	return r
}

// AssertErrorMessage asserts the error message contains the expected text
func (r *TestResponse) AssertErrorMessage(expectedMsg string) *TestResponse {
	r.ensureParsed()
	errMsg := ParseErrorMessage(r.t, r.parsedBody)
	require.Contains(r.t, errMsg, expectedMsg, "error message should contain expected text")
	return r
}

// AssertHasData asserts that the response has a data field
func (r *TestResponse) AssertHasData() *TestResponse {
	r.ensureParsed()
	require.Contains(r.t, r.parsedBody, "data", "response should contain data field")
	return r
}

// AssertDataContains asserts that the data field contains the specified key
func (r *TestResponse) AssertDataContains(key string) *TestResponse {
	r.ensureParsed()
	data := GetDataAsMap(r.t, ParseAPIResponse(r.t, r.httpResp))
	require.Contains(r.t, data, key, fmt.Sprintf("data should contain key: %s", key))
	return r
}

// GetData returns the data field as a map for further assertions
func (r *TestResponse) GetData() *DataWrapper {
	r.ensureParsed()
	return &DataWrapper{
		t:    r.t,
		data: GetDataAsMap(r.t, ParseAPIResponse(r.t, r.httpResp)),
	}
}

// GetBody returns the entire parsed response body
func (r *TestResponse) GetBody() map[string]any {
	r.ensureParsed()
	return r.parsedBody
}

// GetToken extracts and returns the access token from the response
// Convenience method for auth flows
func (r *TestResponse) GetToken() string {
	data := r.GetData()
	return data.GetString("accessToken")
}

// ensureParsed lazily parses the response body on first access
func (r *TestResponse) ensureParsed() {
	if r.parsed {
		return
	}
	r.parsedBody = ParseJSONResponse(r.t, r.httpResp)
	r.parsed = true
}

// DataWrapper provides fluent access to response data fields
//
// Example:
//
//	data := resp.GetData()
//	sessionId := data.GetString("sessionId")
//	expiresAt := data.GetInt("otpExpiresAt")
type DataWrapper struct {
	t    *testing.T
	data map[string]any
}

// GetString returns a string field from the data
func (w *DataWrapper) GetString(key string) string {
	val, ok := w.data[key]
	require.True(w.t, ok, fmt.Sprintf("data should contain key: %s", key))
	strVal, ok := val.(string)
	require.True(w.t, ok, fmt.Sprintf("key '%s' should be a string", key))
	return strVal
}

// GetInt returns an int field from the data
func (w *DataWrapper) GetInt(key string) int {
	val, ok := w.data[key]
	require.True(w.t, ok, fmt.Sprintf("data should contain key: %s", key))

	// Handle both int and float64 (JSON numbers are float64)
	switch v := val.(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		require.Fail(w.t, fmt.Sprintf("key '%s' should be a number", key))
		return 0
	}
}

// GetBool returns a bool field from the data
func (w *DataWrapper) GetBool(key string) bool {
	val, ok := w.data[key]
	require.True(w.t, ok, fmt.Sprintf("data should contain key: %s", key))
	boolVal, ok := val.(bool)
	require.True(w.t, ok, fmt.Sprintf("key '%s' should be a bool", key))
	return boolVal
}

// Has checks if a key exists in the data
func (w *DataWrapper) Has(key string) bool {
	_, ok := w.data[key]
	return ok
}

// All returns the underlying data map
func (w *DataWrapper) All() map[string]any {
	return w.data
}

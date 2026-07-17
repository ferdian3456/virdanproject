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

type APIClient struct {
	t       *testing.T
	app     *fiber.App
	token   string
	baseURL string
}

func NewAPIClient(t *testing.T, app *fiber.App) *APIClient {
	return &APIClient{
		t:       t,
		app:     app,
		baseURL: "/api",
	}
}

func (c *APIClient) WithAuth(token string) *APIClient {
	c.token = token
	return c
}

func (c *APIClient) Post(path string, body any) *TestResponse {
	jsonBody := mustMarshalJSON(c.t, body)
	req := c.createRequest(http.MethodPost, path, jsonBody)
	return c.executeRequest(req)
}

func (c *APIClient) Get(path string) *TestResponse {
	req := c.createRequest(http.MethodGet, path, nil)
	return c.executeRequest(req)
}

func (c *APIClient) Put(path string, body any) *TestResponse {
	jsonBody := mustMarshalJSON(c.t, body)
	req := c.createRequest(http.MethodPut, path, jsonBody)
	return c.executeRequest(req)
}

func (c *APIClient) Delete(path string) *TestResponse {
	req := c.createRequest(http.MethodDelete, path, nil)
	return c.executeRequest(req)
}

func (c *APIClient) PutMultipart(path string, body *bytes.Buffer, contentType string) *TestResponse {
	req := httptest.NewRequest(http.MethodPut, c.baseURL+path, body)
	req.Header.Set("Content-Type", contentType)
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}
	return c.executeRequest(req)
}

func (c *APIClient) createRequest(method, path string, body []byte) *http.Request {
	url := c.baseURL + path
	req := httptest.NewRequest(method, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}
	return req
}

func (c *APIClient) executeRequest(req *http.Request) *TestResponse {
	resp, err := c.app.Test(req)
	require.NoError(c.t, err, "request should execute successfully")
	return NewTestResponse(c.t, resp)
}

func mustMarshalJSON(t *testing.T, v any) []byte {
	body, err := json.Marshal(v)
	require.NoError(t, err, "failed to marshal JSON body")
	return body
}

type TestResponse struct {
	t          *testing.T
	httpResp   *http.Response
	parsedBody map[string]any
	parsed     bool
}

func NewTestResponse(t *testing.T, resp *http.Response) *TestResponse {
	return &TestResponse{
		t:        t,
		httpResp: resp,
		parsed:   false,
	}
}

func (r *TestResponse) AssertStatus(expected int) *TestResponse {
	require.Equal(r.t, expected, r.httpResp.StatusCode, "status code mismatch")
	return r
}

func (r *TestResponse) AssertError() *TestResponse {
	r.ensureParsed()
	require.Contains(r.t, r.parsedBody, "error", "response should contain error field")
	return r
}

func (r *TestResponse) AssertNoError() *TestResponse {
	r.ensureParsed()
	require.NotContains(r.t, r.parsedBody, "error", "response should not contain error field")
	return r
}

func (r *TestResponse) AssertErrorMessage(expectedMsg string) *TestResponse {
	r.ensureParsed()
	errMsg := ParseErrorMessage(r.t, r.parsedBody)
	require.Contains(r.t, errMsg, expectedMsg, "error message should contain expected text")
	return r
}

func (r *TestResponse) AssertHasData() *TestResponse {
	r.ensureParsed()
	require.Contains(r.t, r.parsedBody, "data", "response should contain data field")
	return r
}

func (r *TestResponse) AssertDataContains(key string) *TestResponse {
	r.ensureParsed()
	data := GetDataAsMap(r.t, ParseAPIResponse(r.t, r.httpResp))
	require.Contains(r.t, data, key, fmt.Sprintf("data should contain key: %s", key))
	return r
}

func (r *TestResponse) GetData() *DataWrapper {
	r.ensureParsed()
	return &DataWrapper{
		t:    r.t,
		data: GetDataAsMap(r.t, ParseAPIResponse(r.t, r.httpResp)),
	}
}

func (r *TestResponse) GetBody() map[string]any {
	r.ensureParsed()
	return r.parsedBody
}

func (r *TestResponse) GetToken() string {
	data := r.GetData()
	return data.GetString("accessToken")
}

func (r *TestResponse) ensureParsed() {
	if r.parsed {
		return
	}
	r.parsedBody = ParseJSONResponse(r.t, r.httpResp)
	r.parsed = true
}

type DataWrapper struct {
	t    *testing.T
	data map[string]any
}

func (w *DataWrapper) GetString(key string) string {
	val, ok := w.data[key]
	require.True(w.t, ok, fmt.Sprintf("data should contain key: %s", key))
	strVal, ok := val.(string)
	require.True(w.t, ok, fmt.Sprintf("key '%s' should be a string", key))
	return strVal
}

func (w *DataWrapper) GetInt(key string) int {
	val, ok := w.data[key]
	require.True(w.t, ok, fmt.Sprintf("data should contain key: %s", key))

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

func (w *DataWrapper) GetBool(key string) bool {
	val, ok := w.data[key]
	require.True(w.t, ok, fmt.Sprintf("data should contain key: %s", key))
	boolVal, ok := val.(bool)
	require.True(w.t, ok, fmt.Sprintf("key '%s' should be a bool", key))
	return boolVal
}

func (w *DataWrapper) Has(key string) bool {
	_, ok := w.data[key]
	return ok
}

func (w *DataWrapper) All() map[string]any {
	return w.data
}

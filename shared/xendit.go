package shared

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/knadh/koanf/v2"
	"go.uber.org/zap"
)

type XenditClient struct {
	baseURL    string
	secretKey  string
	successURL string
	cancelURL  string
	httpClient *http.Client
	log        *zap.Logger
}

func NewXenditClient(config *koanf.Koanf, log *zap.Logger) *XenditClient {
	return &XenditClient{
		baseURL:    config.String("XENDIT_API_BASE_URL"),
		secretKey:  config.String("XENDIT_SECRET_KEY"),
		successURL: config.String("XENDIT_SUCCESS_URL"),
		cancelURL:  config.String("XENDIT_CANCEL_URL"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
		log:        log,
	}
}

func (c *XenditClient) doRequest(ctx context.Context, method, path string, reqBody any) ([]byte, int, error) {
	var bodyReader io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return nil, 0, fmt.Errorf("xendit: marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("xendit: build request: %w", err)
	}

	req.SetBasicAuth(c.secretKey, "")
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("xendit: http do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("xendit: read response body: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

func (c *XenditClient) CreatePaymentSession(ctx context.Context, referenceId, description string, amount int64) (string, string, error) {
	payload := map[string]any{
		"reference_id":       referenceId,
		"session_type":       "PAY",
		"mode":               "PAYMENT_LINK",
		"amount":             amount,
		"currency":           "IDR",
		"country":            "ID",
		"success_return_url": c.successURL,
		"cancel_return_url":  c.cancelURL,
		"description":        description,
	}

	respBody, statusCode, err := c.doRequest(ctx, http.MethodPost, "/sessions", payload)
	if err != nil {
		return "", "", err
	}

	if statusCode < 200 || statusCode >= 300 {
		return "", "", fmt.Errorf("xendit: CreatePaymentSession status %d: %s", statusCode, string(respBody))
	}

	var result struct {
		PaymentSessionID string `json:"payment_session_id"`
		PaymentLinkURL   string `json:"payment_link_url"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", fmt.Errorf("xendit: parse CreatePaymentSession response: %w", err)
	}
	if result.PaymentLinkURL == "" {
		return "", "", fmt.Errorf("xendit: empty payment_link_url in response: %s", string(respBody))
	}

	return result.PaymentSessionID, result.PaymentLinkURL, nil
}

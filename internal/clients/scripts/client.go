package scripts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Response represents a proxied response from the script service.
type Response struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}

// Client is a thin HTTP wrapper around the Python llm-script-service API.
type Client struct {
	baseURL string
	http    *http.Client
}

// New creates a new client with the provided baseURL and timeout.
func New(baseURL string, timeout time.Duration) (*Client, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("baseURL is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid baseURL: %w", err)
	}
	if parsed.Scheme == "" {
		return nil, fmt.Errorf("baseURL must include scheme (http/https)")
	}

	return &Client{
		baseURL: strings.TrimRight(parsed.String(), "/"),
		http:    &http.Client{Timeout: timeout},
	}, nil
}

func (c *Client) CreateScript(ctx context.Context, payload []byte) (*Response, error) {
	return c.do(ctx, http.MethodPost, c.baseURL+"/scripts", payload)
}

func (c *Client) ListScripts(ctx context.Context) (*Response, error) {
	return c.do(ctx, http.MethodGet, c.baseURL+"/scripts", nil)
}

func (c *Client) do(ctx context.Context, method, endpoint string, payload []byte) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("script service request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read script service response: %w", err)
	}
	return &Response{StatusCode: resp.StatusCode, Body: body, Header: resp.Header.Clone()}, nil
}

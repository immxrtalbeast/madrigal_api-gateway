package videos

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

type Response struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}

type Client struct {
	baseURL string
	http    *http.Client
}

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

func (c *Client) CreateVideo(ctx context.Context, payload []byte) (*Response, error) {
	return c.do(ctx, http.MethodPost, c.baseURL+"/videos", payload)
}

func (c *Client) ListVideos(ctx context.Context) (*Response, error) {
	return c.do(ctx, http.MethodGet, c.baseURL+"/videos", nil)
}

func (c *Client) GetVideo(ctx context.Context, videoID string) (*Response, error) {
	if videoID == "" {
		return nil, fmt.Errorf("videoID is required")
	}
	return c.do(ctx, http.MethodGet, c.baseURL+"/videos/"+videoID, nil)
}

func (c *Client) ExpandIdea(ctx context.Context, payload []byte) (*Response, error) {
	return c.do(ctx, http.MethodPost, c.baseURL+"/ideas:expand", payload)
}

func (c *Client) ApproveDraft(ctx context.Context, videoID string, payload []byte) (*Response, error) {
	if videoID == "" {
		return nil, fmt.Errorf("videoID is required")
	}
	return c.do(ctx, http.MethodPost, c.baseURL+"/videos/"+videoID+"/draft:approve", payload)
}

func (c *Client) UploadMedia(ctx context.Context, payload []byte) (*Response, error) {
	return c.do(ctx, http.MethodPost, c.baseURL+"/media", payload)
}

func (c *Client) ListMedia(ctx context.Context, folder string) (*Response, error) {
	endpoint := c.baseURL + "/media"
	if folder != "" {
		endpoint = endpoint + "?folder=" + url.QueryEscape(folder)
	}
	return c.do(ctx, http.MethodGet, endpoint, nil)
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
		return nil, fmt.Errorf("video service request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read video service response: %w", err)
	}
	return &Response{
		StatusCode: resp.StatusCode,
		Body:       body,
		Header:     resp.Header.Clone(),
	}, nil
}

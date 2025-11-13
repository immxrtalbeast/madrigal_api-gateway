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

func (c *Client) CreateVideo(ctx context.Context, payload []byte, headers map[string]string) (*Response, error) {
	return c.do(ctx, http.MethodPost, c.baseURL+"/videos", payload, headers)
}

func (c *Client) ListVideos(ctx context.Context, headers map[string]string) (*Response, error) {
	return c.do(ctx, http.MethodGet, c.baseURL+"/videos", nil, headers)
}

func (c *Client) GetVideo(ctx context.Context, videoID string, headers map[string]string) (*Response, error) {
	if videoID == "" {
		return nil, fmt.Errorf("videoID is required")
	}
	return c.do(ctx, http.MethodGet, c.baseURL+"/videos/"+videoID, nil, headers)
}

func (c *Client) ExpandIdea(ctx context.Context, payload []byte, headers map[string]string) (*Response, error) {
	return c.do(ctx, http.MethodPost, c.baseURL+"/ideas:expand", payload, headers)
}

func (c *Client) ApproveDraft(ctx context.Context, videoID string, payload []byte, headers map[string]string) (*Response, error) {
	if videoID == "" {
		return nil, fmt.Errorf("videoID is required")
	}
	return c.do(ctx, http.MethodPost, c.baseURL+"/videos/"+videoID+"/draft:approve", payload, headers)
}

func (c *Client) ApproveSubtitles(ctx context.Context, videoID string, payload []byte, headers map[string]string) (*Response, error) {
	if videoID == "" {
		return nil, fmt.Errorf("videoID is required")
	}
	return c.do(ctx, http.MethodPost, c.baseURL+"/videos/"+videoID+"/subtitles:approve", payload, headers)
}

func (c *Client) UploadMedia(ctx context.Context, payload []byte, headers map[string]string) (*Response, error) {
	return c.do(ctx, http.MethodPost, c.baseURL+"/media", payload, headers)
}

func (c *Client) ListMedia(ctx context.Context, folder string, headers map[string]string) (*Response, error) {
	endpoint := c.baseURL + "/media"
	if folder != "" {
		endpoint = endpoint + "?folder=" + url.QueryEscape(folder)
	}
	return c.do(ctx, http.MethodGet, endpoint, nil, headers)
}

func (c *Client) ListSharedMedia(ctx context.Context, folder string) (*Response, error) {
	endpoint := c.baseURL + "/media/shared"
	if folder != "" {
		endpoint = endpoint + "?folder=" + url.QueryEscape(folder)
	}
	return c.do(ctx, http.MethodGet, endpoint, nil, nil)
}

func (c *Client) ListVoices(ctx context.Context) (*Response, error) {
	return c.do(ctx, http.MethodGet, c.baseURL+"/voices", nil, nil)
}

func (c *Client) ListMusic(ctx context.Context) (*Response, error) {
	return c.do(ctx, http.MethodGet, c.baseURL+"/music", nil, nil)
}

func (c *Client) do(ctx context.Context, method, endpoint string, payload []byte, extraHeaders map[string]string) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range extraHeaders {
		if value == "" {
			continue
		}
		req.Header.Set(key, value)
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

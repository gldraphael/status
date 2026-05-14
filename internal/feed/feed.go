package feed

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client fetches raw HTTP feed bodies.
type Client struct {
	url        string
	httpClient *http.Client
}

// NewClient creates a feed client with the provided timeout.
func NewClient(url string, timeout time.Duration) (*Client, error) {
	if url == "" {
		return nil, fmt.Errorf("feed URL is required")
	}
	return &Client{
		url:        url,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

// Fetch fetches the configured feed body.
func (c *Client) Fetch(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch feed: unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return body, nil
}

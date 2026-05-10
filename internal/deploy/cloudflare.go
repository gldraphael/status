package deploy

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// HookClient triggers a Cloudflare Pages build using the Pages Build Hook URL.
// The hook URL is expected to be an absolute HTTPS URL provided via config/env.
type HookClient struct {
	HookURL    string
	httpClient *http.Client
}

// NewHookClient returns a HookClient with a default HTTP client.
func NewHookClient(hookURL string) *HookClient {
	return &HookClient{
		HookURL:    hookURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Trigger posts to the configured hook URL. It treats any non-2xx response as an error.
func (c *HookClient) Trigger(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.HookURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	// Pages build hooks accept an empty POST; set a JSON content-type for clarity.
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

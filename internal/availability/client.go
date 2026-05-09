package availability

import (
	"context"
	"fmt"

	"github.com/gldraphael/status/internal/calendar"
)

// Client fetches the raw availability calendar feed.
type Client struct {
	calendarURL string
}

// NewClient creates a Client for the given iCal URL.
func NewClient(calendarURL string) (*Client, error) {
	if calendarURL == "" {
		return nil, fmt.Errorf("calendar URL is required")
	}
	return &Client{calendarURL: calendarURL}, nil
}

// Fetch returns the raw iCal body.
func (c *Client) Fetch(ctx context.Context) ([]byte, error) {
	body, err := calendar.FetchICalendarBody(ctx, c.calendarURL)
	if err != nil {
		return nil, fmt.Errorf("fetch availability calendar: %w", err)
	}
	return body, nil
}

package calendar

import (
	"context"
	"fmt"
	"time"

	"github.com/gldraphael/status/internal/feed"
)

// Client fetches calendar events from an iCal URL.
type Client struct {
	feed    *feed.Client
	nowFunc func() time.Time
}

// NewClient creates a Client for the given iCal URL.
func NewClient(calendarURL string) (*Client, error) {
	if calendarURL == "" {
		return nil, fmt.Errorf("calendar URL is required")
	}
	feedClient, err := feed.NewClient(calendarURL, 30*time.Second)
	if err != nil {
		return nil, err
	}
	return &Client{feed: feedClient, nowFunc: time.Now}, nil
}

// ChangedEvent is a calendar event returned from FetchEvents.
type ChangedEvent struct {
	ID        string
	Summary   string
	StartTime time.Time
	EndTime   time.Time
	Cancelled bool
}

// Fetch fetches the raw iCal body from the configured URL.
func (c *Client) Fetch(ctx context.Context) ([]byte, error) {
	body, err := c.feed.Fetch(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch calendar: %w", err)
	}
	return body, nil
}

// FetchEvents fetches and parses events from the iCal URL.
func (c *Client) FetchEvents(ctx context.Context) ([]ChangedEvent, error) {
	body, err := c.Fetch(ctx)
	if err != nil {
		return nil, err
	}
	now := c.nowFunc()
	parsed, err := ParseICalendar(body, now.Add(-24*time.Hour), now.Add(24*time.Hour))
	if err != nil {
		return nil, fmt.Errorf("fetch events: %w", err)
	}

	events := make([]ChangedEvent, len(parsed.Events))
	for i, p := range parsed.Events {
		events[i] = ChangedEvent{
			ID:        p.ID,
			Summary:   p.Summary,
			StartTime: p.StartTime,
			EndTime:   p.EndTime,
			Cancelled: p.Cancelled,
		}
	}
	return events, nil
}

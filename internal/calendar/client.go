package calendar

import (
	"context"
	"fmt"
	"time"
)

// Client fetches calendar events from an iCal URL.
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

// ChangedEvent is a calendar event returned from FetchEvents.
type ChangedEvent struct {
	ID        string
	Summary   string
	StartTime time.Time
	EndTime   time.Time
	Cancelled bool
}

// FetchEvents fetches all events from the iCal URL.
// The syncToken parameter is ignored (kept for compatibility).
func (c *Client) FetchEvents(ctx context.Context) ([]ChangedEvent, error) {
	parsed, err := FetchAndParseICalendar(ctx, c.calendarURL)
	if err != nil {
		return nil, fmt.Errorf("fetch events: %w", err)
	}

	events := make([]ChangedEvent, len(parsed))
	for i, p := range parsed {
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

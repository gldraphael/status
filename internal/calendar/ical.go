package calendar

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/teambition/rrule-go"
)

// ParsedEvent is an event extracted from an iCal file.
type ParsedEvent struct {
	ID        string
	Summary   string
	StartTime time.Time
	EndTime   time.Time
	Cancelled bool
}

// FetchAndParseICalendar fetches and parses an iCal file from the given URL.
func FetchAndParseICalendar(ctx context.Context, calendarURL string) ([]ParsedEvent, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, calendarURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch calendar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch calendar: unexpected status %s", resp.Status)
	}

	// Read the response body into a byte slice.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return parseICalendar(body, time.Now())
}

// parseICalendar parses an iCal stream and extracts VEVENT components.
// now is used as the center of the recurrence expansion window.
func parseICalendar(data interface{}, now time.Time) ([]ParsedEvent, error) {
	var body string
	switch v := data.(type) {
	case []byte:
		body = string(v)
	case string:
		body = v
	default:
		return nil, fmt.Errorf("unsupported data type")
	}

	if now.IsZero() {
		now = time.Now()
	}

	cal, err := ics.ParseCalendar(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse calendar: %w", err)
	}

	var events []ParsedEvent
	// Expand recurrences for a window around now.
	windowEnd := now.Add(24 * time.Hour)

	for _, event := range cal.Events() {
		summary := event.GetProperty(ics.ComponentPropertySummary).Value
		uid := event.GetProperty(ics.ComponentPropertyUniqueId).Value
		status := event.GetProperty(ics.ComponentPropertyStatus)
		cancelled := status != nil && status.Value == "CANCELLED"

		startAt, err := event.GetStartAt()
		if err != nil {
			continue
		}
		endAt, err := event.GetEndAt()
		if err != nil {
			continue
		}
		duration := endAt.Sub(startAt)

		rruleProp := event.GetProperty(ics.ComponentPropertyRrule)
		if rruleProp == nil {
			// Single event.
			events = append(events, ParsedEvent{
				ID:        uid,
				Summary:   summary,
				StartTime: startAt,
				EndTime:   endAt,
				Cancelled: cancelled,
			})
			continue
		}

		// Recurring event.
		option, err := rrule.StrToROption(rruleProp.Value)
		if err == nil {
			option.Dtstart = startAt
			rule, err := rrule.NewRRule(*option)
			if err == nil {
				// Use a window that looks back far enough to catch events that are still active.
				windowStart := now.Add(-duration)
				instances := rule.Between(windowStart, windowEnd, true)
				for _, inst := range instances {
					events = append(events, ParsedEvent{
						ID:        fmt.Sprintf("%s-%s", uid, inst.Format(time.RFC3339)),
						Summary:   summary,
						StartTime: inst,
						EndTime:   inst.Add(duration),
						Cancelled: cancelled,
					})
				}
				continue
			}
		}

		// Fallback to base instance.
		events = append(events, ParsedEvent{
			ID:        uid,
			Summary:   summary,
			StartTime: startAt,
			EndTime:   endAt,
			Cancelled: cancelled,
		})
	}

	return events, nil
}

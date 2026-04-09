package calendar

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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

	return parseICalendar(body)
}

// parseICalendar parses an iCal stream and extracts VEVENT components.
func parseICalendar(data interface{}) ([]ParsedEvent, error) {
	var body []byte
	switch v := data.(type) {
	case []byte:
		body = v
	case *bytes.Buffer:
		body = v.Bytes()
	default:
		return nil, fmt.Errorf("unsupported data type")
	}

	// First, read all lines and unfold continuation lines
	scanner := bufio.NewScanner(bytes.NewReader(body))
	var lines []string
	var currentLine strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// If this line starts with space/tab, it's a continuation
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			currentLine.WriteString(strings.TrimSpace(line))
		} else {
			// Not a continuation; flush current line if any
			if currentLine.Len() > 0 {
				lines = append(lines, currentLine.String())
				currentLine.Reset()
			}
			lines = append(lines, line)
		}
	}
	// Flush any remaining line
	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read lines: %w", err)
	}

	// Now process the unfolded lines
	var events []ParsedEvent
	var inEvent bool
	var event ParsedEvent
	var eventStart, eventEnd string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if line == "BEGIN:VEVENT" {
			inEvent = true
			event = ParsedEvent{}
			eventStart = ""
			eventEnd = ""
			continue
		}
		if line == "END:VEVENT" {
			inEvent = false
			// Parse timestamps if they were collected.
			if eventStart != "" {
				if t, err := parseEventTime(eventStart); err == nil {
					event.StartTime = t
				}
			}
			if eventEnd != "" {
				if t, err := parseEventTime(eventEnd); err == nil {
					event.EndTime = t
				}
			}
			if event.ID != "" && event.Summary != "" {
				events = append(events, event)
			}
			continue
		}

		if !inEvent {
			continue
		}

		// Parse event properties.
		if strings.HasPrefix(line, "UID:") {
			event.ID = strings.TrimPrefix(line, "UID:")
		} else if strings.HasPrefix(line, "SUMMARY:") {
			event.Summary = strings.TrimPrefix(line, "SUMMARY:")
		} else if strings.HasPrefix(line, "DTSTART") {
			// Extract the value part; ignore TZID and other params.
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				eventStart = parts[len(parts)-1]
			}
		} else if strings.HasPrefix(line, "DTEND") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				eventEnd = parts[len(parts)-1]
			}
		} else if line == "STATUS:CANCELLED" {
			event.Cancelled = true
		}
	}

	return events, nil
}

// parseEventTime parses an iCal DATE-TIME or DATE string.
// Handles formats: YYYYMMDDTHHMMSSZ, YYYYMMDD, YYYYMMDDTHHMMSS
func parseEventTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)

	// DATE-TIME with Z suffix (UTC).
	if strings.HasSuffix(s, "Z") {
		return time.Parse("20060102T150405Z", s)
	}

	// DATE-TIME without timezone (local).
	if len(s) == 15 && strings.Contains(s, "T") {
		return time.Parse("20060102T150405", s)
	}

	// DATE only.
	if len(s) == 8 && !strings.Contains(s, "T") {
		return time.Parse("20060102", s)
	}

	return time.Time{}, fmt.Errorf("unsupported time format: %q", s)
}

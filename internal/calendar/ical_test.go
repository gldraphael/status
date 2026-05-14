package calendar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func parseEvents(data string, now time.Time) ([]ParsedEvent, error) {
	parsed, err := ParseICalendar([]byte(data), now.Add(-24*time.Hour), now.Add(24*time.Hour))
	if err != nil {
		return nil, err
	}
	return parsed.Events, nil
}

func fetchEvents(t *testing.T, calendarURL string, now time.Time) ([]ChangedEvent, error) {
	t.Helper()
	client, err := NewClient(calendarURL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.nowFunc = func() time.Time { return now }

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return client.FetchEvents(ctx)
}

func TestParseICalendar_BasicEvent(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
PRODID:-//Test//Test Calendar//EN
VERSION:2.0
BEGIN:VEVENT
UID:event1@example.com
DTSTART:20260406T100000Z
DTEND:20260406T110000Z
SUMMARY:Test Event
END:VEVENT
END:VCALENDAR`

	now := time.Date(2026, 4, 6, 10, 30, 0, 0, time.UTC)
	events, err := parseEvents(icalData, now)
	if err != nil {
		t.Fatalf("parseEvents: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Summary != "Test Event" {
		t.Errorf("Summary: got %q, want %q", events[0].Summary, "Test Event")
	}
	if !strings.HasPrefix(events[0].ID, "event1@example.com") {
		t.Errorf("ID: got %q, want prefix %q", events[0].ID, "event1@example.com")
	}
}

func TestParseICalendar_MultipleEvents(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
PRODID:-//Test//Test Calendar//EN
VERSION:2.0
BEGIN:VEVENT
UID:event1
DTSTART:20260406T100000Z
DTEND:20260406T110000Z
SUMMARY:First Event
END:VEVENT
BEGIN:VEVENT
UID:event2
DTSTART:20260406T150000Z
DTEND:20260406T160000Z
SUMMARY:Second Event
END:VEVENT
END:VCALENDAR`

	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	events, err := parseEvents(icalData, now)
	if err != nil {
		t.Fatalf("parseEvents: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Summary != "First Event" {
		t.Errorf("First event summary: got %q", events[0].Summary)
	}
	if events[1].Summary != "Second Event" {
		t.Errorf("Second event summary: got %q", events[1].Summary)
	}
}

func TestParseICalendar_CancelledEvent(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
PRODID:-//Test//Test Calendar//EN
VERSION:2.0
BEGIN:VEVENT
UID:event1
DTSTART:20260406T100000Z
DTEND:20260406T110000Z
SUMMARY:Cancelled Event
STATUS:CANCELLED
END:VEVENT
END:VCALENDAR`

	now := time.Date(2026, 4, 6, 10, 30, 0, 0, time.UTC)
	events, err := parseEvents(icalData, now)
	if err != nil {
		t.Fatalf("parseEvents: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if !events[0].Cancelled {
		t.Error("expected Cancelled=true for event with STATUS:CANCELLED")
	}
}

func TestParseICalendar_DateOnly(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
PRODID:-//Test//Test Calendar//EN
VERSION:2.0
BEGIN:VEVENT
UID:event1
DTSTART;VALUE=DATE:20260406
DTEND;VALUE=DATE:20260407
SUMMARY:All Day Event
END:VEVENT
END:VCALENDAR`

	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	events, err := parseEvents(icalData, now)
	if err != nil {
		t.Fatalf("parseEvents: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].StartTime.Year() != 2026 || events[0].StartTime.Month() != 4 || events[0].StartTime.Day() != 6 {
		t.Errorf("StartTime: got %v", events[0].StartTime)
	}
}

func TestClientFetchEvents_WithHTTPServer(t *testing.T) {
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	start := now.Add(-time.Hour).Format("20060102T150405Z")
	end := now.Add(time.Hour).Format("20060102T150405Z")
	icalData := "BEGIN:VCALENDAR\n" +
		"PRODID:-//Google Inc//Google Calendar 70.9054//EN\n" +
		"VERSION:2.0\n" +
		"CALSCALE:GREGORIAN\n" +
		"METHOD:PUBLISH\n" +
		"BEGIN:VEVENT\n" +
		"UID:test-event@example.com\n" +
		"DTSTART:" + start + "\n" +
		"DTEND:" + end + "\n" +
		"SUMMARY:Team Sync\n" +
		"STATUS:CONFIRMED\n" +
		"END:VEVENT\n" +
		"END:VCALENDAR"

	// Create a test server that returns text/calendar content-type
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(icalData))
	}))
	defer server.Close()

	events, err := fetchEvents(t, server.URL, now)
	if err != nil {
		t.Fatalf("FetchEvents: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Summary != "Team Sync" {
		t.Errorf("Summary: got %q, want %q", events[0].Summary, "Team Sync")
	}
}

func TestClientFetchEvents_404Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := fetchEvents(t, server.URL, time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC))
	if err == nil {
		t.Error("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error message, got: %v", err)
	}
}

func TestClientFetchEvents_GoogleCalendarFormat(t *testing.T) {
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	start := now.Add(-time.Hour).Format("20060102T150405Z")
	end := now.Add(time.Hour).Format("20060102T150405Z")
	icalData := "BEGIN:VCALENDAR\n" +
		"PRODID:-//Google Inc//Google Calendar 70.9054//EN\n" +
		"VERSION:2.0\n" +
		"CALSCALE:GREGORIAN\n" +
		"METHOD:PUBLISH\n" +
		"X-WR-CALNAME:Status\n" +
		"X-WR-TIMEZONE:Europe/London\n" +
		"X-WR-CALDESC:Calendar to control my public status\n" +
		"BEGIN:VEVENT\n" +
		"UID:test-event@example.com\n" +
		"DTSTART:" + start + "\n" +
		"DTEND:" + end + "\n" +
		"SUMMARY:Team Meeting\n" +
		"STATUS:CONFIRMED\n" +
		"END:VEVENT\n" +
		"END:VCALENDAR"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Exact header from the curl example
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, max-age=0, must-revalidate")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(icalData))
	}))
	defer server.Close()

	events, err := fetchEvents(t, server.URL, now)
	if err != nil {
		t.Fatalf("FetchEvents: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Summary != "Team Meeting" {
		t.Errorf("Summary: got %q, want %q", events[0].Summary, "Team Meeting")
	}
	if !strings.HasPrefix(events[0].ID, "test-event@example.com") {
		t.Errorf("ID: got %q, want prefix %q", events[0].ID, "test-event@example.com")
	}
	if events[0].Cancelled {
		t.Errorf("Cancelled: got true, want false")
	}

	t.Logf("✅ Successfully parsed Google Calendar iCal format")
}

func TestParseICalendar_RecurringEvents(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:recurring-daily
DTSTART:20260401T090000Z
DTEND:20260401T100000Z
RRULE:FREQ=DAILY
SUMMARY:Daily Sync
END:VEVENT
END:VCALENDAR`

	// Test a date well after the initial DTSTART to verify RRULE expansion.
	now := time.Date(2026, 4, 15, 9, 30, 0, 0, time.UTC)
	events, err := parseEvents(icalData, now)
	if err != nil {
		t.Fatalf("parseEvents: %v", err)
	}

	found := false
	for _, ev := range events {
		// The expanded event should have the same summary but a different start/end time.
		if ev.Summary == "Daily Sync" && !ev.StartTime.After(now) && ev.EndTime.After(now) {
			found = true
			// Verify it's the instance for the 15th
			if ev.StartTime.Day() != 15 {
				t.Errorf("expected instance for the 15th, got %v", ev.StartTime)
			}
			break
		}
	}

	if !found {
		t.Errorf("expected to find active recurring instance of 'Daily Sync' for %v", now)
	}
}

func TestParseICalendar_MultiDayRecurringEvent(t *testing.T) {
	// Event starts Monday 09:00, ends Wednesday 17:00, repeats weekly on Monday.
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:multi-day-recurring
DTSTART:20260406T090000Z
DTEND:20260408T170000Z
RRULE:FREQ=WEEKLY;BYDAY=MO
SUMMARY:Multi-day Workshop
END:VEVENT
END:VCALENDAR`

	// 2026-04-06 is Monday.
	// 2026-04-07 is Tuesday.
	// Tuesday 12:00 is exactly in the middle of the first occurrence.
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)

	// The current window in ical.go is now - 24h to now + 24h.
	// now - 24h is 2026-04-06 12:00.
	// The event started at 2026-04-06 09:00, which is BEFORE the window.

	events, err := parseEvents(icalData, now)
	if err != nil {
		t.Fatalf("parseEvents: %v", err)
	}

	found := false
	for _, ev := range events {
		if ev.Summary == "Multi-day Workshop" {
			if !ev.StartTime.After(now) && ev.EndTime.After(now) {
				found = true
				break
			}
		}
	}

	if !found {
		t.Errorf("expected to find active recurring instance of 'Multi-day Workshop' for %v", now)
	}
}

func TestParseICalendar_MultiDaySingleEvent(t *testing.T) {
	// Single event spanning multiple days.
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:multi-day-single
DTSTART:20260406T090000Z
DTEND:20260408T170000Z
SUMMARY:Long Workshop
END:VEVENT
END:VCALENDAR`

	// 2026-04-07 is Tuesday, in the middle of the event.
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)

	events, err := parseEvents(icalData, now)
	if err != nil {
		t.Fatalf("parseEvents: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	ev := events[0]
	if ev.Summary != "Long Workshop" {
		t.Errorf("Summary: got %q, want %q", ev.Summary, "Long Workshop")
	}

	if ev.StartTime.After(now) || ev.EndTime.Before(now) {
		t.Errorf("Event should be active at %v, but got StartTime=%v, EndTime=%v", now, ev.StartTime, ev.EndTime)
	}
}

func TestExtractICalendarTimezone(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
X-WR-TIMEZONE:Europe/London
BEGIN:VEVENT
UID:event1
DTSTART:20260406T100000Z
DTEND:20260406T110000Z
SUMMARY:Test Event
END:VEVENT
END:VCALENDAR`

	tz, err := ExtractICalendarTimezone([]byte(icalData))
	if err != nil {
		t.Fatalf("ExtractICalendarTimezone: %v", err)
	}
	if tz != "Europe/London" {
		t.Fatalf("timezone: got %q, want %q", tz, "Europe/London")
	}
}

func TestParseICalendar_Transparency(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:busy-event
DTSTART:20260406T100000Z
DTEND:20260406T110000Z
SUMMARY:Busy Event
TRANSP:OPAQUE
END:VEVENT
BEGIN:VEVENT
UID:free-event
DTSTART:20260406T120000Z
DTEND:20260406T130000Z
SUMMARY:Free Event
TRANSP:TRANSPARENT
END:VEVENT
BEGIN:VEVENT
UID:default-busy-event
DTSTART:20260406T140000Z
DTEND:20260406T150000Z
SUMMARY:Default Busy Event
END:VEVENT
END:VCALENDAR`

	now := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	events, err := parseEvents(icalData, now)
	if err != nil {
		t.Fatalf("parseEvents: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	tests := []struct {
		summary string
		busy    bool
	}{
		{"Busy Event", true},
		{"Free Event", false},
		{"Default Busy Event", true},
	}

	for _, tt := range tests {
		found := false
		for _, ev := range events {
			if ev.Summary == tt.summary {
				found = true
				if ev.Busy != tt.busy {
					t.Errorf("%s: Busy got %v, want %v", tt.summary, ev.Busy, tt.busy)
				}
				break
			}
		}
		if !found {
			t.Errorf("could not find event: %s", tt.summary)
		}
	}
}

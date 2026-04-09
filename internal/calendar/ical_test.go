package calendar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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

	events, err := parseICalendar([]byte(icalData))
	if err != nil {
		t.Fatalf("parseICalendar: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Summary != "Test Event" {
		t.Errorf("Summary: got %q, want %q", events[0].Summary, "Test Event")
	}
	if events[0].ID != "event1@example.com" {
		t.Errorf("ID: got %q, want %q", events[0].ID, "event1@example.com")
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

	events, err := parseICalendar([]byte(icalData))
	if err != nil {
		t.Fatalf("parseICalendar: %v", err)
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

	events, err := parseICalendar([]byte(icalData))
	if err != nil {
		t.Fatalf("parseICalendar: %v", err)
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

	events, err := parseICalendar([]byte(icalData))
	if err != nil {
		t.Fatalf("parseICalendar: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].StartTime.Year() != 2026 || events[0].StartTime.Month() != 4 || events[0].StartTime.Day() != 6 {
		t.Errorf("StartTime: got %v", events[0].StartTime)
	}
}

func TestFetchAndParseICalendar_WithHTTPServer(t *testing.T) {
	icalData := `BEGIN:VCALENDAR
PRODID:-//Google Inc//Google Calendar 70.9054//EN
VERSION:2.0
CALSCALE:GREGORIAN
METHOD:PUBLISH
BEGIN:VEVENT
UID:test-event@example.com
DTSTART:20260406T140000Z
DTEND:20260406T150000Z
SUMMARY:Team Sync
STATUS:CONFIRMED
END:VEVENT
END:VCALENDAR`

	// Create a test server that returns text/calendar content-type
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(icalData))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, err := FetchAndParseICalendar(ctx, server.URL)
	if err != nil {
		t.Fatalf("FetchAndParseICalendar: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Summary != "Team Sync" {
		t.Errorf("Summary: got %q, want %q", events[0].Summary, "Team Sync")
	}
}

func TestFetchAndParseICalendar_404Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := FetchAndParseICalendar(ctx, server.URL)
	if err == nil {
		t.Error("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error message, got: %v", err)
	}
}

func TestParseEventTime_DateTimeUTC(t *testing.T) {
	// Test UTC datetime format (ends with Z)
	time1, err := parseEventTime("20260406T140530Z")
	if err != nil {
		t.Fatalf("parseEventTime: %v", err)
	}

	if time1.Year() != 2026 || time1.Month() != 4 || time1.Day() != 6 {
		t.Errorf("Date: got %v", time1)
	}
	if time1.Hour() != 14 || time1.Minute() != 5 || time1.Second() != 30 {
		t.Errorf("Time: got %v", time1)
	}
}

func TestParseEventTime_DateTime(t *testing.T) {
	// Test datetime format without timezone
	time1, err := parseEventTime("20260406T100000")
	if err != nil {
		t.Fatalf("parseEventTime: %v", err)
	}

	if time1.Year() != 2026 || time1.Month() != 4 || time1.Day() != 6 {
		t.Errorf("Date: got %v", time1)
	}
}

func TestParseEventTime_DateOnly(t *testing.T) {
	// Test date-only format
	time1, err := parseEventTime("20260406")
	if err != nil {
		t.Fatalf("parseEventTime: %v", err)
	}

	if time1.Year() != 2026 || time1.Month() != 4 || time1.Day() != 6 {
		t.Errorf("Date: got %v", time1)
	}
}

func TestFetchAndParseICalendar_GoogleCalendarFormat(t *testing.T) {
	// This is the exact format from Google Calendar's iCal export
	icalData := `BEGIN:VCALENDAR
PRODID:-//Google Inc//Google Calendar 70.9054//EN
VERSION:2.0
CALSCALE:GREGORIAN
METHOD:PUBLISH
X-WR-CALNAME:Status
X-WR-TIMEZONE:Europe/London
X-WR-CALDESC:Calendar to control my public status
BEGIN:VEVENT
UID:test-event@example.com
DTSTART:20260406T140000Z
DTEND:20260406T150000Z
SUMMARY:Team Meeting
STATUS:CONFIRMED
END:VEVENT
END:VCALENDAR`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Exact header from the curl example
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, max-age=0, must-revalidate")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(icalData))
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, err := FetchAndParseICalendar(ctx, server.URL)
	if err != nil {
		t.Fatalf("FetchAndParseICalendar: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Summary != "Team Meeting" {
		t.Errorf("Summary: got %q, want %q", events[0].Summary, "Team Meeting")
	}
	if events[0].ID != "test-event@example.com" {
		t.Errorf("ID: got %q, want %q", events[0].ID, "test-event@example.com")
	}
	if events[0].Cancelled {
		t.Errorf("Cancelled: got true, want false")
	}

	t.Logf("✅ Successfully parsed Google Calendar iCal format")
}

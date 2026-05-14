package timeutil

import (
	"testing"
	"time"
)

func TestParseClockRange(t *testing.T) {
	start, end, err := ParseClockRange("09:00", "17:50")
	if err != nil {
		t.Fatalf("ParseClockRange: %v", err)
	}
	if start != 9*time.Hour {
		t.Fatalf("start: got %v, want 9h", start)
	}
	if end != 17*time.Hour+50*time.Minute {
		t.Fatalf("end: got %v, want 17h50m", end)
	}
}

func TestParseClockRange_Invalid(t *testing.T) {
	if _, _, err := ParseClockRange("17:00", "09:00"); err == nil {
		t.Fatal("expected error for non-increasing range")
	}
	if _, _, err := ParseClockRange("bad", "09:00"); err == nil {
		t.Fatal("expected error for invalid clock")
	}
}

func TestLoadLocationFallback(t *testing.T) {
	if got := LoadLocation(""); got != time.UTC {
		t.Fatalf("empty location: got %v, want UTC", got)
	}
	if got := LoadLocation("not-a-location"); got != time.UTC {
		t.Fatalf("invalid location: got %v, want UTC", got)
	}
	if got := LoadLocation("Europe/London"); got.String() != "Europe/London" {
		t.Fatalf("valid location: got %v", got)
	}
}

func TestOverlaps(t *testing.T) {
	base := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	if !Overlaps(base, base.Add(time.Hour), base.Add(30*time.Minute), base.Add(90*time.Minute)) {
		t.Fatal("expected overlapping ranges")
	}
	if Overlaps(base, base.Add(time.Hour), base.Add(time.Hour), base.Add(2*time.Hour)) {
		t.Fatal("expected touching ranges not to overlap")
	}
}

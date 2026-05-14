package timeutil

import (
	"fmt"
	"strings"
	"time"
)

const clockLayout = "15:04"

// ParseClock parses an HH:MM value into a duration since midnight.
func ParseClock(value string) (time.Duration, error) {
	t, err := time.Parse(clockLayout, strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute, nil
}

// ParseClockRange parses and validates an ordered HH:MM range.
func ParseClockRange(startValue, endValue string) (time.Duration, time.Duration, error) {
	start, err := ParseClock(startValue)
	if err != nil {
		return 0, 0, err
	}
	end, err := ParseClock(endValue)
	if err != nil {
		return 0, 0, err
	}
	if end <= start {
		return 0, 0, fmt.Errorf("end must be after start")
	}
	return start, end, nil
}

// LoadLocation returns the named location, falling back to UTC for empty or invalid names.
func LoadLocation(name string) *time.Location {
	if name == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

// Overlaps reports whether [start, end) intersects [windowStart, windowEnd).
func Overlaps(start, end, windowStart, windowEnd time.Time) bool {
	return start.Before(windowEnd) && end.After(windowStart)
}

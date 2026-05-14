package calendar

import (
	"fmt"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/teambition/rrule-go"

	"github.com/gldraphael/status/internal/timeutil"
)

// ParsedEvent is an event extracted from an iCal file.
type ParsedEvent struct {
	ID        string
	Summary   string
	StartTime time.Time
	EndTime   time.Time
	Cancelled bool
	Busy      bool
}

// ParsedCalendar is the parsed representation of an iCal feed.
type ParsedCalendar struct {
	Timezone string
	Events   []ParsedEvent
}

// ParseICalendar parses an iCal stream and expands recurring events within the
// requested time window.
func ParseICalendar(data []byte, windowStart, windowEnd time.Time) (*ParsedCalendar, error) {
	if windowStart.IsZero() {
		windowStart = time.Now()
	}
	if windowEnd.IsZero() {
		windowEnd = windowStart.Add(24 * time.Hour)
	}
	if windowEnd.Before(windowStart) {
		return nil, fmt.Errorf("window end must not be before window start")
	}

	cal, err := ics.ParseCalendar(strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("parse calendar: %w", err)
	}

	parsed := &ParsedCalendar{
		Timezone: calendarTimezone(cal),
		Events:   make([]ParsedEvent, 0, len(cal.Events())),
	}

	type baseEvent struct {
		id        string
		summary   string
		startTime time.Time
		endTime   time.Time
		cancelled bool
		busy      bool
		rrule     string
	}

	baseEvents := make([]baseEvent, 0, len(cal.Events()))
	maxDuration := time.Duration(0)

	for _, event := range cal.Events() {
		summary := ""
		if prop := event.GetProperty(ics.ComponentPropertySummary); prop != nil {
			summary = prop.Value
		}
		uid := ""
		if prop := event.GetProperty(ics.ComponentPropertyUniqueId); prop != nil {
			uid = prop.Value
		}
		status := event.GetProperty(ics.ComponentPropertyStatus)
		cancelled := status != nil && status.Value == "CANCELLED"

		busy := true
		if transp := event.GetProperty(ics.ComponentPropertyTransp); transp != nil {
			busy = strings.ToUpper(transp.Value) == "OPAQUE"
		}

		startAt, err := event.GetStartAt()
		if err != nil {
			continue
		}
		endAt, err := event.GetEndAt()
		if err != nil {
			continue
		}
		if duration := endAt.Sub(startAt); duration > maxDuration {
			maxDuration = duration
		}

		base := baseEvent{
			id:        uid,
			summary:   summary,
			startTime: startAt,
			endTime:   endAt,
			cancelled: cancelled,
			busy:      busy,
		}
		if rruleProp := event.GetProperty(ics.ComponentPropertyRrule); rruleProp != nil {
			base.rrule = rruleProp.Value
		}
		baseEvents = append(baseEvents, base)
	}

	lookback := windowStart.Add(-maxDuration)

	for _, base := range baseEvents {
		if base.rrule == "" {
			if timeutil.Overlaps(base.startTime, base.endTime, windowStart, windowEnd) {
				parsed.Events = append(parsed.Events, ParsedEvent{
					ID:        base.id,
					Summary:   base.summary,
					StartTime: base.startTime,
					EndTime:   base.endTime,
					Cancelled: base.cancelled,
					Busy:      base.busy,
				})
			}
			continue
		}

		option, err := rrule.StrToROption(base.rrule)
		if err == nil {
			option.Dtstart = base.startTime
			rule, err := rrule.NewRRule(*option)
			if err == nil {
				instances := rule.Between(lookback, windowEnd, true)
				duration := base.endTime.Sub(base.startTime)
				for _, inst := range instances {
					endAt := inst.Add(duration)
					if !timeutil.Overlaps(inst, endAt, windowStart, windowEnd) {
						continue
					}
					parsed.Events = append(parsed.Events, ParsedEvent{
						ID:        fmt.Sprintf("%s-%s", base.id, inst.Format(time.RFC3339)),
						Summary:   base.summary,
						StartTime: inst,
						EndTime:   endAt,
						Cancelled: base.cancelled,
						Busy:      base.busy,
					})
				}
				continue
			}
		}

		if timeutil.Overlaps(base.startTime, base.endTime, windowStart, windowEnd) {
			parsed.Events = append(parsed.Events, ParsedEvent{
				ID:        base.id,
				Summary:   base.summary,
				StartTime: base.startTime,
				EndTime:   base.endTime,
				Cancelled: base.cancelled,
				Busy:      base.busy,
			})
		}
	}

	return parsed, nil
}

// ExtractICalendarTimezone returns the calendar's timezone, falling back to UTC.
func ExtractICalendarTimezone(data []byte) (string, error) {
	cal, err := ics.ParseCalendar(strings.NewReader(string(data)))
	if err != nil {
		return "", fmt.Errorf("parse calendar: %w", err)
	}
	return calendarTimezone(cal), nil
}

func calendarTimezone(cal *ics.Calendar) string {
	for _, prop := range cal.CalendarProperties {
		switch prop.IANAToken {
		case string(ics.PropertyXWRTimezone), string(ics.PropertyTimezoneId), string(ics.PropertyTzid):
			if prop.Value != "" {
				return prop.Value
			}
		}
	}
	return "UTC"
}

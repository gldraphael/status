package availability

import (
	"fmt"
	"time"

	"github.com/gldraphael/status/internal/calendar"
	"github.com/gldraphael/status/internal/config"
)

// Block is one ordered availability window.
type Block struct {
	Name  string
	Start time.Duration
	End   time.Duration
}

// Entry is one availability result returned by the API.
type Entry struct {
	DayOfWeek string `json:"day_of_week"`
	Block     string `json:"block"`
	Date      string `json:"date"`
}

// ParseBlocks converts configured blocks into runtime blocks.
func ParseBlocks(blocks []config.AvailabilityBlockConfig) ([]Block, error) {
	parsed := make([]Block, 0, len(blocks))
	for i, block := range blocks {
		start, err := parseClock(block.Start)
		if err != nil {
			return nil, fmt.Errorf("availability.blocks[%d].start: %w", i, err)
		}
		end, err := parseClock(block.End)
		if err != nil {
			return nil, fmt.Errorf("availability.blocks[%d].end: %w", i, err)
		}
		parsed = append(parsed, Block{
			Name:  block.Name,
			Start: start,
			End:   end,
		})
	}
	return parsed, nil
}

// Compute derives availability entries from a stored raw iCal body.
func Compute(body string, timezone string, blocks []Block, now time.Time) ([]Entry, error) {
	if len(blocks) == 0 {
		return nil, fmt.Errorf("no availability blocks configured")
	}

	loc := loadLocation(timezone)
	nowLocal := now.In(loc)
	dayStart := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc)
	windowStart := dayStart
	windowEnd := dayStart.AddDate(0, 0, 10)

	parsed, err := calendar.ParseICalendar([]byte(body), windowStart, windowEnd)
	if err != nil {
		return nil, err
	}

	entries := make([]Entry, 0, 10)
	for dayOffset := 0; dayOffset < 10; dayOffset++ {
		day := dayStart.AddDate(0, 0, dayOffset)

		for _, block := range blocks {
			blockStart := day.Add(block.Start)
			blockEnd := day.Add(block.End)

			if dayOffset == 0 && blockStart.Before(nowLocal) {
				continue
			}
			if !blockIsFree(parsed.Events, blockStart, blockEnd, loc) {
				continue
			}

			entries = append(entries, Entry{
				DayOfWeek: day.Weekday().String(),
				Block:     block.Name,
				Date:      day.Format("2006-01-02"),
			})
			break
		}
	}

	return entries, nil
}

func parseClock(value string) (time.Duration, error) {
	t, err := time.Parse("15:04", value)
	if err != nil {
		return 0, err
	}
	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute, nil
}

func loadLocation(name string) *time.Location {
	if name == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

func blockIsFree(events []calendar.ParsedEvent, start, end time.Time, loc *time.Location) bool {
	for _, event := range events {
		if event.Cancelled {
			continue
		}

		eventStart := event.StartTime.In(loc)
		eventEnd := event.EndTime.In(loc)
		if eventStart.Before(end) && eventEnd.After(start) {
			return false
		}
	}
	return true
}

package availability

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gldraphael/status/internal/calendar"
	"github.com/gldraphael/status/internal/store"
	"github.com/gldraphael/status/internal/timeutil"
)

var (
	// ErrSnapshotNotFound is returned when a required availability or holiday snapshot is missing.
	ErrSnapshotNotFound = errors.New("snapshot not found")
)

// Block is one ordered availability window.
type Block struct {
	Name  string
	Start time.Duration
	End   time.Duration
}

// WorkingHours describes the weekday time window used to suppress availability.
type WorkingHours struct {
	Start time.Duration
	End   time.Duration
}

// Entry is one availability result returned by the API.
type Entry struct {
	DayOfWeek string `json:"day_of_week"`
	Block     string `json:"block"`
	Date      string `json:"date"`
}

// ComputeOptions configures availability computation.
type ComputeOptions struct {
	WorkingHours               WorkingHours
	HolidayDates               []string
	ExcludeEnglandBankHolidays bool
	Now                        time.Time
}

// ParseBlock converts a named clock range into a runtime availability block.
func ParseBlock(name, startValue, endValue string) (Block, error) {
	start, err := timeutil.ParseClock(startValue)
	if err != nil {
		return Block{}, fmt.Errorf("start: %w", err)
	}
	end, err := timeutil.ParseClock(endValue)
	if err != nil {
		return Block{}, fmt.Errorf("end: %w", err)
	}
	return Block{Name: name, Start: start, End: end}, nil
}

// ParseWorkingHours converts the configured weekday working-hours window.
func ParseWorkingHours(startValue, endValue string) (WorkingHours, error) {
	start, end, err := timeutil.ParseClockRange(startValue, endValue)
	if err != nil {
		return WorkingHours{}, err
	}
	return WorkingHours{Start: start, End: end}, nil
}

// Provider computes and provides availability entries.
type Provider struct {
	store                      *store.Store
	blocks                     []Block
	workingHours               WorkingHours
	excludeEnglandBankHolidays bool
	nowFunc                    func() time.Time
}

// NewProvider creates a new availability provider.
func NewProvider(st *store.Store, blocks []Block, workingHours WorkingHours, excludeEnglandBankHolidays bool) *Provider {
	return &Provider{
		store:                      st,
		blocks:                     blocks,
		workingHours:               workingHours,
		excludeEnglandBankHolidays: excludeEnglandBankHolidays,
		nowFunc:                    time.Now,
	}
}

// GetEntries returns current availability entries.
func (p *Provider) GetEntries() ([]Entry, error) {
	data, err := p.GetEntriesJSON()
	if err != nil {
		return nil, err
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("decode current availability: %w", err)
	}
	return entries, nil
}

// GetEntriesJSON returns current availability entries serialized as JSON.
func (p *Provider) GetEntriesJSON() ([]byte, error) {
	data, ok, err := p.store.GetAvailabilityCurrent()
	if err != nil {
		return nil, fmt.Errorf("get current availability: %w", err)
	}
	if !ok {
		return nil, ErrSnapshotNotFound
	}
	return data, nil
}

// RefreshCurrentFromStoredSnapshot recomputes and stores the current API
// response from the latest persisted raw availability snapshot.
func (p *Provider) RefreshCurrentFromStoredSnapshot() error {
	snap, ok, err := p.store.GetAvailabilityRawSnapshot()
	if err != nil {
		return fmt.Errorf("get availability raw snapshot: %w", err)
	}
	if !ok {
		if err := p.store.ClearAvailabilityCurrent(); err != nil {
			return fmt.Errorf("clear current availability: %w", err)
		}
		return ErrSnapshotNotFound
	}

	data, err := p.ComputeEntriesJSONFromSnapshot(snap)
	if err != nil {
		return err
	}
	if err := p.store.SetAvailabilityCurrent(data); err != nil {
		return fmt.Errorf("store current availability: %w", err)
	}
	return nil
}

// ComputeEntriesJSONFromSnapshot computes the current API response JSON from a
// raw snapshot. This is intended for sync/startup paths, not request serving.
func (p *Provider) ComputeEntriesJSONFromSnapshot(snap *store.CalendarSnapshot) ([]byte, error) {
	opts := ComputeOptions{
		WorkingHours: p.workingHours,
		Now:          p.nowFunc(),
	}
	if p.excludeEnglandBankHolidays {
		holidaySnap, ok, err := p.store.GetHolidaySnapshot()
		if err != nil {
			return nil, fmt.Errorf("get bank holiday snapshot: %w", err)
		}
		if !ok {
			return nil, ErrSnapshotNotFound
		}
		opts.ExcludeEnglandBankHolidays = true
		opts.HolidayDates = holidaySnap.Dates
	}

	entries, err := Compute(snap.Body, snap.Timezone, p.blocks, opts)
	if err != nil {
		return nil, err
	}
	return json.Marshal(entries)
}

// Compute derives availability entries from a stored raw iCal body.
func Compute(body string, timezone string, blocks []Block, opts ComputeOptions) ([]Entry, error) {
	if len(blocks) == 0 {
		return nil, fmt.Errorf("no availability blocks configured")
	}
	if opts.WorkingHours.End <= opts.WorkingHours.Start {
		return nil, fmt.Errorf("invalid working hours")
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}

	loc := timeutil.LoadLocation(timezone)
	nowLocal := opts.Now.In(loc)
	dayStart := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc)
	windowEnd := dayStart.AddDate(0, 0, 10)

	parsed, err := calendar.ParseICalendar([]byte(body), dayStart, windowEnd)
	if err != nil {
		return nil, err
	}

	holidaySet := holidayDatesSet(opts.HolidayDates, opts.ExcludeEnglandBankHolidays)

	entries := make([]Entry, 0, 10)
	for dayOffset := 0; dayOffset < 10; dayOffset++ {
		day := dayStart.AddDate(0, 0, dayOffset)
		block, ok := firstFreeBlock(
			parsed.Events,
			blocks,
			day,
			dayOffset == 0,
			nowLocal,
			opts.WorkingHours,
			holidaySet,
			opts.ExcludeEnglandBankHolidays,
			loc,
		)
		if ok {
			entries = append(entries, Entry{
				DayOfWeek: day.Weekday().String(),
				Block:     block.Name,
				Date:      day.Format("2006-01-02"),
			})
		}
	}

	return entries, nil
}

func holidayDatesSet(dates []string, enabled bool) map[string]struct{} {
	holidaySet := make(map[string]struct{}, len(dates))
	if !enabled {
		return holidaySet
	}
	for _, date := range dates {
		if date == "" {
			continue
		}
		holidaySet[date] = struct{}{}
	}
	return holidaySet
}

func firstFreeBlock(
	events []calendar.ParsedEvent,
	blocks []Block,
	day time.Time,
	isToday bool,
	now time.Time,
	workingHours WorkingHours,
	holidaySet map[string]struct{},
	excludeHolidays bool,
	loc *time.Location,
) (Block, bool) {
	applyWorkingHours := isWeekday(day) && !isExcludedHoliday(day, holidaySet, excludeHolidays)
	workingStart := day.Add(workingHours.Start)
	workingEnd := day.Add(workingHours.End)

	for _, block := range blocks {
		blockStart := day.Add(block.Start)
		blockEnd := day.Add(block.End)

		if isToday && blockStart.Before(now) {
			continue
		}
		if applyWorkingHours && timeutil.Overlaps(blockStart, blockEnd, workingStart, workingEnd) {
			continue
		}
		if !blockIsFree(events, blockStart, blockEnd, loc) {
			continue
		}
		return block, true
	}
	return Block{}, false
}

func isWeekday(day time.Time) bool {
	return day.Weekday() != time.Saturday && day.Weekday() != time.Sunday
}

func isExcludedHoliday(day time.Time, holidaySet map[string]struct{}, enabled bool) bool {
	if !enabled {
		return false
	}
	_, ok := holidaySet[day.Format("2006-01-02")]
	return ok
}

func blockIsFree(events []calendar.ParsedEvent, start, end time.Time, loc *time.Location) bool {
	for _, event := range events {
		if event.Cancelled || !event.Busy {
			continue
		}

		eventStart := event.StartTime.In(loc)
		eventEnd := event.EndTime.In(loc)
		if timeutil.Overlaps(eventStart, eventEnd, start, end) {
			return false
		}
	}
	return true
}

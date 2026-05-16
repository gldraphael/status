package calendar

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/gldraphael/status/internal/poll"
	"github.com/gldraphael/status/internal/store"
	"github.com/gldraphael/status/internal/target"
)

// calendarClient defines the interface for fetching raw calendar data.
type calendarClient interface {
	Fetch(ctx context.Context) ([]byte, error)
}

// Syncer periodically syncs calendar events to configured targets.
type Syncer struct {
	store   *store.Store
	cal     calendarClient
	targets []target.Target
	logger  zerolog.Logger
	nowFunc func() time.Time
}

// NewSyncer creates a new Syncer.
func NewSyncer(st *store.Store, cal calendarClient, targets []target.Target, logger zerolog.Logger) *Syncer {
	return &Syncer{
		store:   st,
		cal:     cal,
		targets: targets,
		logger:  logger,
		nowFunc: time.Now,
	}
}

// Run starts the sync loop, fetching events and syncing status at the given interval.
// Run blocks until ctx is cancelled.
func (s *Syncer) Run(ctx context.Context, interval time.Duration) error {
	return poll.Every(ctx, interval, func() error {
		return s.syncOnce(ctx)
	}, func(err error, startup bool) {
		msg := "sync cycle"
		if startup {
			msg = "sync on startup"
		}
		s.logger.Error().Err(err).Msg(msg)
	})
}

func (s *Syncer) syncOnce(ctx context.Context) error {
	now := s.nowFunc()
	body, err := s.cal.Fetch(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("fetch status calendar failed; using cached snapshot if available")
		cached, ok, getErr := s.store.GetStatusRawSnapshot()
		if getErr != nil {
			return fmt.Errorf("get cached status raw snapshot: %w", getErr)
		}
		if !ok {
			return fmt.Errorf("fetch status calendar: %w", err)
		}
		events, err := parseStatusEvents([]byte(cached.Body), now)
		if err != nil {
			return fmt.Errorf("parse cached status raw snapshot: %w", err)
		}
		return s.syncStatus(ctx, events, nil)
	}

	raw, events, err := statusRawSnapshot(body, now)
	if err != nil {
		return err
	}
	return s.syncStatus(ctx, events, raw)
}

// syncStatus computes and syncs the current status to all targets.
func (s *Syncer) syncStatus(ctx context.Context, events []ChangedEvent, raw *store.CalendarSnapshot) error {
	now := s.nowFunc()
	st := currentStatus(events, now)

	// Push to every target, collecting errors.
	var errs []error
	for _, tgt := range s.targets {
		if err := tgt.Sync(ctx, st); err != nil {
			s.logger.Error().Err(err).Msg("sync target")
			errs = append(errs, err)
		}
	}

	stored := storeStatus(st)
	if raw != nil {
		if err := s.store.ReplaceStatusProjection(raw, stored); err != nil {
			errs = append(errs, fmt.Errorf("store status projection: %w", err))
		}
	} else if err := s.store.ReplaceStatusCurrent(stored); err != nil {
		errs = append(errs, fmt.Errorf("store status: %w", err))
	}

	if st != nil {
		s.logger.Info().Str("emoji", st.Emoji).Str("text", st.Text).Time("expiry", st.Expiration).Msg("synced status")
	} else {
		s.logger.Info().Msg("cleared status")
	}

	return errors.Join(errs...)
}

func statusRawSnapshot(body []byte, fetchedAt time.Time) (*store.CalendarSnapshot, []ChangedEvent, error) {
	parsed, err := ParseICalendar(body, fetchedAt.Add(-24*time.Hour), fetchedAt.Add(24*time.Hour))
	if err != nil {
		return nil, nil, fmt.Errorf("parse status calendar: %w", err)
	}
	return &store.CalendarSnapshot{
		Body:      string(body),
		Timezone:  parsed.Timezone,
		FetchedAt: fetchedAt.UTC(),
	}, changedEvents(parsed.Events), nil
}

func parseStatusEvents(body []byte, now time.Time) ([]ChangedEvent, error) {
	parsed, err := ParseICalendar(body, now.Add(-24*time.Hour), now.Add(24*time.Hour))
	if err != nil {
		return nil, err
	}
	return changedEvents(parsed.Events), nil
}

func changedEvents(parsedEvents []ParsedEvent) []ChangedEvent {
	events := make([]ChangedEvent, len(parsedEvents))
	for i, ev := range parsedEvents {
		events[i] = ChangedEvent{
			ID:        ev.ID,
			Summary:   ev.Summary,
			StartTime: ev.StartTime,
			EndTime:   ev.EndTime,
			Cancelled: ev.Cancelled,
		}
	}
	return events
}

func currentStatus(events []ChangedEvent, now time.Time) *target.Status {
	selected := -1
	for i := range events {
		ev := events[i]
		if ev.Cancelled || ev.StartTime.After(now) || !ev.EndTime.After(now) {
			continue
		}
		if selected == -1 || eventSortsBefore(ev, events[selected]) {
			selected = i
		}
	}
	if selected == -1 {
		return nil
	}

	ev := events[selected]
	return &target.Status{
		Emoji:      ":calendar:",
		Text:       ev.Summary,
		Expiration: ev.EndTime,
	}
}

func eventSortsBefore(a, b ChangedEvent) bool {
	if !a.EndTime.Equal(b.EndTime) {
		return a.EndTime.Before(b.EndTime)
	}
	if !a.StartTime.Equal(b.StartTime) {
		return a.StartTime.Before(b.StartTime)
	}
	return a.ID < b.ID
}

func storeStatus(st *target.Status) *store.Status {
	if st == nil {
		return nil
	}
	return &store.Status{
		Emoji:      st.Emoji,
		Text:       st.Text,
		Expiration: st.Expiration,
	}
}

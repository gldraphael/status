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

// calendarClient defines the interface for fetching calendar events.
type calendarClient interface {
	FetchEvents(ctx context.Context) ([]ChangedEvent, error)
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
	// Fetch all events from iCal.
	events, err := s.cal.FetchEvents(ctx)
	if err != nil {
		return fmt.Errorf("fetch events: %w", err)
	}

	// Store events and sync status.
	for _, ev := range events {
		stored := &store.Event{
			ID:        ev.ID,
			Summary:   ev.Summary,
			StartTime: ev.StartTime,
			EndTime:   ev.EndTime,
			Cancelled: ev.Cancelled,
		}
		if err := s.store.SetEvent(stored); err != nil {
			s.logger.Error().Err(err).Str("eventID", ev.ID).Msg("store event")
		}
	}

	return s.syncStatus(ctx)
}

// syncStatus computes and syncs the current status to all targets.
func (s *Syncer) syncStatus(ctx context.Context) error {
	now := s.nowFunc()
	active, err := s.store.ListActiveEvents(now)
	if err != nil {
		return fmt.Errorf("list active events: %w", err)
	}

	// Build the status to sync (nil = clear).
	var st *target.Status
	if len(active) > 0 {
		ev := active[0]
		st = &target.Status{
			Emoji:      ":calendar:",
			Text:       ev.Summary,
			Expiration: ev.EndTime,
		}
	}

	// Push to every target, collecting errors.
	var errs []error
	for _, tgt := range s.targets {
		if err := tgt.Sync(ctx, st); err != nil {
			s.logger.Error().Err(err).Msg("sync target")
			errs = append(errs, err)
		}
	}

	// Persist the effective status regardless of individual target errors.
	if st != nil {
		stored := &store.Status{Emoji: st.Emoji, Text: st.Text, Expiration: st.Expiration}
		if err := s.store.SetStatus(stored); err != nil {
			errs = append(errs, fmt.Errorf("store status: %w", err))
		}
		s.logger.Info().Str("emoji", st.Emoji).Str("text", st.Text).Time("expiry", st.Expiration).Msg("synced status")
	} else {
		if err := s.store.DeleteStatus(); err != nil {
			errs = append(errs, fmt.Errorf("delete status: %w", err))
		}
		s.logger.Info().Msg("cleared status")
	}

	return errors.Join(errs...)
}

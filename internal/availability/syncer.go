package availability

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/gldraphael/status/internal/calendar"
	"github.com/gldraphael/status/internal/poll"
	"github.com/gldraphael/status/internal/store"
)

type feedClient interface {
	Fetch(context.Context) ([]byte, error)
}

// Syncer periodically fetches the availability calendar and stores a snapshot.
type Syncer struct {
	store    *store.Store
	provider *Provider
	cal      feedClient
	logger   zerolog.Logger
	nowFunc  func() time.Time
}

// NewSyncer creates a new Syncer.
func NewSyncer(st *store.Store, provider *Provider, cal feedClient, logger zerolog.Logger) *Syncer {
	return &Syncer{
		store:    st,
		provider: provider,
		cal:      cal,
		logger:   logger,
		nowFunc:  time.Now,
	}
}

// Run starts the sync loop and blocks until ctx is cancelled.
func (s *Syncer) Run(ctx context.Context, interval time.Duration) error {
	return poll.Every(ctx, interval, func() error {
		return s.syncOnce(ctx)
	}, func(err error, startup bool) {
		msg := "sync availability cycle"
		if startup {
			msg = "sync availability on startup"
		}
		s.logger.Error().Err(err).Msg(msg)
	})
}

func (s *Syncer) syncOnce(ctx context.Context) error {
	var snap *store.CalendarSnapshot
	fetched := false

	body, err := s.cal.Fetch(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("fetch availability calendar failed; using cached snapshot if available")
		cached, ok, err := s.store.GetAvailabilityRawSnapshot()
		if err != nil {
			return fmt.Errorf("get cached availability raw snapshot: %w", err)
		}
		if !ok {
			return ErrSnapshotNotFound
		}
		snap = cached
	} else {
		timezone, err := calendar.ExtractICalendarTimezone(body)
		if err != nil {
			return fmt.Errorf("extract availability timezone: %w", err)
		}

		snap = &store.CalendarSnapshot{
			Body:      string(body),
			Timezone:  timezone,
			FetchedAt: s.nowFunc().UTC(),
		}
		fetched = true
	}

	// Change detection: compare current entries with last deployed ones.
	currentEntries, err := s.provider.ComputeEntriesJSONFromSnapshot(snap)
	if err != nil {
		return fmt.Errorf("compute current availability: %w", err)
	}
	if fetched {
		if err := s.store.ReplaceAvailabilityProjection(snap, currentEntries); err != nil {
			return fmt.Errorf("store availability projection: %w", err)
		}
	} else if err := s.store.SetAvailabilityCurrent(currentEntries); err != nil {
		return fmt.Errorf("store current availability: %w", err)
	}

	lastDeployed, ok, err := s.store.GetLastDeployedAvailability()
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to get last deployed availability")
	}

	if !ok || !bytes.Equal(currentEntries, lastDeployed) {
		if err := s.store.SetAvailabilityDirty(currentEntries); err != nil {
			s.logger.Error().Err(err).Msg("failed to set availability dirty flag")
		} else {
			s.logger.Info().Msg("availability changed; marked as dirty for deployment")
		}
	}

	return nil
}

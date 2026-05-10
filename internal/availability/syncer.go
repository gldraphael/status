package availability

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/gldraphael/status/internal/calendar"
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
}

// NewSyncer creates a new Syncer.
func NewSyncer(st *store.Store, provider *Provider, cal feedClient, logger zerolog.Logger) *Syncer {
	return &Syncer{
		store:    st,
		provider: provider,
		cal:      cal,
		logger:   logger,
	}
}

// Run starts the sync loop and blocks until ctx is cancelled.
func (s *Syncer) Run(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if err := s.syncOnce(ctx); err != nil {
		s.logger.Error().Err(err).Msg("sync availability on startup")
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.syncOnce(ctx); err != nil {
				s.logger.Error().Err(err).Msg("sync availability cycle")
			}
		}
	}
}

func (s *Syncer) syncOnce(ctx context.Context) error {
	body, err := s.cal.Fetch(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("fetch availability calendar failed; using cached snapshot if available")
	} else {
		timezone, err := calendar.ExtractICalendarTimezone(body)
		if err != nil {
			return fmt.Errorf("extract availability timezone: %w", err)
		}

		snap := &store.AvailabilitySnapshot{
			Body:      string(body),
			Timezone:  timezone,
			FetchedAt: time.Now().UTC(),
		}
		if err := s.store.SetAvailabilitySnapshot(snap); err != nil {
			return fmt.Errorf("store availability snapshot: %w", err)
		}
	}

	// Change detection: compare current entries with last deployed ones.
	currentEntries, err := s.provider.GetEntriesJSON(ctx)
	if err != nil {
		return fmt.Errorf("compute current availability: %w", err)
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

package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/gldraphael/status/internal/store"
)

// Client defines the minimal interface the Deployer needs to trigger a build.
type Client interface {
	Trigger(ctx context.Context) error
}

// Deployer periodically triggers builds using the provided Client.
type Deployer struct {
	client  Client
	store   *store.Store
	logger  zerolog.Logger
	nowFunc func() time.Time
}

// NewDeployer constructs a Deployer.
func NewDeployer(client Client, st *store.Store, logger zerolog.Logger) *Deployer {
	return &Deployer{client: client, store: st, logger: logger, nowFunc: time.Now}
}

// Run starts the deploy loop. It schedules the first deploy at the next time
// which is offset by one minute after the hour and then repeats at the configured interval.
// For example, with interval 10m the deploys happen at HH:01, HH:11, HH:21, ...
func (d *Deployer) Run(ctx context.Context, interval time.Duration) error {
	if interval < time.Minute {
		return fmt.Errorf("interval must be >= 1m")
	}

	now := d.nowFunc()
	first := nextAlignedTime(now, interval, 1)
	// Wait until the first scheduled time.
	if wait := time.Until(first); wait > 0 {
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil
		case <-t.C:
		}
	}

	// Trigger on the scheduled time.
	d.triggerIfDirty(ctx, first)

	// Then repeat at the configured interval.
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			d.triggerIfDirty(ctx, d.nowFunc())
		}
	}
}

func (d *Deployer) triggerIfDirty(ctx context.Context, scheduledAt time.Time) {
	dirtyJSON, ok, err := d.store.GetAvailabilityDirty()
	if err != nil {
		d.logger.Error().Err(err).Msg("failed to check availability dirty flag; assuming clean and skipping deploy")
		return
	}

	if !ok {
		d.logger.Info().Time("scheduled_at", scheduledAt).Msg("skipping deploy: no availability changes detected")
		return
	}

	if err := d.client.Trigger(ctx); err != nil {
		d.logger.Error().Err(err).Time("scheduled_at", scheduledAt).Msg("deploy failed")
	} else {
		d.logger.Info().Time("scheduled_at", scheduledAt).Msg("deploy triggered")
		if err := d.store.SetLastDeployedAvailability(dirtyJSON); err != nil {
			d.logger.Error().Err(err).Msg("failed to update last deployed availability")
		}
		if err := d.store.ClearAvailabilityDirty(); err != nil {
			d.logger.Error().Err(err).Msg("failed to clear availability dirty flag")
		}
	}
}

// nextAlignedTime returns the next time at or after now where the schedule
// is anchored to the given offset minute in the current hour and repeats every interval.
func nextAlignedTime(now time.Time, interval time.Duration, offsetMinute int) time.Time {
	anchor := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), offsetMinute, 0, 0, now.Location())
	if anchor.After(now) {
		return anchor
	}
	elapsed := now.Sub(anchor)
	k := elapsed / interval
	if elapsed%interval == 0 {
		return anchor.Add(k * interval)
	}
	return anchor.Add((k + 1) * interval)
}

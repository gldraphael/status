package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

// Client defines the minimal interface the Deployer needs to trigger a build.
type Client interface {
	Trigger(ctx context.Context) error
}

// Deployer periodically triggers builds using the provided Client.
type Deployer struct {
	client Client
	logger zerolog.Logger
}

// NewDeployer constructs a Deployer.
func NewDeployer(client Client, logger zerolog.Logger) *Deployer {
	return &Deployer{client: client, logger: logger}
}

// Run starts the deploy loop. It schedules the first deploy at the next time
// which is offset by one minute after the hour and then repeats at the configured interval.
// For example, with interval 10m the deploys happen at HH:01, HH:11, HH:21, ...
func (d *Deployer) Run(ctx context.Context, interval time.Duration) error {
	if interval < time.Minute {
		return fmt.Errorf("interval must be >= 1m")
	}

	now := time.Now()
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
	if err := d.client.Trigger(ctx); err != nil {
		d.logger.Error().Err(err).Time("scheduled_at", first).Msg("deploy on schedule")
	} else {
		d.logger.Info().Time("scheduled_at", first).Msg("deploy triggered")
	}

	// Then repeat at the configured interval.
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := d.client.Trigger(ctx); err != nil {
				d.logger.Error().Err(err).Msg("deploy cycle")
			} else {
				d.logger.Info().Msg("deploy triggered")
			}
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

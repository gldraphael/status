package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/gldraphael/status/internal/availability"
	"github.com/gldraphael/status/internal/calendar"
	"github.com/gldraphael/status/internal/config"
	githubTarget "github.com/gldraphael/status/internal/github"
	"github.com/gldraphael/status/internal/server"
	"github.com/gldraphael/status/internal/store"
	"github.com/gldraphael/status/internal/target"
)

func main() {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()
	if err := run(logger); err != nil {
		logger.Fatal().Err(err).Msg("fatal error")
	}
}

func run(logger zerolog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Availability.Validate(); err != nil {
		return fmt.Errorf("validate availability config: %w", err)
	}

	// Clear any persisted data on startup to ensure a fresh sync from the calendar.
	// Ignore errors if directory doesn't exist or can't be cleared (e.g., in containers with restricted permissions).
	_ = os.RemoveAll(cfg.PebblePath)

	st, err := store.New(cfg.PebblePath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	calClient, err := calendar.NewClient(cfg.CalendarURL)
	if err != nil {
		return fmt.Errorf("create calendar client: %w", err)
	}

	targets := buildTargets(cfg)
	syncer := calendar.NewSyncer(st, calClient, targets, logger)
	var (
		availabilitySyncer *availability.Syncer
	)

	// Health-check endpoint.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	if cfg.Availability.IsEnabled {
		availabilityClient, err := availability.NewClient(cfg.Availability.CalendarURL)
		if err != nil {
			return fmt.Errorf("create availability client: %w", err)
		}
		availabilityBlocks, err := availability.ParseBlocks(cfg.Availability.Blocks)
		if err != nil {
			return fmt.Errorf("parse availability blocks: %w", err)
		}

		availabilitySyncer = availability.NewSyncer(st, availabilityClient, logger)
		mux.Handle("GET /api/availability", availability.NewHandler(st, cfg.Availability.APIKey, availabilityBlocks, logger))
	}

	// Start the sync loops only after all startup validation succeeds.
	go func() {
		if err := syncer.Run(ctx, 5*time.Minute); err != nil {
			logger.Error().Err(err).Msg("sync loop exited")
		}
	}()
	if availabilitySyncer != nil {
		go func() {
			if err := availabilitySyncer.Run(ctx, 5*time.Minute); err != nil {
				logger.Error().Err(err).Msg("availability sync loop exited")
			}
		}()
	}

	srv := server.New(cfg.Port, mux, logger)
	return srv.Start(ctx)
}

// buildTargets constructs the list of enabled status targets from config.
// A target is enabled when its token is non-empty.
func buildTargets(cfg *config.Config) []target.Target {
	var targets []target.Target
	if t := cfg.Targets.GitHub.Token; t != "" {
		targets = append(targets, githubTarget.NewTarget(t))
	}
	return targets
}

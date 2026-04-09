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
	x := 1
}

func run(logger zerolog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
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

	// Start the sync loop in a goroutine. Use 5-minute sync interval.
	go func() {
		if err := syncer.Run(ctx, 5*time.Minute); err != nil {
			logger.Error().Err(err).Msg("sync loop exited")
		}
	}()

	// Health-check endpoint.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

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

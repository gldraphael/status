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
	deploy "github.com/gldraphael/status/internal/deploy"
	"github.com/gldraphael/status/internal/feed"
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
	buildInterval, err := cfg.Build.IntervalDuration()
	if err != nil {
		return fmt.Errorf("validate build config: %w", err)
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

	// Health-check endpoint.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	availabilitySyncer, err := registerAvailability(ctx, cfg.Availability, st, mux, logger)
	if err != nil {
		return err
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

	// Start deploy loop if enabled.
	startDeployLoop(ctx, cfg.Build, buildInterval, st, logger)

	srv := server.New(cfg.Port, mux, logger)
	return srv.Start(ctx)
}

func registerAvailability(ctx context.Context, cfg config.AvailabilityConfig, st *store.Store, mux *http.ServeMux, logger zerolog.Logger) (*availability.Syncer, error) {
	if !cfg.IsEnabled {
		return nil, nil
	}

	workingHours, err := availability.ParseWorkingHours(cfg.WorkingHours.Start, cfg.WorkingHours.End)
	if err != nil {
		return nil, fmt.Errorf("parse availability working hours: %w", err)
	}
	availabilityBlocks, err := parseAvailabilityBlocks(cfg.Blocks)
	if err != nil {
		return nil, fmt.Errorf("parse availability blocks: %w", err)
	}
	availabilityClient, err := feed.NewClient(cfg.CalendarURL, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("create availability client: %w", err)
	}

	if cfg.ExcludeEnglandBankHolidays {
		holidayClient, err := availability.NewHolidayClient()
		if err != nil {
			return nil, fmt.Errorf("create bank holiday client: %w", err)
		}
		if err := availability.SyncHolidaySnapshot(ctx, st, holidayClient, logger); err != nil {
			return nil, fmt.Errorf("seed bank holidays: %w", err)
		}
	}

	provider := availability.NewProvider(st, availabilityBlocks, workingHours, cfg.ExcludeEnglandBankHolidays)
	mux.Handle("GET /api/availability", availability.NewHandler(provider, cfg.APIKey, logger))
	return availability.NewSyncer(st, provider, availabilityClient, logger), nil
}

func parseAvailabilityBlocks(blocks []config.AvailabilityBlockConfig) ([]availability.Block, error) {
	parsed := make([]availability.Block, 0, len(blocks))
	for i, block := range blocks {
		parsedBlock, err := availability.ParseBlock(block.Name, block.Start, block.End)
		if err != nil {
			return nil, fmt.Errorf("availability.blocks[%d]: %w", i, err)
		}
		parsed = append(parsed, parsedBlock)
	}
	return parsed, nil
}

func startDeployLoop(ctx context.Context, cfg config.BuildConfig, interval time.Duration, st *store.Store, logger zerolog.Logger) {
	if !cfg.IsEnabled {
		return
	}

	client := deploy.NewHookClient(cfg.CfDeployHook)
	deployer := deploy.NewDeployer(client, st, logger)
	go func() {
		if err := deployer.Run(ctx, interval); err != nil {
			logger.Error().Err(err).Msg("build deploy loop exited")
		}
	}()
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

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	var statusInterval time.Duration
	if cfg.Status.Targets.Enabled() {
		statusInterval, err = cfg.Status.Sources.ICal.IntervalDuration("status.sources.ical.interval")
		if err != nil {
			return fmt.Errorf("validate status source config: %w", err)
		}
	}
	var availabilityInterval time.Duration
	if cfg.Availability.Enabled() {
		availabilityInterval, err = cfg.Availability.Sources.ICal.IntervalDuration("availability.sources.ical.interval")
		if err != nil {
			return fmt.Errorf("validate availability source config: %w", err)
		}
	}
	cloudflarePagesInterval, err := cfg.Availability.Targets.CloudflarePages.IntervalDuration()
	if err != nil {
		return fmt.Errorf("validate Cloudflare Pages target config: %w", err)
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

	targets := buildTargets(cfg.Status.Targets)
	var statusSyncer *calendar.Syncer
	if len(targets) > 0 {
		calClient, err := calendar.NewClient(cfg.Status.Sources.ICal.URL)
		if err != nil {
			return fmt.Errorf("create status calendar client: %w", err)
		}
		statusSyncer = calendar.NewSyncer(st, calClient, targets, logger)
	}

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
	if statusSyncer != nil {
		go func() {
			if err := statusSyncer.Run(ctx, statusInterval); err != nil {
				logger.Error().Err(err).Msg("status source loop exited")
			}
		}()
	}
	if availabilitySyncer != nil {
		go func() {
			if err := availabilitySyncer.Run(ctx, availabilityInterval); err != nil {
				logger.Error().Err(err).Msg("availability source loop exited")
			}
		}()
	}

	// Start deploy loop if enabled.
	startDeployLoop(ctx, cfg.Availability.Targets.CloudflarePages, cloudflarePagesInterval, st, logger)

	srv := server.New(cfg.Port, mux, logger)
	return srv.Start(ctx)
}

func registerAvailability(ctx context.Context, cfg config.AvailabilityConfig, st *store.Store, mux *http.ServeMux, logger zerolog.Logger) (*availability.Syncer, error) {
	if !cfg.Enabled() {
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
	availabilityClient, err := feed.NewClient(cfg.Sources.ICal.URL, 30*time.Second)
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
	if cfg.API.IsEnabled {
		mux.Handle("GET /api/availability", availability.NewHandler(provider, cfg.API.Key, logger))
	}
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

func startDeployLoop(ctx context.Context, cfg config.CloudflarePagesTargetConfig, interval time.Duration, st *store.Store, logger zerolog.Logger) {
	if !cfg.IsEnabled {
		return
	}

	client := deploy.NewHookClient(cfg.DeployHook)
	deployer := deploy.NewDeployer(client, st, logger)
	go func() {
		if err := deployer.Run(ctx, interval); err != nil {
			logger.Error().Err(err).Msg("Cloudflare Pages target loop exited")
		}
	}()
}

// buildTargets constructs the list of enabled status targets from config.
// A target is enabled when its token is non-empty.
func buildTargets(cfg config.StatusTargetsConfig) []target.Target {
	var targets []target.Target
	if t := strings.TrimSpace(cfg.GitHub.Token); t != "" {
		targets = append(targets, githubTarget.NewTarget(t))
	}
	return targets
}

package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"

	"github.com/gldraphael/status/internal/timeutil"
)

// Config holds application configuration.
type Config struct {
	Port       int    `koanf:"port"`
	PebblePath string `koanf:"pebble_path"`

	Status       StatusConfig       `koanf:"status"`
	Availability AvailabilityConfig `koanf:"availability"`
}

// StatusConfig configures status sources and targets.
type StatusConfig struct {
	Sources StatusSourcesConfig `koanf:"sources"`
	Targets StatusTargetsConfig `koanf:"targets"`
}

// StatusSourcesConfig configures status data sources.
type StatusSourcesConfig struct {
	ICal ICalSourceConfig `koanf:"ical"`
}

// ICalSourceConfig configures an iCal feed source.
type ICalSourceConfig struct {
	URL      string `koanf:"url"`
	Interval string `koanf:"interval"`
}

// StatusTargetsConfig holds the configuration for each supported status target.
// A target is enabled when its token is non-empty.
// Add a new field here to support additional targets in the future.
type StatusTargetsConfig struct {
	GitHub GitHubTargetConfig `koanf:"github"`
}

// GitHubTargetConfig configures the GitHub status target.
type GitHubTargetConfig struct {
	Token string `koanf:"token"` // personal access token; requires user scope
}

// Enabled reports whether the GitHub status target is configured.
func (g GitHubTargetConfig) Enabled() bool {
	return strings.TrimSpace(g.Token) != ""
}

// Enabled reports whether any status target is configured.
func (t StatusTargetsConfig) Enabled() bool {
	return t.GitHub.Enabled()
}

// AvailabilityConfig configures availability sources, serving, targets, and rules.
type AvailabilityConfig struct {
	Sources      AvailabilitySourcesConfig      `koanf:"sources"`
	API          AvailabilityAPIConfig          `koanf:"api"`
	Targets      AvailabilityTargetsConfig      `koanf:"targets"`
	Suppressions AvailabilitySuppressionsConfig `koanf:"suppressions"`
	Blocks       []AvailabilityBlockConfig      `koanf:"blocks"`
}

// AvailabilitySourcesConfig configures availability data sources.
type AvailabilitySourcesConfig struct {
	ICal ICalSourceConfig `koanf:"ical"`
}

// AvailabilityAPIConfig configures the optional availability HTTP API.
type AvailabilityAPIConfig struct {
	IsEnabled bool   `koanf:"is_enabled"`
	Key       string `koanf:"key"`
}

// AvailabilityTargetsConfig holds availability publish targets.
type AvailabilityTargetsConfig struct {
	CloudflarePages CloudflarePagesTargetConfig `koanf:"cloudflare_pages"`
}

// CloudflarePagesTargetConfig configures Cloudflare Pages build hook publishes.
type CloudflarePagesTargetConfig struct {
	IsEnabled  bool   `koanf:"is_enabled"`
	Interval   string `koanf:"interval"`
	DeployHook string `koanf:"deploy_hook"`
}

// Enabled reports whether the Cloudflare Pages target is configured.
func (c CloudflarePagesTargetConfig) Enabled() bool {
	return c.IsEnabled
}

// Enabled reports whether any availability target is configured.
func (t AvailabilityTargetsConfig) Enabled() bool {
	return t.CloudflarePages.Enabled()
}

// Enabled reports whether availability computation is needed.
func (a AvailabilityConfig) Enabled() bool {
	return a.API.IsEnabled || a.Targets.Enabled()
}

// AvailabilitySuppressionsConfig configures availability suppression rules.
type AvailabilitySuppressionsConfig struct {
	WorkingHours               AvailabilityWorkingHoursConfig `koanf:"working_hours"`
	ExcludeEnglandBankHolidays bool                           `koanf:"exclude_england_bank_holidays"`
}

// AvailabilityWorkingHoursConfig defines the weekday working-hours window.
type AvailabilityWorkingHoursConfig struct {
	Start string `koanf:"start"` // HH:MM, 24-hour clock
	End   string `koanf:"end"`   // HH:MM, 24-hour clock
}

// AvailabilityBlockConfig defines one ordered availability window.
type AvailabilityBlockConfig struct {
	Name  string `koanf:"name"`
	Start string `koanf:"start"` // HH:MM, 24-hour clock
	End   string `koanf:"end"`   // HH:MM, 24-hour clock
}

// envMapping maps environment variable names to koanf config keys.
// Only variables listed here are loaded; all others are ignored.
var envMapping = map[string]string{
	"PORT":                                                    "port",
	"PEBBLE_PATH":                                             "pebble_path",
	"STATUS_SOURCES_ICAL_URL":                                 "status.sources.ical.url",
	"STATUS_SOURCES_ICAL_INTERVAL":                            "status.sources.ical.interval",
	"STATUS_TARGETS_GITHUB_TOKEN":                             "status.targets.github.token",
	"AVAILABILITY_SOURCES_ICAL_URL":                           "availability.sources.ical.url",
	"AVAILABILITY_SOURCES_ICAL_INTERVAL":                      "availability.sources.ical.interval",
	"AVAILABILITY_API_IS_ENABLED":                             "availability.api.is_enabled",
	"AVAILABILITY_API_KEY":                                    "availability.api.key",
	"AVAILABILITY_SUPPRESSIONS_WORKING_HOURS_START":           "availability.suppressions.working_hours.start",
	"AVAILABILITY_SUPPRESSIONS_WORKING_HOURS_END":             "availability.suppressions.working_hours.end",
	"AVAILABILITY_SUPPRESSIONS_EXCLUDE_ENGLAND_BANK_HOLIDAYS": "availability.suppressions.exclude_england_bank_holidays",
	"AVAILABILITY_TARGETS_CLOUDFLARE_PAGES_IS_ENABLED":        "availability.targets.cloudflare_pages.is_enabled",
	"AVAILABILITY_TARGETS_CLOUDFLARE_PAGES_INTERVAL":          "availability.targets.cloudflare_pages.interval",
	"AVAILABILITY_TARGETS_CLOUDFLARE_PAGES_DEPLOY_HOOK":       "availability.targets.cloudflare_pages.deploy_hook",
}

// configFile is the optional YAML config file loaded between defaults and env vars.
const configFile = "config.yaml"

// Load reads configuration in the following precedence order (highest last wins):
//
//  1. Built-in defaults
//  2. config.yaml  (optional; silently skipped if absent)
//  3. Environment variables
func Load() (*Config, error) {
	k := koanf.New(".")

	// 1. Defaults.
	if err := k.Load(confmap.Provider(map[string]interface{}{
		"port":                               8080,
		"pebble_path":                        "./data",
		"status.sources.ical.interval":       "5m",
		"availability.sources.ical.interval": "5m",
		"availability.suppressions.working_hours.start":           "09:00",
		"availability.suppressions.working_hours.end":             "17:50",
		"availability.suppressions.exclude_england_bank_holidays": false,
		"availability.api.is_enabled":                             false,
		"availability.targets.cloudflare_pages.is_enabled":        false,
		"availability.targets.cloudflare_pages.interval":          "10m",
	}, "."), nil); err != nil {
		return nil, fmt.Errorf("load defaults: %w", err)
	}

	// 2. config.yaml - optional.
	if _, err := os.Stat(configFile); err == nil {
		if err := k.Load(file.Provider(configFile), yaml.Parser()); err != nil {
			return nil, fmt.Errorf("load %s: %w", configFile, err)
		}
	}

	// 3. Environment variables (only non-empty values override lower layers).
	overrides := make(map[string]interface{})
	for envKey, cfgKey := range envMapping {
		if val := os.Getenv(envKey); val != "" {
			overrides[cfgKey] = val
		}
	}
	if err := k.Load(confmap.Provider(overrides, "."), nil); err != nil {
		return nil, fmt.Errorf("load env: %w", err)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}

// Validate checks cross-feature configuration and feature-specific settings.
func (c Config) Validate() error {
	if err := c.Status.Validate(); err != nil {
		return err
	}
	if err := c.Availability.Validate(); err != nil {
		return err
	}
	return nil
}

// Validate checks status source and target settings.
func (s StatusConfig) Validate() error {
	if !s.Targets.Enabled() {
		return nil
	}
	if strings.TrimSpace(s.Sources.ICal.URL) == "" {
		return fmt.Errorf("status.sources.ical.url is required when at least one status target is enabled")
	}
	if _, err := s.Sources.ICal.IntervalDuration("status.sources.ical.interval"); err != nil {
		return err
	}
	return nil
}

// IntervalDuration returns the validated source polling interval.
func (s ICalSourceConfig) IntervalDuration(path string) (time.Duration, error) {
	if strings.TrimSpace(s.Interval) == "" {
		return 0, fmt.Errorf("%s is required", path)
	}
	dur, err := time.ParseDuration(s.Interval)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	if dur < time.Minute {
		return 0, fmt.Errorf("%s must be at least 1m", path)
	}
	return dur, nil
}

// Validate checks the configured availability settings.
func (a AvailabilityConfig) Validate() error {
	if !a.Enabled() {
		return nil
	}
	if strings.TrimSpace(a.Sources.ICal.URL) == "" {
		return fmt.Errorf("availability.sources.ical.url is required when availability is enabled")
	}
	if _, err := a.Sources.ICal.IntervalDuration("availability.sources.ical.interval"); err != nil {
		return err
	}
	if a.API.IsEnabled && strings.TrimSpace(a.API.Key) == "" {
		return fmt.Errorf("availability.api.key is required when availability API is enabled")
	}
	if a.Suppressions.WorkingHours.Start == "" {
		return fmt.Errorf("availability.suppressions.working_hours.start is required when availability is enabled")
	}
	if a.Suppressions.WorkingHours.End == "" {
		return fmt.Errorf("availability.suppressions.working_hours.end is required when availability is enabled")
	}
	if _, _, err := timeutil.ParseClockRange(a.Suppressions.WorkingHours.Start, a.Suppressions.WorkingHours.End); err != nil {
		return fmt.Errorf("availability.suppressions.working_hours: %w", err)
	}
	if len(a.Blocks) == 0 {
		return fmt.Errorf("availability.blocks is required when availability is enabled")
	}
	for i, block := range a.Blocks {
		if block.Name == "" {
			return fmt.Errorf("availability.blocks[%d].name is required", i)
		}
		if block.Start == "" {
			return fmt.Errorf("availability.blocks[%d].start is required", i)
		}
		if block.End == "" {
			return fmt.Errorf("availability.blocks[%d].end is required", i)
		}
		if _, _, err := timeutil.ParseClockRange(block.Start, block.End); err != nil {
			return fmt.Errorf("availability.blocks[%d]: %w", i, err)
		}
	}
	if err := a.Targets.CloudflarePages.Validate(); err != nil {
		return err
	}
	return nil
}

// Validate checks the Cloudflare Pages target settings.
func (c CloudflarePagesTargetConfig) Validate() error {
	if !c.IsEnabled {
		return nil
	}
	if _, err := c.IntervalDuration(); err != nil {
		return err
	}
	if strings.TrimSpace(c.DeployHook) == "" {
		return fmt.Errorf("availability.targets.cloudflare_pages.deploy_hook is required when Cloudflare Pages target is enabled")
	}
	return nil
}

// IntervalDuration returns the validated Cloudflare Pages publish interval.
func (c CloudflarePagesTargetConfig) IntervalDuration() (time.Duration, error) {
	if !c.IsEnabled {
		return 0, nil
	}
	if strings.TrimSpace(c.Interval) == "" {
		return 0, fmt.Errorf("availability.targets.cloudflare_pages.interval is required when Cloudflare Pages target is enabled")
	}
	dur, err := time.ParseDuration(c.Interval)
	if err != nil {
		return 0, fmt.Errorf("availability.targets.cloudflare_pages.interval: %w", err)
	}
	if dur < time.Minute {
		return 0, fmt.Errorf("availability.targets.cloudflare_pages.interval must be at least 1m")
	}
	return dur, nil
}

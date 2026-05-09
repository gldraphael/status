package config

import (
	"fmt"
	"os"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Config holds application configuration.
type Config struct {
	Port        int    `koanf:"port"`
	PebblePath  string `koanf:"pebble_path"`
	CalendarURL string `koanf:"calendar_url"` // iCal URL (e.g. https://calendar.google.com/calendar/ical/.../public/basic.ics)

	Targets      TargetsConfig      `koanf:"targets"`
	Availability AvailabilityConfig `koanf:"availability"`
}

// TargetsConfig holds the configuration for each supported status target.
// A target is enabled when its token is non-empty.
// Add a new field here to support additional targets in the future.
type TargetsConfig struct {
	GitHub GitHubTargetConfig `koanf:"github"`
}

// GitHubTargetConfig configures the GitHub status target.
type GitHubTargetConfig struct {
	Token string `koanf:"token"` // personal access token — requires user scope
}

// AvailabilityConfig configures the optional availability feature.
type AvailabilityConfig struct {
	IsEnabled   bool                      `koanf:"is_enabled"`
	CalendarURL string                    `koanf:"calendar_url"`
	APIKey      string                    `koanf:"api_key"`
	Blocks      []AvailabilityBlockConfig `koanf:"blocks"`
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
	"PORT":                      "port",
	"PEBBLE_PATH":               "pebble_path",
	"CALENDAR_URL":              "calendar_url",
	"GITHUB_TOKEN":              "targets.github.token",
	"GITHUB_USERNAME":           "targets.github.username",
	"AVAILABILITY_IS_ENABLED":   "availability.is_enabled",
	"AVAILABILITY_CALENDAR_URL": "availability.calendar_url",
	"AVAILABILITY_API_KEY":      "availability.api_key",
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
		"port":        8080,
		"pebble_path": "./data",
	}, "."), nil); err != nil {
		return nil, fmt.Errorf("load defaults: %w", err)
	}

	// 2. config.yaml — optional.
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

// Validate checks the configured feature-specific settings that cannot be
// validated by koanf decoding alone.
func (a AvailabilityConfig) Validate() error {
	if !a.IsEnabled {
		return nil
	}
	if a.CalendarURL == "" {
		return fmt.Errorf("availability.calendar_url is required when availability is enabled")
	}
	if a.APIKey == "" {
		return fmt.Errorf("availability.api_key is required when availability is enabled")
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
		start, err := time.Parse("15:04", block.Start)
		if err != nil {
			return fmt.Errorf("availability.blocks[%d].start: %w", i, err)
		}
		end, err := time.Parse("15:04", block.End)
		if err != nil {
			return fmt.Errorf("availability.blocks[%d].end: %w", i, err)
		}
		if !end.After(start) {
			return fmt.Errorf("availability.blocks[%d] end must be after start", i)
		}
	}
	return nil
}

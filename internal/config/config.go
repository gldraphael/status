package config

import (
	"fmt"
	"os"

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

	Targets TargetsConfig `koanf:"targets"`
}

// TargetsConfig holds the configuration for each supported status target.
// A target is enabled when its token is non-empty.
// Add a new field here to support additional targets in the future.
type TargetsConfig struct {
	GitHub GitHubTargetConfig `koanf:"github"`
}

// GitHubTargetConfig configures the GitHub status target.
type GitHubTargetConfig struct {
	Token    string `koanf:"token"`    // personal access token — requires user scope
}

// envMapping maps environment variable names to koanf config keys.
// Only variables listed here are loaded; all others are ignored.
var envMapping = map[string]string{
	"PORT":            "port",
	"PEBBLE_PATH":     "pebble_path",
	"CALENDAR_URL":    "calendar_url",
	"GITHUB_TOKEN":    "targets.github.token",
	"GITHUB_USERNAME": "targets.github.username",
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

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// chdir changes the working directory for the duration of the test.
// config.go looks for config.yaml in the cwd, so tests that need it
// must place it there.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}

var configEnvKeys = []string{
	"PORT",
	"PEBBLE_PATH",
	"STATUS_SOURCES_ICAL_URL",
	"STATUS_SOURCES_ICAL_INTERVAL",
	"STATUS_TARGETS_GITHUB_TOKEN",
	"AVAILABILITY_SOURCES_ICAL_URL",
	"AVAILABILITY_SOURCES_ICAL_INTERVAL",
	"AVAILABILITY_API_IS_ENABLED",
	"AVAILABILITY_API_KEY",
	"AVAILABILITY_WORKING_HOURS_START",
	"AVAILABILITY_WORKING_HOURS_END",
	"AVAILABILITY_EXCLUDE_ENGLAND_BANK_HOLIDAYS",
	"AVAILABILITY_TARGETS_CLOUDFLARE_PAGES_IS_ENABLED",
	"AVAILABILITY_TARGETS_CLOUDFLARE_PAGES_INTERVAL",
	"AVAILABILITY_TARGETS_CLOUDFLARE_PAGES_DEPLOY_HOOK",
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range configEnvKeys {
		t.Setenv(key, "")
	}
}

func writeConfigYAML(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_Defaults(t *testing.T) {
	chdir(t, t.TempDir())
	clearConfigEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port: got %d, want 8080", cfg.Port)
	}
	if cfg.PebblePath != "./data" {
		t.Errorf("PebblePath: got %q, want %q", cfg.PebblePath, "./data")
	}
	if cfg.Status.Sources.ICal.URL != "" {
		t.Errorf("Status.Sources.ICal.URL: got %q, want empty", cfg.Status.Sources.ICal.URL)
	}
	if cfg.Status.Sources.ICal.Interval != "5m" {
		t.Errorf("Status.Sources.ICal.Interval: got %q, want 5m", cfg.Status.Sources.ICal.Interval)
	}
	if cfg.Status.Targets.GitHub.Token != "" {
		t.Errorf("Status.Targets.GitHub.Token: got %q, want empty", cfg.Status.Targets.GitHub.Token)
	}
	if cfg.Availability.Sources.ICal.URL != "" {
		t.Errorf("Availability.Sources.ICal.URL: got %q, want empty", cfg.Availability.Sources.ICal.URL)
	}
	if cfg.Availability.Sources.ICal.Interval != "5m" {
		t.Errorf("Availability.Sources.ICal.Interval: got %q, want 5m", cfg.Availability.Sources.ICal.Interval)
	}
	if cfg.Availability.WorkingHours.Start != "09:00" || cfg.Availability.WorkingHours.End != "17:50" {
		t.Errorf("Availability.WorkingHours: got %+v, want start 09:00 end 17:50", cfg.Availability.WorkingHours)
	}
	if cfg.Availability.ExcludeEnglandBankHolidays {
		t.Error("expected bank holiday exclusion to be disabled by default")
	}
	if cfg.Availability.API.IsEnabled {
		t.Error("expected availability API to be disabled by default")
	}
	if cfg.Availability.Targets.CloudflarePages.IsEnabled {
		t.Error("expected Cloudflare Pages target to be disabled by default")
	}
	if cfg.Availability.Targets.CloudflarePages.Interval != "10m" {
		t.Errorf("CloudflarePages.Interval: got %q, want 10m", cfg.Availability.Targets.CloudflarePages.Interval)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	chdir(t, t.TempDir())
	clearConfigEnv(t)

	t.Setenv("PORT", "9090")
	t.Setenv("PEBBLE_PATH", "/tmp/mydb")
	t.Setenv("STATUS_SOURCES_ICAL_URL", "https://calendar.example.com/ical.ics")
	t.Setenv("STATUS_SOURCES_ICAL_INTERVAL", "15m")
	t.Setenv("STATUS_TARGETS_GITHUB_TOKEN", "gh-abc123")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port: got %d, want 9090", cfg.Port)
	}
	if cfg.PebblePath != "/tmp/mydb" {
		t.Errorf("PebblePath: got %q, want %q", cfg.PebblePath, "/tmp/mydb")
	}
	if cfg.Status.Sources.ICal.URL != "https://calendar.example.com/ical.ics" {
		t.Errorf("Status.Sources.ICal.URL: got %q", cfg.Status.Sources.ICal.URL)
	}
	if cfg.Status.Sources.ICal.Interval != "15m" {
		t.Errorf("Status.Sources.ICal.Interval: got %q", cfg.Status.Sources.ICal.Interval)
	}
	if cfg.Status.Targets.GitHub.Token != "gh-abc123" {
		t.Errorf("Status.Targets.GitHub.Token: got %q", cfg.Status.Targets.GitHub.Token)
	}
}

func TestLoad_AvailabilityFromEnv(t *testing.T) {
	chdir(t, t.TempDir())
	clearConfigEnv(t)

	t.Setenv("AVAILABILITY_SOURCES_ICAL_URL", "https://availability.example.com/ical.ics")
	t.Setenv("AVAILABILITY_SOURCES_ICAL_INTERVAL", "7m")
	t.Setenv("AVAILABILITY_API_IS_ENABLED", "true")
	t.Setenv("AVAILABILITY_API_KEY", "secret-key")
	t.Setenv("AVAILABILITY_WORKING_HOURS_START", "08:30")
	t.Setenv("AVAILABILITY_WORKING_HOURS_END", "17:15")
	t.Setenv("AVAILABILITY_EXCLUDE_ENGLAND_BANK_HOLIDAYS", "true")
	t.Setenv("AVAILABILITY_TARGETS_CLOUDFLARE_PAGES_IS_ENABLED", "true")
	t.Setenv("AVAILABILITY_TARGETS_CLOUDFLARE_PAGES_INTERVAL", "12m")
	t.Setenv("AVAILABILITY_TARGETS_CLOUDFLARE_PAGES_DEPLOY_HOOK", "https://example.com/hook")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Availability.Sources.ICal.URL != "https://availability.example.com/ical.ics" {
		t.Errorf("Availability.Sources.ICal.URL: got %q", cfg.Availability.Sources.ICal.URL)
	}
	if cfg.Availability.Sources.ICal.Interval != "7m" {
		t.Errorf("Availability.Sources.ICal.Interval: got %q", cfg.Availability.Sources.ICal.Interval)
	}
	if !cfg.Availability.API.IsEnabled {
		t.Fatal("expected availability API to be enabled from env")
	}
	if cfg.Availability.API.Key != "secret-key" {
		t.Errorf("Availability.API.Key: got %q", cfg.Availability.API.Key)
	}
	if cfg.Availability.WorkingHours.Start != "08:30" || cfg.Availability.WorkingHours.End != "17:15" {
		t.Errorf("Availability.WorkingHours: got %+v", cfg.Availability.WorkingHours)
	}
	if !cfg.Availability.ExcludeEnglandBankHolidays {
		t.Error("expected holiday exclusion to be enabled from env")
	}
	if !cfg.Availability.Targets.CloudflarePages.IsEnabled {
		t.Fatal("expected Cloudflare Pages target to be enabled from env")
	}
	if cfg.Availability.Targets.CloudflarePages.Interval != "12m" {
		t.Errorf("CloudflarePages.Interval: got %q", cfg.Availability.Targets.CloudflarePages.Interval)
	}
	if cfg.Availability.Targets.CloudflarePages.DeployHook != "https://example.com/hook" {
		t.Errorf("CloudflarePages.DeployHook: got %q", cfg.Availability.Targets.CloudflarePages.DeployHook)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	chdir(t, t.TempDir())
	clearConfigEnv(t)

	t.Setenv("PORT", "not-a-number")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid PORT, got nil")
	}
}

func TestLoad_FromYAML(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	clearConfigEnv(t)

	yaml := `
port: 7777
pebble_path: /yaml/data
status:
  sources:
    ical:
      url: https://yaml-cal.example.com/ical.ics
      interval: 12m
  targets:
    github:
      token: gh-yaml
`
	writeConfigYAML(t, dir, yaml)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 7777 {
		t.Errorf("Port: got %d, want 7777", cfg.Port)
	}
	if cfg.PebblePath != "/yaml/data" {
		t.Errorf("PebblePath: got %q", cfg.PebblePath)
	}
	if cfg.Status.Sources.ICal.URL != "https://yaml-cal.example.com/ical.ics" {
		t.Errorf("Status.Sources.ICal.URL: got %q", cfg.Status.Sources.ICal.URL)
	}
	if cfg.Status.Sources.ICal.Interval != "12m" {
		t.Errorf("Status.Sources.ICal.Interval: got %q", cfg.Status.Sources.ICal.Interval)
	}
	if cfg.Status.Targets.GitHub.Token != "gh-yaml" {
		t.Errorf("Status.Targets.GitHub.Token: got %q", cfg.Status.Targets.GitHub.Token)
	}
}

func TestLoad_AvailabilityFromYAML(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	clearConfigEnv(t)

	yaml := `
availability:
  sources:
    ical:
      url: https://availability.example.com/ical.ics
      interval: 8m
  api:
    is_enabled: true
    key: secret-yaml-key
  targets:
    cloudflare_pages:
      is_enabled: true
      interval: 14m
      deploy_hook: https://example.com/hook
  working_hours:
    start: "09:00"
    end: "17:50"
  exclude_england_bank_holidays: true
  blocks:
    - name: First half
      start: "09:00"
      end: "15:00"
    - name: Evening
      start: "17:30"
      end: "22:00"
`
	writeConfigYAML(t, dir, yaml)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Availability.Sources.ICal.URL != "https://availability.example.com/ical.ics" {
		t.Errorf("Availability.Sources.ICal.URL: got %q", cfg.Availability.Sources.ICal.URL)
	}
	if cfg.Availability.Sources.ICal.Interval != "8m" {
		t.Errorf("Availability.Sources.ICal.Interval: got %q", cfg.Availability.Sources.ICal.Interval)
	}
	if !cfg.Availability.API.IsEnabled {
		t.Fatal("expected availability API to be enabled")
	}
	if cfg.Availability.API.Key != "secret-yaml-key" {
		t.Errorf("Availability.API.Key: got %q", cfg.Availability.API.Key)
	}
	if cfg.Availability.WorkingHours.Start != "09:00" || cfg.Availability.WorkingHours.End != "17:50" {
		t.Errorf("Availability.WorkingHours: got %+v", cfg.Availability.WorkingHours)
	}
	if !cfg.Availability.ExcludeEnglandBankHolidays {
		t.Error("expected holiday exclusion to be enabled from YAML")
	}
	if !cfg.Availability.Targets.CloudflarePages.IsEnabled {
		t.Fatal("expected Cloudflare Pages target to be enabled")
	}
	if cfg.Availability.Targets.CloudflarePages.Interval != "14m" {
		t.Errorf("CloudflarePages.Interval: got %q", cfg.Availability.Targets.CloudflarePages.Interval)
	}
	if cfg.Availability.Targets.CloudflarePages.DeployHook != "https://example.com/hook" {
		t.Errorf("CloudflarePages.DeployHook: got %q", cfg.Availability.Targets.CloudflarePages.DeployHook)
	}
	if len(cfg.Availability.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(cfg.Availability.Blocks))
	}
	if cfg.Availability.Blocks[0].Name != "First half" || cfg.Availability.Blocks[1].Name != "Evening" {
		t.Errorf("unexpected blocks: %+v", cfg.Availability.Blocks)
	}
}

func TestLoad_EnvOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	clearConfigEnv(t)

	yaml := `
port: 7777
status:
  sources:
    ical:
      url: https://yaml-cal.example.com/ical.ics
      interval: 20m
  targets:
    github:
      token: gh-yaml
availability:
  sources:
    ical:
      url: https://yaml-availability.example.com/ical.ics
      interval: 30m
  api:
    is_enabled: false
    key: yaml-key
  working_hours:
    start: "10:00"
    end: "16:00"
  exclude_england_bank_holidays: false
`
	writeConfigYAML(t, dir, yaml)

	t.Setenv("PORT", "9999")
	t.Setenv("STATUS_TARGETS_GITHUB_TOKEN", "gh-env")
	t.Setenv("STATUS_SOURCES_ICAL_URL", "https://env-cal.example.com/ical.ics")
	t.Setenv("STATUS_SOURCES_ICAL_INTERVAL", "7m")
	t.Setenv("AVAILABILITY_SOURCES_ICAL_URL", "https://env-availability.example.com/ical.ics")
	t.Setenv("AVAILABILITY_SOURCES_ICAL_INTERVAL", "9m")
	t.Setenv("AVAILABILITY_API_IS_ENABLED", "true")
	t.Setenv("AVAILABILITY_API_KEY", "env-key")
	t.Setenv("AVAILABILITY_WORKING_HOURS_START", "08:45")
	t.Setenv("AVAILABILITY_WORKING_HOURS_END", "17:15")
	t.Setenv("AVAILABILITY_EXCLUDE_ENGLAND_BANK_HOLIDAYS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 9999 {
		t.Errorf("Port: got %d, want 9999", cfg.Port)
	}
	if cfg.Status.Targets.GitHub.Token != "gh-env" {
		t.Errorf("Status.Targets.GitHub.Token: got %q, want gh-env", cfg.Status.Targets.GitHub.Token)
	}
	if cfg.Status.Sources.ICal.URL != "https://env-cal.example.com/ical.ics" {
		t.Errorf("Status.Sources.ICal.URL: got %q, want env value", cfg.Status.Sources.ICal.URL)
	}
	if cfg.Status.Sources.ICal.Interval != "7m" {
		t.Errorf("Status.Sources.ICal.Interval: got %q, want env value", cfg.Status.Sources.ICal.Interval)
	}
	if cfg.Availability.Sources.ICal.URL != "https://env-availability.example.com/ical.ics" {
		t.Errorf("Availability.Sources.ICal.URL: got %q, want env value", cfg.Availability.Sources.ICal.URL)
	}
	if cfg.Availability.Sources.ICal.Interval != "9m" {
		t.Errorf("Availability.Sources.ICal.Interval: got %q, want env value", cfg.Availability.Sources.ICal.Interval)
	}
	if !cfg.Availability.API.IsEnabled {
		t.Fatal("expected env to enable availability API")
	}
	if cfg.Availability.API.Key != "env-key" {
		t.Errorf("Availability.API.Key: got %q", cfg.Availability.API.Key)
	}
	if cfg.Availability.WorkingHours.Start != "08:45" || cfg.Availability.WorkingHours.End != "17:15" {
		t.Errorf("Availability.WorkingHours: got %+v", cfg.Availability.WorkingHours)
	}
	if !cfg.Availability.ExcludeEnglandBankHolidays {
		t.Error("expected env to enable bank holiday exclusion")
	}
}

func TestLoad_YAMLOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	clearConfigEnv(t)

	yaml := `
port: 3000
status:
  sources:
    ical:
      interval: 11m
availability:
  sources:
    ical:
      interval: 13m
`
	writeConfigYAML(t, dir, yaml)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 3000 {
		t.Errorf("Port: got %d, want 3000", cfg.Port)
	}
	if cfg.Status.Sources.ICal.Interval != "11m" {
		t.Errorf("Status.Sources.ICal.Interval: got %q, want 11m", cfg.Status.Sources.ICal.Interval)
	}
	if cfg.Availability.Sources.ICal.Interval != "13m" {
		t.Errorf("Availability.Sources.ICal.Interval: got %q, want 13m", cfg.Availability.Sources.ICal.Interval)
	}
	if cfg.PebblePath != "./data" {
		t.Errorf("PebblePath: got %q, want default ./data", cfg.PebblePath)
	}
}

func TestLoad_MissingYAML(t *testing.T) {
	chdir(t, t.TempDir())
	clearConfigEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load without config.yaml: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port: got %d, want 8080", cfg.Port)
	}
}

func validAvailabilityConfig() AvailabilityConfig {
	return AvailabilityConfig{
		Sources: AvailabilitySourcesConfig{
			ICal: ICalSourceConfig{
				URL:      "https://example.com/availability.ics",
				Interval: "5m",
			},
		},
		API: AvailabilityAPIConfig{
			IsEnabled: true,
			Key:       "secret",
		},
		WorkingHours: AvailabilityWorkingHoursConfig{
			Start: "09:00",
			End:   "17:50",
		},
		Blocks: []AvailabilityBlockConfig{
			{Name: "Morning", Start: "09:00", End: "12:00"},
		},
	}
}

func TestAvailabilityValidate(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		if err := (AvailabilityConfig{}).Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	})

	t.Run("API enabled valid", func(t *testing.T) {
		cfg := validAvailabilityConfig()
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	})

	t.Run("Cloudflare Pages enabled valid without API key", func(t *testing.T) {
		cfg := validAvailabilityConfig()
		cfg.API = AvailabilityAPIConfig{}
		cfg.Targets.CloudflarePages = CloudflarePagesTargetConfig{
			IsEnabled:  true,
			Interval:   "10m",
			DeployHook: "https://example.com/hook",
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	})

	t.Run("API enabled missing key", func(t *testing.T) {
		cfg := validAvailabilityConfig()
		cfg.API.Key = ""
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for missing API key")
		}
	})

	t.Run("enabled missing source", func(t *testing.T) {
		cfg := validAvailabilityConfig()
		cfg.Sources.ICal.URL = ""
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for missing availability source")
		}
	})

	t.Run("enabled invalid working hours", func(t *testing.T) {
		cfg := validAvailabilityConfig()
		cfg.WorkingHours.Start = "bad-value"
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for invalid working hours")
		}
	})

	t.Run("enabled missing working hours start", func(t *testing.T) {
		cfg := validAvailabilityConfig()
		cfg.WorkingHours.Start = ""
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for missing working hours start")
		}
	})

	t.Run("enabled incomplete", func(t *testing.T) {
		cfg := AvailabilityConfig{API: AvailabilityAPIConfig{IsEnabled: true}}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for incomplete enabled availability config")
		}
	})
}

func TestConfigValidate(t *testing.T) {
	t.Run("availability only does not require status source", func(t *testing.T) {
		cfg := Config{
			Availability: validAvailabilityConfig(),
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	})

	t.Run("status target requires status source", func(t *testing.T) {
		cfg := Config{
			Status: StatusConfig{
				Sources: StatusSourcesConfig{
					ICal: ICalSourceConfig{Interval: "5m"},
				},
				Targets: StatusTargetsConfig{
					GitHub: GitHubTargetConfig{Token: "gh-token"},
				},
			},
		}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for missing status source")
		}
	})

	t.Run("status target with source", func(t *testing.T) {
		cfg := Config{
			Status: StatusConfig{
				Sources: StatusSourcesConfig{
					ICal: ICalSourceConfig{
						URL:      "https://example.com/status.ics",
						Interval: "5m",
					},
				},
				Targets: StatusTargetsConfig{
					GitHub: GitHubTargetConfig{Token: "gh-token"},
				},
			},
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	})
}

func TestICalSourceIntervalDuration(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		dur, err := (ICalSourceConfig{Interval: "5m"}).IntervalDuration("status.sources.ical.interval")
		if err != nil {
			t.Fatalf("IntervalDuration: %v", err)
		}
		if dur != 5*time.Minute {
			t.Fatalf("duration: got %v, want 5m", dur)
		}
	})

	t.Run("missing", func(t *testing.T) {
		if _, err := (ICalSourceConfig{}).IntervalDuration("status.sources.ical.interval"); err == nil {
			t.Fatal("expected error for missing interval")
		}
	})

	t.Run("too short", func(t *testing.T) {
		if _, err := (ICalSourceConfig{Interval: "30s"}).IntervalDuration("status.sources.ical.interval"); err == nil {
			t.Fatal("expected error for short interval")
		}
	})
}

func TestCloudflarePagesTargetValidateAndIntervalDuration(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		cfg := CloudflarePagesTargetConfig{}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
		dur, err := cfg.IntervalDuration()
		if err != nil {
			t.Fatalf("IntervalDuration: %v", err)
		}
		if dur != 0 {
			t.Fatalf("duration: got %v, want 0", dur)
		}
	})

	t.Run("enabled valid", func(t *testing.T) {
		cfg := CloudflarePagesTargetConfig{
			IsEnabled:  true,
			Interval:   "10m",
			DeployHook: "https://example.com/hook",
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
		dur, err := cfg.IntervalDuration()
		if err != nil {
			t.Fatalf("IntervalDuration: %v", err)
		}
		if dur != 10*time.Minute {
			t.Fatalf("duration: got %v, want 10m", dur)
		}
	})

	t.Run("interval too short", func(t *testing.T) {
		cfg := CloudflarePagesTargetConfig{
			IsEnabled:  true,
			Interval:   "30s",
			DeployHook: "https://example.com/hook",
		}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for short interval")
		}
	})

	t.Run("missing hook", func(t *testing.T) {
		cfg := CloudflarePagesTargetConfig{
			IsEnabled: true,
			Interval:  "10m",
		}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for missing deploy hook")
		}
	})
}

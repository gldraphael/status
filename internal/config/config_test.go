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

func TestLoad_Defaults(t *testing.T) {
	// Work in a temp dir with no config.yaml.
	chdir(t, t.TempDir())

	for _, key := range []string{"PORT", "PEBBLE_PATH", "CALENDAR_URL", "GITHUB_TOKEN", "AVAILABILITY_IS_ENABLED", "AVAILABILITY_CALENDAR_URL", "AVAILABILITY_API_KEY", "AVAILABILITY_WORKING_HOURS_START", "AVAILABILITY_WORKING_HOURS_END", "AVAILABILITY_EXCLUDE_ENGLAND_BANK_HOLIDAYS", "BUILD_IS_ENABLED", "BUILD_INTERVAL", "BUILD_CF_DEPLOY_HOOK"} {
		t.Setenv(key, "")
	}

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
	if cfg.CalendarURL != "" {
		t.Errorf("CalendarURL: got %q, want empty (no default)", cfg.CalendarURL)
	}
	if cfg.Availability.WorkingHours.Start != "09:00" || cfg.Availability.WorkingHours.End != "17:50" {
		t.Errorf("Availability.WorkingHours: got %+v, want start 09:00 end 17:50", cfg.Availability.WorkingHours)
	}
	if cfg.Availability.ExcludeEnglandBankHolidays {
		t.Error("expected bank holiday exclusion to be disabled by default")
	}
	if cfg.Availability.IsEnabled {
		t.Error("expected availability to be disabled by default")
	}
}

func TestLoad_FromEnv(t *testing.T) {
	chdir(t, t.TempDir())

	t.Setenv("PORT", "9090")
	t.Setenv("PEBBLE_PATH", "/tmp/mydb")
	t.Setenv("CALENDAR_URL", "https://calendar.example.com/ical.ics")
	t.Setenv("GITHUB_TOKEN", "gh-abc123")
	t.Setenv("AVAILABILITY_IS_ENABLED", "")
	t.Setenv("AVAILABILITY_CALENDAR_URL", "")
	t.Setenv("AVAILABILITY_API_KEY", "")
	t.Setenv("AVAILABILITY_WORKING_HOURS_START", "")
	t.Setenv("AVAILABILITY_WORKING_HOURS_END", "")
	t.Setenv("AVAILABILITY_EXCLUDE_ENGLAND_BANK_HOLIDAYS", "")

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
	if cfg.CalendarURL != "https://calendar.example.com/ical.ics" {
		t.Errorf("CalendarURL: got %q", cfg.CalendarURL)
	}
	if cfg.Targets.GitHub.Token != "gh-abc123" {
		t.Errorf("Targets.GitHub.Token: got %q", cfg.Targets.GitHub.Token)
	}
}

func TestLoad_AvailabilityFromEnv(t *testing.T) {
	chdir(t, t.TempDir())

	t.Setenv("PORT", "")
	t.Setenv("PEBBLE_PATH", "")
	t.Setenv("CALENDAR_URL", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("AVAILABILITY_IS_ENABLED", "true")
	t.Setenv("AVAILABILITY_CALENDAR_URL", "https://availability.example.com/ical.ics")
	t.Setenv("AVAILABILITY_API_KEY", "secret-key")
	t.Setenv("AVAILABILITY_WORKING_HOURS_START", "08:30")
	t.Setenv("AVAILABILITY_WORKING_HOURS_END", "17:15")
	t.Setenv("AVAILABILITY_EXCLUDE_ENGLAND_BANK_HOLIDAYS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Availability.IsEnabled {
		t.Fatal("expected availability to be enabled from env")
	}
	if cfg.Availability.CalendarURL != "https://availability.example.com/ical.ics" {
		t.Errorf("Availability.CalendarURL: got %q", cfg.Availability.CalendarURL)
	}
	if cfg.Availability.APIKey != "secret-key" {
		t.Errorf("Availability.APIKey: got %q", cfg.Availability.APIKey)
	}
	if cfg.Availability.WorkingHours.Start != "08:30" || cfg.Availability.WorkingHours.End != "17:15" {
		t.Errorf("Availability.WorkingHours: got %+v", cfg.Availability.WorkingHours)
	}
	if !cfg.Availability.ExcludeEnglandBankHolidays {
		t.Error("expected holiday exclusion to be enabled from env")
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	chdir(t, t.TempDir())
	t.Setenv("PEBBLE_PATH", "")
	t.Setenv("CALENDAR_URL", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("PORT", "not-a-number")
	t.Setenv("AVAILABILITY_IS_ENABLED", "")
	t.Setenv("AVAILABILITY_CALENDAR_URL", "")
	t.Setenv("AVAILABILITY_API_KEY", "")
	t.Setenv("AVAILABILITY_WORKING_HOURS_START", "")
	t.Setenv("AVAILABILITY_WORKING_HOURS_END", "")
	t.Setenv("AVAILABILITY_EXCLUDE_ENGLAND_BANK_HOLIDAYS", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid PORT, got nil")
	}
}

func TestLoad_FromYAML(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	// Clear env vars so they don't interfere with YAML loading
	t.Setenv("PORT", "")
	t.Setenv("PEBBLE_PATH", "")
	t.Setenv("CALENDAR_URL", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("AVAILABILITY_IS_ENABLED", "")
	t.Setenv("AVAILABILITY_CALENDAR_URL", "")
	t.Setenv("AVAILABILITY_API_KEY", "")
	t.Setenv("AVAILABILITY_WORKING_HOURS_START", "")
	t.Setenv("AVAILABILITY_WORKING_HOURS_END", "")
	t.Setenv("AVAILABILITY_EXCLUDE_ENGLAND_BANK_HOLIDAYS", "")

	yaml := `
port: 7777
pebble_path: /yaml/data
calendar_url: https://yaml-cal.example.com/ical.ics
targets:
  github:
    token: gh-yaml
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

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
	if cfg.CalendarURL != "https://yaml-cal.example.com/ical.ics" {
		t.Errorf("CalendarURL: got %q", cfg.CalendarURL)
	}
	if cfg.Targets.GitHub.Token != "gh-yaml" {
		t.Errorf("Targets.GitHub.Token: got %q", cfg.Targets.GitHub.Token)
	}
}

func TestLoad_AvailabilityFromYAML(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	t.Setenv("PORT", "")
	t.Setenv("PEBBLE_PATH", "")
	t.Setenv("CALENDAR_URL", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("AVAILABILITY_IS_ENABLED", "")
	t.Setenv("AVAILABILITY_CALENDAR_URL", "")
	t.Setenv("AVAILABILITY_API_KEY", "")
	t.Setenv("AVAILABILITY_WORKING_HOURS_START", "")
	t.Setenv("AVAILABILITY_WORKING_HOURS_END", "")
	t.Setenv("AVAILABILITY_EXCLUDE_ENGLAND_BANK_HOLIDAYS", "")

	yaml := `
availability:
  is_enabled: true
  calendar_url: https://availability.example.com/ical.ics
  api_key: secret-yaml-key
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
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Availability.IsEnabled {
		t.Fatal("expected availability to be enabled")
	}
	if cfg.Availability.CalendarURL != "https://availability.example.com/ical.ics" {
		t.Errorf("Availability.CalendarURL: got %q", cfg.Availability.CalendarURL)
	}
	if cfg.Availability.APIKey != "secret-yaml-key" {
		t.Errorf("Availability.APIKey: got %q", cfg.Availability.APIKey)
	}
	if cfg.Availability.WorkingHours.Start != "09:00" || cfg.Availability.WorkingHours.End != "17:50" {
		t.Errorf("Availability.WorkingHours: got %+v", cfg.Availability.WorkingHours)
	}
	if !cfg.Availability.ExcludeEnglandBankHolidays {
		t.Error("expected holiday exclusion to be enabled from YAML")
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

	t.Setenv("PEBBLE_PATH", "")
	t.Setenv("CALENDAR_URL", "")
	t.Setenv("GITHUB_TOKEN", "")
	yaml := `
port: 7777
calendar_url: https://yaml-cal.example.com/ical.ics
availability:
  working_hours:
    start: "10:00"
    end: "16:00"
  exclude_england_bank_holidays: false
targets:
  github:
    token: gh-yaml
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

	// Env var must win over yaml.
	t.Setenv("GITHUB_TOKEN", "gh-env")
	t.Setenv("PORT", "9999")
	t.Setenv("CALENDAR_URL", "https://env-cal.example.com/ical.ics")
	t.Setenv("AVAILABILITY_IS_ENABLED", "")
	t.Setenv("AVAILABILITY_CALENDAR_URL", "")
	t.Setenv("AVAILABILITY_API_KEY", "")
	t.Setenv("AVAILABILITY_WORKING_HOURS_START", "")
	t.Setenv("AVAILABILITY_WORKING_HOURS_END", "")
	t.Setenv("AVAILABILITY_EXCLUDE_ENGLAND_BANK_HOLIDAYS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Targets.GitHub.Token != "gh-env" {
		t.Errorf("Targets.GitHub.Token: got %q, want gh-env (env should beat yaml)", cfg.Targets.GitHub.Token)
	}
	if cfg.Port != 9999 {
		t.Errorf("Port: got %d, want 9999 (env should beat yaml)", cfg.Port)
	}
	if cfg.CalendarURL != "https://env-cal.example.com/ical.ics" {
		t.Errorf("CalendarURL: got %q, want env value", cfg.CalendarURL)
	}
}

func TestLoad_AvailabilityEnvOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	t.Setenv("PORT", "")
	t.Setenv("PEBBLE_PATH", "")
	t.Setenv("CALENDAR_URL", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("AVAILABILITY_IS_ENABLED", "")
	t.Setenv("AVAILABILITY_CALENDAR_URL", "")
	t.Setenv("AVAILABILITY_API_KEY", "")
	t.Setenv("AVAILABILITY_WORKING_HOURS_START", "")
	t.Setenv("AVAILABILITY_WORKING_HOURS_END", "")
	t.Setenv("AVAILABILITY_EXCLUDE_ENGLAND_BANK_HOLIDAYS", "")

	yaml := `
availability:
  is_enabled: false
  calendar_url: https://yaml-availability.example.com/ical.ics
  api_key: yaml-key
  working_hours:
    start: "10:00"
    end: "16:00"
  exclude_england_bank_holidays: false
  blocks:
    - name: Morning
      start: "09:00"
      end: "12:00"
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AVAILABILITY_IS_ENABLED", "true")
	t.Setenv("AVAILABILITY_CALENDAR_URL", "https://env-availability.example.com/ical.ics")
	t.Setenv("AVAILABILITY_API_KEY", "env-key")
	t.Setenv("AVAILABILITY_WORKING_HOURS_START", "08:45")
	t.Setenv("AVAILABILITY_WORKING_HOURS_END", "17:15")
	t.Setenv("AVAILABILITY_EXCLUDE_ENGLAND_BANK_HOLIDAYS", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Availability.IsEnabled {
		t.Fatal("expected env to enable availability")
	}
	if cfg.Availability.CalendarURL != "https://env-availability.example.com/ical.ics" {
		t.Errorf("Availability.CalendarURL: got %q", cfg.Availability.CalendarURL)
	}
	if cfg.Availability.APIKey != "env-key" {
		t.Errorf("Availability.APIKey: got %q", cfg.Availability.APIKey)
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

	// Clear env vars so they don't interfere with YAML loading
	t.Setenv("PORT", "")
	t.Setenv("PEBBLE_PATH", "")
	t.Setenv("CALENDAR_URL", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("AVAILABILITY_IS_ENABLED", "")
	t.Setenv("AVAILABILITY_CALENDAR_URL", "")
	t.Setenv("AVAILABILITY_API_KEY", "")
	t.Setenv("AVAILABILITY_WORKING_HOURS_START", "")
	t.Setenv("AVAILABILITY_WORKING_HOURS_END", "")
	t.Setenv("AVAILABILITY_EXCLUDE_ENGLAND_BANK_HOLIDAYS", "")

	yaml := `
port: 3000
calendar_url: https://cal.example.com/ical.ics
`
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 3000 {
		t.Errorf("Port: got %d, want 3000 (yaml should beat default)", cfg.Port)
	}
	if cfg.CalendarURL != "https://cal.example.com/ical.ics" {
		t.Errorf("CalendarURL: got %q", cfg.CalendarURL)
	}
	// Other defaults should remain.
	if cfg.PebblePath != "./data" {
		t.Errorf("PebblePath: got %q, want default ./data", cfg.PebblePath)
	}
}

func TestLoad_MissingYAML(t *testing.T) {
	// No config.yaml in the temp dir — should load without error.
	chdir(t, t.TempDir())
	t.Setenv("PORT", "")
	t.Setenv("PEBBLE_PATH", "")
	t.Setenv("CALENDAR_URL", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("AVAILABILITY_IS_ENABLED", "")
	t.Setenv("AVAILABILITY_CALENDAR_URL", "")
	t.Setenv("AVAILABILITY_API_KEY", "")
	t.Setenv("AVAILABILITY_WORKING_HOURS_START", "")
	t.Setenv("AVAILABILITY_WORKING_HOURS_END", "")
	t.Setenv("AVAILABILITY_EXCLUDE_ENGLAND_BANK_HOLIDAYS", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load without config.yaml: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port: got %d, want 8080 (default)", cfg.Port)
	}
}

func TestAvailabilityValidate(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		if err := (AvailabilityConfig{}).Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	})

	t.Run("enabled valid", func(t *testing.T) {
		cfg := AvailabilityConfig{
			IsEnabled:   true,
			CalendarURL: "https://example.com/ical.ics",
			APIKey:      "secret",
			WorkingHours: AvailabilityWorkingHoursConfig{
				Start: "09:00",
				End:   "17:50",
			},
			Blocks: []AvailabilityBlockConfig{
				{Name: "Morning", Start: "09:00", End: "12:00"},
			},
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	})

	t.Run("enabled invalid working hours", func(t *testing.T) {
		cfg := AvailabilityConfig{
			IsEnabled:   true,
			CalendarURL: "https://example.com/ical.ics",
			APIKey:      "secret",
			WorkingHours: AvailabilityWorkingHoursConfig{
				Start: "bad-value",
				End:   "17:50",
			},
			Blocks: []AvailabilityBlockConfig{
				{Name: "Morning", Start: "09:00", End: "12:00"},
			},
		}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for invalid working hours")
		}
	})

	t.Run("enabled missing working hours start", func(t *testing.T) {
		cfg := AvailabilityConfig{
			IsEnabled:   true,
			CalendarURL: "https://example.com/ical.ics",
			APIKey:      "secret",
			WorkingHours: AvailabilityWorkingHoursConfig{
				End: "17:50",
			},
			Blocks: []AvailabilityBlockConfig{
				{Name: "Morning", Start: "09:00", End: "12:00"},
			},
		}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for missing working hours start")
		}
	})

	t.Run("enabled incomplete", func(t *testing.T) {
		cfg := AvailabilityConfig{IsEnabled: true}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for incomplete enabled availability config")
		}
	})
}

func TestBuildValidateAndIntervalDuration(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		cfg := BuildConfig{}
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
		cfg := BuildConfig{
			IsEnabled:    true,
			Interval:     "10m",
			CfDeployHook: "https://example.com/hook",
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
		cfg := BuildConfig{
			IsEnabled:    true,
			Interval:     "30s",
			CfDeployHook: "https://example.com/hook",
		}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for short interval")
		}
	})

	t.Run("missing hook", func(t *testing.T) {
		cfg := BuildConfig{
			IsEnabled: true,
			Interval:  "10m",
		}
		if err := cfg.Validate(); err == nil {
			t.Fatal("expected error for missing deploy hook")
		}
	})
}

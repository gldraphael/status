package config

import (
	"os"
	"path/filepath"
	"testing"
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

	for _, key := range []string{"PORT", "PEBBLE_PATH", "CALENDAR_URL", "GITHUB_TOKEN"} {
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
}

func TestLoad_FromEnv(t *testing.T) {
	chdir(t, t.TempDir())

	t.Setenv("PORT", "9090")
	t.Setenv("PEBBLE_PATH", "/tmp/mydb")
	t.Setenv("CALENDAR_URL", "https://calendar.example.com/ical.ics")
	t.Setenv("GITHUB_TOKEN", "gh-abc123")

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

func TestLoad_InvalidPort(t *testing.T) {
	chdir(t, t.TempDir())
	t.Setenv("PORT", "not-a-number")
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
	t.Setenv("GITHUB_USERNAME", "")

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

func TestLoad_EnvOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	yaml := `
port: 7777
calendar_url: https://yaml-cal.example.com/ical.ics
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

func TestLoad_YAMLOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	// Clear env vars so they don't interfere with YAML loading
	t.Setenv("PORT", "")
	t.Setenv("PEBBLE_PATH", "")
	t.Setenv("CALENDAR_URL", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_USERNAME", "")

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
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load without config.yaml: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port: got %d, want 8080 (default)", cfg.Port)
	}
}

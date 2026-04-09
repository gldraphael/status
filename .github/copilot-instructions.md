# Copilot Instructions

## Project Overview

`status` is a personal Go application to sync events from a "Status" calendar with multiple platforms automatically.

- **Multi-target status sync**: When a calendar event starts/ends, the user's status is updated on all enabled targets (GitHub, etc.).
- **Polling-based**: Fetches events from a public iCal URL every 5 minutes. No webhooks or Google Calendar API required.
- **Datastore**: Uses [Pebble](https://github.com/cockroachdb/pebble) (an LSM-tree key-value store) as the embedded database.
- **O(1) status lookups**: The data model is designed for constant-time status retrieval.
- **Extensible targets**: New status targets can be added by implementing the `target.Target` interface.

## Architecture

```
iCal URL (public calendar feed)
        │
        ▼ (poll every 5 min)
  Syncer (calendar/handler.go)  ──── Pebble datastore
        │
        ├── status → O(1) current-status lookup
        │
        └── Target[]  ── GitHub, [future targets]
            ├── internal/github  (personal access token)
            └── internal/{new}
```

### Key components

- **`main.go`** — Entrypoint; wires up config, store, calendar client, targets, and polling loop
- **`internal/calendar/handler.go`** — Syncer: polls calendar, computes active events, syncs to all targets
- **`internal/calendar/client.go`** — Fetches events from iCal URL
- **`internal/calendar/ical.go`** — RFC 5545 iCal parser (handles line folding, timezones, cancellations)
- **`internal/target/target.go`** — Target interface; `Status` struct (emoji, text, expiration)
- **`internal/github/target.go`** — GitHub target implementation
- **`internal/server/server.go`** — HTTP server for health checks and metrics
- **`internal/store/store.go`** — Pebble wrapper for status and event storage
- **`internal/config/config.go`** — Three-layer config (defaults → config.yaml → env vars)

### Sync flow

1. `Syncer.Run()` starts a ticker that runs every 5 minutes
2. On each tick (and on startup), `syncOnce()` is called
3. `Syncer.syncOnce()` calls `Client.FetchEvents()` to get all events from the iCal URL
4. Events are parsed (RFC 5545 format) and stored in Pebble
5. `Syncer.syncStatus()` computes which events are currently active
6. For each active event, a status is built; if no events are active, status is nil (to clear)
7. For each target, `target.Sync()` is called with either the active status or nil
8. Errors from individual targets are logged and joined; status is persisted regardless

## Build, Test & Lint

```sh
# Build
go build ./...

# Run all tests
go test ./...

# Run a single test
go test ./... -run TestName

# Vet
go vet ./...

# Tidy dependencies
go mod tidy
```

## Key Conventions

- **iCal URL**: The app reads a public iCal URL (e.g., Google Calendar public export, CalDAV server, etc.). No OAuth or authentication required. Ensure the URL is publicly accessible.
- **Polling interval**: Currently hardcoded to 5 minutes in `main.go`. To change, adjust the argument to `syncer.Run()`.
- **Pebble key design**: Keys are structured for efficient event storage and status lookup. Keys follow the pattern:
  - `status` — current user status (single-tenant)
  - `event:{eventID}` — calendar event state
  - `channel:{channelID}` — push notification channel registration
  - `sync:{calendarID}` — incremental sync token
- **Target interface**: Implement `target.Target` to add support for a new status platform. Use `nil` status to signal target to clear. Errors are logged and joined; individual target failures do not block other targets.
- **Config layers**: Defaults are baked in (e.g. port 8080, pebble_path ./data), config.yaml overrides defaults, env vars override both. Empty env vars are treated as unset.
- **Status persistence**: Status is stored or deleted in Pebble regardless of individual target failures. If no events are active, the status is cleared.

## Adding a new target

1. Create `internal/{platform}/target.go` with a struct implementing `target.Target`
   ```go
   type Target struct {
       token string
   }

   func (t *Target) Sync(ctx context.Context, status *target.Status) error {
       // If status is nil, clear the user's status on the platform
       // If status is non-nil, set emoji, text, and expiration
       // Return error if the API call fails
   }
   ```

2. Add config fields to `TargetsConfig` in `internal/config/config.go`:
   ```go
   type YourPlatformTargetConfig struct {
       Token string `koanf:"token"`
   }
   
   // In TargetsConfig:
   YourPlatform YourPlatformTargetConfig `koanf:"your_platform"`
   ```

3. Add env var mapping in `internal/config/config.go`'s `envMapping`:
   ```go
   "YOUR_PLATFORM_TOKEN": "targets.your_platform.token",
   ```

4. Instantiate in `buildTargets()` in `main.go` when token is non-empty:
   ```go
   if t := cfg.Targets.YourPlatform.Token; t != "" {
       targets = append(targets, yourplatform.NewTarget(t))
   }
   ```

5. Write tests in `internal/{platform}/target_test.go`

6. Update README.md with platform details and credential creation instructions

## Important implementation notes

- **iCal parsing**: Line folding (continuation lines starting with space/tab) must be unfolded before processing. Check `internal/calendar/ical.go` for the parser implementation.
- **Date/time handling**: RFC 5545 allows DATE (YYYYMMDD), DATE-TIME (YYYYMMDDTHHmmss), and DATE-TIME+Z (UTC) formats. Timezones with TZID parameters are extracted but UTC is assumed for simplicity.
- **Cancelled events**: Events with a CANCELLED status are stored but never set a user status (treated as if not active).
- **Single-tenant**: The application is designed for a single user. To add multi-user support, you would need to parse additional iCal properties and extend the key design to include user identification.

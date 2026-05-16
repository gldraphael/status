# Agent Instructions

This repository is a Go service that syncs calendar status and exposes availability from a separate calendar.

## Project Overview
- Personal app for syncing calendar-driven status to GitHub.
- Status sync fans out to configured targets rather than being tied to GitHub alone.
- Optional availability feature reads a separate calendar, applies weekday `suppressions.working_hours.start/end`, and returns the first free block per day for the next 10 days through its API.
- Both features are polling-based and use Pebble for persistence.
- Pebble stores status and snapshots so lookups stay O(1) for the hot path.
- The config is organized by top-level `status` and `availability` domains, each with nested `sources` and `targets` where applicable.
- The code is organized so status fetch/publish, availability fetch/publish, and HTTP serving are separate concerns.

## Architecture
- `main.go` loads config, opens Pebble, builds the HTTP mux, and starts the sync loops after startup validation.
- `internal/calendar` fetches iCal feeds, parses events, expands recurrences, and extracts feed timezone; `internal/calendar/handler.go` owns the status syncer.
- `internal/availability` fetches and stores the raw availability feed, optionally caches GOV.UK bank holidays, computes free blocks, and serves `/api/availability`.
- `internal/store` owns Pebble persistence for status, events, channels, sync tokens, and the availability and holiday snapshots.
- `internal/target` defines the status target interface; `internal/github` implements the GitHub target.
- `internal/server` wraps `http.Server` with graceful shutdown.
- `internal/config` loads defaults, `config.yaml`, and environment variables.

## Sync Flow
- Status sync is enabled only when `status.enabled` is true; it fetches `status.sources.ical.url`, stores events, computes the current active event, and syncs configured targets on `status.sources.ical.interval`.
- Availability sync is enabled only when `availability.enabled` is true; it fetches `availability.sources.ical.url` on `availability.sources.ical.interval`, stores the raw ICS body plus timezone metadata in Pebble, exposes the API, and starts availability targets.
- If a feature-level `enabled` flag is false, both fetching and publishing for that feature must stay disabled regardless of nested configuration.
- If `availability.suppressions.exclude_england_bank_holidays` is enabled, the app fetches GOV.UK bank holidays once at startup and stores the parsed holiday dates locally.
- That startup seed is required for the availability endpoint when holiday exclusion is enabled, so startup should fail if the holiday feed cannot be read or parsed.
- Weekday availability uses `availability.suppressions.working_hours.start/end` as a suppression window; if `availability.suppressions.exclude_england_bank_holidays` is enabled, that suppression is lifted on England-and-Wales bank holidays from GOV.UK.
- `/api/availability` is only registered when `availability.enabled` is true.
- The availability handler requires an exact `Authorization` header match with `availability.api.key`.

## Build, Test & Lint
- `go build ./...`
- `go test ./...`
- `go vet ./...`
- `go mod tidy`

## Key Conventions
- Keep status and availability calendars separate.
- Status calendar URL and availability calendar URL are separate config values: `status.sources.ical.url` and `availability.sources.ical.url`.
- Status and availability fetch intervals are separate config values and default to `5m`.
- Time blocks come from config and are checked in order.
- `availability.suppressions.working_hours.start` defaults to `09:00` and `availability.suppressions.working_hours.end` defaults to `17:50`; together they are treated as weekday working time, not as an availability block.
- Availability data is stored as the fetched raw ICS body plus metadata, not as live network state.
- When enabled, bank holiday data is fetched from `https://www.gov.uk/bank-holidays.json` at startup and cached in Pebble.
- The availability route is disabled when `availability.enabled` is false.
- Empty env vars are treated as unset.
- Pebble key design includes:
  - `status` for the current status record
  - `event:{eventID}` for stored calendar events
  - `availability` for the latest raw availability snapshot
  - `availability_dirty` for tracking if availability changed since last deploy (stores pending JSON)
  - `availability_last_deployed` for tracking the availability entries JSON from the last successful deploy
  - `availability_holidays` for the cached England bank holiday snapshot
- The availability snapshot stores the raw ICS body, extracted timezone, and fetch timestamp so computation can happen locally without another network read.
- The holiday snapshot stores the raw GOV.UK JSON body, parsed dates, and fetch timestamp so availability can be computed offline.
- Status is single-tenant.
- Time zone handling should use the feed timezone when available, with UTC fallback.
- If `status.enabled` is true, `status.sources.ical.url` and at least one status target are required; otherwise availability can run without a status calendar.
- If `availability.enabled` is true, `availability.sources.ical.url`, availability blocks, `availability.api.key`, and `availability.targets.cloudflare_pages.deploy_hook` are required.

## Adding a New Status Target
- Add a target under `internal/{platform}` implementing `target.Target`.
- Extend `StatusTargetsConfig` and `envMapping` in `internal/config/config.go`.
- Register the target in `buildTargets()` in `main.go`.
- Add tests and update docs.

## Important Implementation Notes
- `internal/calendar/ical.go` is the central parser; keep recurrence expansion and date handling there.
- Avoid reimplementing iCal line folding or date parsing by hand; keep tests around parser behavior instead.
- Cancelled events are stored but do not count as active.
- Availability computation checks today plus the next 9 days and returns the first free configured block per day.
- On weekdays, blocks that overlap working hours are suppressed unless the day is a bank holiday and holiday exclusion is enabled.
- If `availability.enabled` is true but availability config is incomplete, startup should fail fast.
- The app is designed for a single user; multi-user support would require key design changes.

## Cloudflare Pages auto-deploy

- Feature: When `availability.enabled` is true, the application triggers a Cloudflare Pages deployment at a regular interval.
- Change Tracking: To save build minutes, deployments are only triggered if the availability calendar has changed since the last successful deployment. This is managed by the `availability.Syncer`, which compares the current computed availability JSON with the `availability_last_deployed` JSON in Pebble. If a change is detected (including time-based changes), the new JSON is stored in `availability_dirty`, signaling the `Deployer` to trigger a build.
- Config keys: `availability.enabled` (bool), `availability.targets.cloudflare_pages.interval` (Go duration string, e.g., "10m"), `availability.targets.cloudflare_pages.deploy_hook` (Pages Build Hook URL).
- Scheduling: Deploys are scheduled to always fall offset by one minute after the hour. Example: with `availability.targets.cloudflare_pages.interval = 10m` deploys occur at HH:01, HH:11, HH:21, ... This reduces the chance of publishing stale calendar events that commonly start at round minutes (e.g., HH:20, HH:30).
- Security: Do not commit `availability.targets.cloudflare_pages.deploy_hook` into source control; provide it via `config.yaml` or the `AVAILABILITY_TARGETS_CLOUDFLARE_PAGES_DEPLOY_HOOK` environment variable.

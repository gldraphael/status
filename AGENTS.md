# Agent Instructions

This repository is a Go service that syncs calendar status and exposes availability from a separate calendar.

## Project Overview
- Personal app for syncing calendar-driven status to GitHub.
- Status sync fans out to enabled targets rather than being tied to GitHub alone.
- Optional availability API reads a separate calendar, applies weekday `working_hours.start/end`, and returns the first free block per day for the next 10 days.
- Both features are polling-based and use Pebble for persistence.
- Pebble stores status and snapshots so lookups stay O(1) for the hot path.
- The code is organized so status sync, availability sync, and HTTP serving are separate concerns.

## Architecture
- `main.go` loads config, opens Pebble, builds the HTTP mux, and starts the sync loops after startup validation.
- `internal/calendar` fetches iCal feeds, parses events, expands recurrences, and extracts feed timezone; `internal/calendar/handler.go` owns the status syncer.
- `internal/availability` fetches and stores the raw availability feed, optionally caches GOV.UK bank holidays, computes free blocks, and serves `/api/availability`.
- `internal/store` owns Pebble persistence for status, events, channels, sync tokens, and the availability and holiday snapshots.
- `internal/target` defines the status target interface; `internal/github` implements the GitHub target.
- `internal/server` wraps `http.Server` with graceful shutdown.
- `internal/config` loads defaults, `config.yaml`, and environment variables.

## Sync Flow
- Status sync fetches the status calendar, stores events, computes the current active event, and syncs enabled targets every 5 minutes.
- Availability sync fetches the availability calendar and stores the raw ICS body plus timezone metadata in Pebble.
- If `availability.exclude_england_bank_holidays` is enabled, the app fetches GOV.UK bank holidays once at startup and stores the parsed holiday dates locally.
- That startup seed is required for the availability endpoint when holiday exclusion is enabled, so startup should fail if the holiday feed cannot be read or parsed.
- Weekday availability uses `availability.working_hours.start/end` as a suppression window; if `availability.exclude_england_bank_holidays` is enabled, that suppression is lifted on England-and-Wales bank holidays from GOV.UK.
- `/api/availability` is only registered when `availability.is_enabled` is true.
- The availability handler requires an exact `Authorization` header match with `availability.api_key`.

## Build, Test & Lint
- `go build ./...`
- `go test ./...`
- `go vet ./...`
- `go mod tidy`

## Key Conventions
- Keep status and availability calendars separate.
- Status calendar URL and availability calendar URL are separate config values.
- Time blocks come from config and are checked in order.
- `availability.working_hours.start` defaults to `09:00` and `availability.working_hours.end` defaults to `17:50`; together they are treated as weekday working time, not as an availability block.
- Availability data is stored as the fetched raw ICS body plus metadata, not as live network state.
- When enabled, bank holiday data is fetched from `https://www.gov.uk/bank-holidays.json` at startup and cached in Pebble.
- The availability route is disabled when the feature is not configured.
- Empty env vars are treated as unset.
- Pebble key design includes:
  - `status` for the current status record
  - `event:{eventID}` for stored calendar events
  - `channel:{channelID}` for push notification channels
  - `sync:{calendarID}` for incremental sync tokens
  - `availability` for the latest raw availability snapshot
  - `availability_holidays` for the cached England bank holiday snapshot
- The availability snapshot stores the raw ICS body, extracted timezone, and fetch timestamp so computation can happen locally without another network read.
- The holiday snapshot stores the raw GOV.UK JSON body, parsed dates, and fetch timestamp so availability can be computed offline.
- Status is single-tenant.
- Time zone handling should use the feed timezone when available, with UTC fallback.

## Adding a New Status Target
- Add a target under `internal/{platform}` implementing `target.Target`.
- Extend `TargetsConfig` and `envMapping` in `internal/config/config.go`.
- Register the target in `buildTargets()` in `main.go`.
- Add tests and update docs.

## Important Implementation Notes
- `internal/calendar/ical.go` is the central parser; keep recurrence expansion and date handling there.
- Avoid reimplementing iCal line folding or date parsing by hand; keep tests around parser behavior instead.
- Cancelled events are stored but do not count as active.
- Availability computation checks today plus the next 9 days and returns the first free configured block per day.
- On weekdays, blocks that overlap working hours are suppressed unless the day is a bank holiday and holiday exclusion is enabled.
- If the availability config is enabled but incomplete, startup should fail fast.
- The app is designed for a single user; multi-user support would require key design changes.

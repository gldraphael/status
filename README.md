# status

Personal app to sync my calendar status with GitHub and expose availability from a separate calendar.

## Quickstart

```bash
export STATUS_SOURCES_ICAL_URL="https://calendar.google.com/calendar/ical/...%40group.calendar.google.com/public/basic.ics"
export STATUS_TARGETS_GITHUB_TOKEN="ghp_..."

podman compose up
```

Status sync starts only when at least one status target is configured. The
status calendar fetch interval is configured with
`STATUS_SOURCES_ICAL_INTERVAL` / `status.sources.ical.interval`, defaulting to
`5m`.

## Availability

Set `AVAILABILITY_API_IS_ENABLED=true` to expose `/api/availability` from a separate calendar feed.

- `AVAILABILITY_WORKING_HOURS_START` defaults to `09:00` and `AVAILABILITY_WORKING_HOURS_END` defaults to `17:50`; weekday blocks that overlap that window are suppressed unless the day is a bank holiday.
- Set `AVAILABILITY_EXCLUDE_ENGLAND_BANK_HOLIDAYS=true` to lift that weekday suppression on England-and-Wales bank holidays from GOV.UK. Holiday data is fetched at startup and cached in Pebble.
- `AVAILABILITY_SOURCES_ICAL_URL` controls the availability feed, and `AVAILABILITY_API_KEY` controls the exact `Authorization` header required by the endpoint.
- `AVAILABILITY_SOURCES_ICAL_INTERVAL` / `availability.sources.ical.interval` controls the availability calendar fetch interval, defaulting to `5m`.

## Cloudflare Pages

Set `AVAILABILITY_TARGETS_CLOUDFLARE_PAGES_IS_ENABLED=true` to trigger Cloudflare
Pages deploys when computed availability changes. Configure the build hook with
`AVAILABILITY_TARGETS_CLOUDFLARE_PAGES_DEPLOY_HOOK`; the publish interval is
`AVAILABILITY_TARGETS_CLOUDFLARE_PAGES_INTERVAL` /
`availability.targets.cloudflare_pages.interval`, defaulting to `10m`.

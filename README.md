# status

Personal app to sync my calendar status with GitHub and expose availability from a separate calendar.

## Quickstart

```bash
export CALENDAR_URL="https://calendar.google.com/calendar/ical/...%40group.calendar.google.com/public/basic.ics"
export GITHUB_TOKEN="ghp_..."

podman compose up
```

## Availability

Set `AVAILABILITY_IS_ENABLED=true` to expose `/api/availability` from a separate calendar feed.

- `AVAILABILITY_WORKING_HOURS_START` defaults to `09:00` and `AVAILABILITY_WORKING_HOURS_END` defaults to `17:50`; weekday blocks that overlap that window are suppressed unless the day is a bank holiday.
- Set `AVAILABILITY_EXCLUDE_ENGLAND_BANK_HOLIDAYS=true` to lift that weekday suppression on England-and-Wales bank holidays from GOV.UK. Holiday data is fetched at startup and cached in Pebble.
- `AVAILABILITY_CALENDAR_URL` and `AVAILABILITY_API_KEY` still control the availability feed and the exact `Authorization` header required by the endpoint.

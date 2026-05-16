# Pebble schema

The store uses versioned singleton keys. Runtime reads must be direct Pebble
`Get` calls against one known key. Do not add iterator-backed reads or prefix
scans to serve current status, availability, deploy state, or HTTP requests.

Writes can do slower work. Sync and startup paths are responsible for fetching,
parsing, computing, and replacing projections.

## Versioning

All active keys use the `v1:` prefix. Older keys, including previous unversioned
keys and the short-lived `v1:*:source` keys, may remain on disk, but the code
does not read or migrate them. This is intentional because the app does not
require schema backward compatibility.

## Keys

| Key                                | Payload                 | Written by                    | Read by                       | Notes                  |
| ---------------------------------- | ----------------------- | ----------------------------- | ----------------------------- | ---------------------- |
| `v1:status:current`                | JSON `Status`           | Status sync                   | Status callers/tests          | Current active status, or absent when idle. |
| `v1:status:raw`                    | JSON `CalendarSnapshot` | Status sync                   | Status sync                   | Raw ICS body, extracted timezone, and fetch timestamp. Used as fallback when a status fetch fails. |
| `v1:availability:raw`              | JSON `CalendarSnapshot` | Availability sync             | Startup and availability sync | Raw ICS body, extracted timezone, and fetch timestamp. |
| `v1:availability:current`          | JSON API response bytes | Startup and availability sync | `/api/availability`           | Precomputed response served directly by the handler. |
| `v1:availability:holidays:england` | JSON `HolidaySnapshot`  | Startup holiday seed          | Availability recomputation    | Required when England bank holiday suppression is enabled. |
| `v1:availability:deploy:dirty`     | JSON API response bytes | Availability sync             | Cloudflare Pages deployer     | Pending availability response that has not been successfully deployed. |
| `v1:availability:deploy:last`      | JSON API response bytes | Cloudflare Pages deployer     | Availability sync             | Last availability response successfully deployed. |

## Atomic updates

Use Pebble batches when multiple keys represent one logical state transition:

- Status sync replaces `v1:status:raw` and `v1:status:current` together after a successful fetch. On fetch failure, it may recompute and replace only `v1:status:current` from cached raw data.
- Availability sync replaces `v1:availability:raw` and
  `v1:availability:current` together after a successful fetch.
- Deploy success writes `v1:availability:deploy:last` and deletes
  `v1:availability:deploy:dirty` together.

## Startup behavior

Startup must not wipe the configured Pebble directory. Availability startup
rehydrates `v1:availability:current` from `v1:availability:raw` when a raw
snapshot exists. If no raw snapshot exists, the current availability response
is cleared and the API returns `503` until the first successful sync.

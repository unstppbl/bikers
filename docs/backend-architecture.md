# Backend architecture

The approved architecture decision is the [backend package layout in AGENTS.md](../AGENTS.md#backend-package-layout). This document maps that decision to the current application. Keep the package map, dependency checker, and architecture decision aligned when responsibilities change.

## Package ownership

All paths below are relative to `apps/backend/internal`.

| Package | Owns |
| --- | --- |
| `models` | Shared domain records, serialization tags, error vocabularies, value validation, pure domain-value functions, international phone normalization, and the curated bike-model/alias and city vocabularies. |
| `config` | Loading and validating application settings. |
| `database` | Repository interfaces and callback types used to coordinate transactions. |
| `database/postgres` | SQL, schema migrations, stored-record mapping, locks, transaction boundaries, and persistence concurrency guards. |
| `managers/accounts` | OTP challenges, account and session rules, profiles, authentication limits, the external Gateway interface, and strict provider request/phone/status binding decisions. |
| `managers/rides` | Ride plans, public and participant visibility, organizer decisions, participation, capacity, plan versions, discovery/personal-list selection, filter validation and signed pagination cursors. |
| `managers/geo` | Kazakhstan map envelope, language/point/query validation, per-user request budgets, bounded geocoder/routing concurrency, normalized map plans, and the external Geocoder/Router interfaces. |
| `managers/places` | Public useful-place discovery, moderated new/correction/closure suggestions, contribution quotas, publication/version decisions, and pinned local OSM import validation/preservation. |
| `managers/routes` | Community route discovery/publication, private GPS recordings and collections, consent, geometry validation, dated condition reports and creation quotas. |
| `managers/rooms` | Room access, message decisions, idempotency, history, and foreground event behavior. |
| `managers/garage` | Bike ownership/public projections and public catalogue search, model lookup, private maintenance, bike-photo visibility, quotas, reservations, cover selection, optimistic versions, and cursor pagination. |
| `managers/clubs` | Club previews/roster permissions, membership decisions, ownership transfer/closure, public ride-association rules, and club-image access, reservations, and replacement. |
| `managers/photos` | Shared image decoding/orientation/re-encoding, metadata stripping, budget defaults, and the two-slot processing module used by garage, clubs and media. |
| `managers/media` | Self/context avatar permissions and replacement, participant-only ride albums, external video links and upload finalization decisions. |
| `managers/trust` | Reporting/evidence access, blocks, administrator roles, moderated visibility, account restrictions and audited community badges. |
| `managers/notifications` | Independent preferences, unread counts, durable inbox/read state, push-category consent/subscription limits and retry/eligibility decisions. |
| `managers/reminders` | Per-ride consent, participation-generation binding, due-window checks and current reminder eligibility. |
| `managers/system` | Explicit migration execution and timeouts, plus schema and media readiness checks. |
| `clients/telegram` | Telegram Gateway protocol requests and response decoding, including transport settings and defaults. |
| `clients/photon` | Bounded requests to the fixed private Photon origin, country-filtered place decoding, and normalized address labels. |
| `clients/valhalla` | Fixed motorcycle routing requests to the private Valhalla origin and bounded OSRM GeoJSON decoding. |
| `clients/webpush` | Web Push encryption/VAPID through webpush-go, provider validation and bounded HTTPS transport with public-IP dialing. |
| `workers` | Push and reminder polling cadence/shutdown; decisions run through their managers. |
| `resources/http` | Request decoding, API routing, response mapping, cookies, CSRF, HTTP middleware, static assets, and WebSocket transport. Its Go package name is `httpapi` to distinguish it from `net/http`. |
| `app` | Constructing and injecting dependencies, startup, shutdown, and composed integration tests. |

`workers` contains the actual push and reminder polling loops; it calls an injected dispatcher interface. Keep feature and service packages one level below `managers` and `clients`; helpers stay with their owner. Commands remain under `cmd/<tool>`: `api` owns server/migration execution, audited administrator bootstrap and private VAPID key-file generation; `architecture` checks source boundaries.

## Dependency direction

| Owner | Allowed internal dependencies |
| --- | --- |
| `models`, `config` | None |
| `database` | `models` |
| `database/<implementation>` | `database`, `models` |
| `managers/<feature>` | `models`, `database`, other managers without cycles |
| `clients/<service>` | `models` |
| `resources/<transport>` | Managers, `models` |
| `workers` | Managers, `models`, and actual queue clients needed for polling |
| `app` | All layers |

Managers declare the interfaces for external services. `app` injects client implementations and passes validated configuration to constructors. A manager does not import a concrete Telegram client or PostgreSQL repository. Transport calls managers and maps their results to protocol responses.

## Transactions and callbacks

Managers own application decisions and coordinate operations through the contracts in `database`. PostgreSQL repositories own transaction boundaries, SQL, lock acquisition, and lock ordering. When a repository operation is atomic, it passes locked state to a manager-supplied callback and persists the resulting decision within the same transaction.

Garage operations lock the owning account before the bike, then revalidate ownership, active account state, and versions in manager callbacks. Club operations lock the club before account/ride rows; callbacks decide membership transitions, ownership, and whether a published ride may be linked. Linked rides remain public summaries and do not alter ride participation or permissions. Managers supply clocks and result limits to repositories, including upcoming-first club ride selection.

Aggregate transactions acquire the common exclusive trust advisory lock first, serializing blocks/moderation with membership changes, reads and upload finalization. Image operations acquire the shared photo advisory key next, followed by their entity/account locks. Bike operations then lock owner → bike; club-image operations lock club → actor account. This consistent order avoids feature-to-trust lock inversions. Managers revalidate active accounts under the transaction; profile patches also apply to fresh locked state.

All four image tables use PostgreSQL `bytea`, with metadata, bytes, references and deletion in the same database. Their manager callbacks reserve shared count/byte budgets and expire pending reservations across all four tables. A club image's original uploader remains its quota attribution after transfer; that field grants no permissions. `app` injects one `managers/photos` instance into garage, clubs and media, giving the application two shared processing slots. A slot is acquired before reservation/body reading and retained until synchronous processing exits outside database locks. Finalization rechecks account, ownership/participation, entity versions and expiry. Ride photos capture participation and plan versions, rejecting remove/reapprove and plan-change races. Club images capture club/image versions, rejecting transfer away/back and closure. Replacement/removal atomically changes the current reference and deletes old content.

Failed-upload cleanup uses an independent three-second context and deletes only the caller's pending reservation; it must retain ready content after an uncertain successful commit. Later reservations lazily expire pending rows across all image tables. There is no filesystem photo adapter, GC queue, or cleanup worker; originals are discarded and `MEDIA_DIR` is reserved for future media. PostgreSQL backups include all processed images and their references.

Ride announcements, significant plan changes, cancellation, ride participation and club membership transitions write durable notification intent in the same transaction. Club requests notify the owner at commit time; approval/rejection/removal notify the affected rider. Managers choose recipients and kinds only for real transitions; PostgreSQL persists the membership, inbox row and push jobs atomically. Notification targets contain exactly one ride or club ID, with kind/target constraints in the schema. Push claims lease jobs and recheck current account/session/membership before committing; the manager makes encrypted network calls after releasing locks, then finishes only the matching lease. Report evidence stores bounded text/metadata snapshots and references original image bytes without duplicating quota-limited content. Moderator hiding preserves stored evidence while ordinary content endpoints return `404`.

Run external network, model, and embedding calls before acquiring database locks. Under the locks, revalidate the relevant state and assumptions before applying the result. Callbacks executed under locks contain local decisions and persistence work only. Keep limits and authorization decisions in managers; move shared value validation into `models` and leave persistence mechanics in the repository.

Moving a type preserves serialization tags and field order. Moving behavior updates every caller and removes the old location; forwarding aliases are not part of the architecture.

The explicit migration runner applies versions 1–12 in order, verifies stored checksums, and rejects unknown versions. Version 3 adds bike photos, version 4 club images, version 5 avatars/ride media, version 6 trust/reporting/admin state, version 7 preferences/announcements/notifications/push, version 8 club notification targets, version 9 push categories/reminder consent/occurrence receipts, version 10 the nullable private ride map plan, version 11 public useful places and private moderated submissions, and version 12 community routes, private recordings/collections and dated conditions. Applied versions 1–11 remain unchanged. Readiness requires all twelve receipts. Schema mechanics remain in `database/postgres`; `managers/system` owns execution timeouts and readiness decisions.

## Community routes and GPS recordings

`managers/routes` owns visibility, publication consent, optimistic versions, private collection membership, recording bounds and condition freshness. `database.RoutesRepository` exposes local decision callbacks. PostgreSQL acquires the common trust lock before the routes lock, so route mutations, reporting visibility, blocks, role changes and account restrictions serialize. Public and private reads run in this same bounded transaction; no provider calls execute under locks.

Planned routes use the existing geo manager to calculate geometry before the write transaction. The routes manager checks the owner and version before calculation and rechecks both under the final locks. A recorded route copies an owned private track into a separate snapshot. Publishing and changing published geometry require explicit consent. Private ride plans are never read or republished by this flow. Public lists use summaries without geometry or private source IDs; full route geometry is available only for deliberately published routes or to the owner. Hidden content, restricted/hidden authors and bilateral blocks are rechecked for each viewer.

GPS tracks contain named, bounded longitude/latitude segments and elapsed duration. Distances are computed on the server within each segment, so pauses and lost-signal gaps do not add invented connecting distances. The foreground recorder keeps an account-bound draft until a server save succeeds; switching accounts or signing out stops location capture and removes the previous actor's local draft. Tracks and named collections have owner-only access. Deleting a private source recording leaves a published snapshot intact; deleted/unpublished/hidden routes are filtered from other riders' saved collections.

Road-condition reports retain attribution, observation and update dates. Freshness expires thirty days after observation; resolved reports have a separate state. Only the report author may edit/delete it, while administrators use the existing audited hide/restore controls. A separate per-account/date count survives report deletion to enforce the daily creation limit without retaining deleted report content. These reports provide dated community observations rather than a route-safety assessment.

## Useful places

`GET /api/v1/places` and `/places/{id}` expose published records only, with explicit public place fields and OSM/community provenance. `/places/config` exposes the same-origin map archive path without granting private geocoder or ride-plan access. Filters are a literal name/city/address/description substring (`q`, at most 160 bytes), one category, a city alias, and an optional ordered bounding box (`west,south,east,north`, all four required within the Kazakhstan map envelope). Pagination is ascending UUID, 50 rows by default and at most 100; `limit` is bounded and `after` is an opaque cursor bound to the filters and page size. Cursors survive process restarts. A changed filter/cursor returns `invalid_cursor`; restart from the first page. These places are public business/meeting locations, unlike private ride map points.

Authenticated contributors submit a new place, a complete correction, or a closure report through `POST /api/v1/places/submissions`. The manager checks current active account state, onboarding, the UTC daily limit of 20 submissions, and the place version under repository locks. Nothing publishes automatically. Own submission history is private and excludes actor/reviewer identifiers; the administrator queue includes full proposals and review evidence. Review accepts only the current pending submission version. Approval rechecks active administrator and contributor accounts plus the current published place version, then atomically updates the place and persists reviewer/reason/time in the submission. Rejection never changes the place. Competing or stale approvals return `stale_version` and leave the unsuccessful proposal pending. Contributors submit a fresh correction after reviewing changed details; an administrator can reject the obsolete proposal with a reason.

Place transactions acquire the common trust advisory lock, then the places advisory lock, then account and record locks. This serializes publication with account restrictions and administrator role changes. Imported data does not acquire account locks and uses only the places lock. Public reads require no authenticated state and never join private proposals or accounts.

`api places import --file PATH --sha256 HASH` calls the places manager with a local pinned NDJSON artifact. Every nonblank line contains `source_ref` (`node/123`, `way/123` or `relation/123`), `source_date` (`YYYY-MM-DD`), and editable place fields: name, category, latitude/longitude (`lat`,`lon`), city, address, description, website, phone and opening_hours. Categories are fuel, workshop, tires, motorcycle_shop, parking and meeting_spot. Imports are bounded to 32 MiB/50,000 records, reject duplicate OSM references, unknown fields, invalid points/URLs and future dates, and validate the complete artifact/checksum before starting a transaction. The repository batches atomic writes; the manager skips community-modified/closed records and older source dates, preserving approved edits and stable UUIDs. Replaying an identical artifact creates no rows or version changes. Missing OSM objects are not implicitly deleted or closed. There is no remote fetch inside the command, manager or transaction.

## Maps and private ride plans

Authenticated clients obtain same-origin archive configuration from `GET /api/v1/geo/config`, search addresses with `POST /api/v1/geo/search` and JSON `{query,lang,near?:{lat,lon}}`, reverse-geocode points with `POST /api/v1/geo/reverse` and JSON `{point:{lat,lon},lang}`, and calculate motorcycle routes with `POST /api/v1/geo/route`. All three POST endpoints require the existing CSRF token and strict bounded JSON decoding. Search text, coordinates and route points travel in request bodies so normal edge access logs do not record them in URL query strings; the old search/reverse GET endpoints are removed. Error responses use fixed messages and do not echo private input. These use the existing private API `no-store` policy. Photon remains a private internal GET API without an enabled request logger; Valhalla receives only POST route bodies. The operator configures Photon and Valhalla together using fixed `PHOTON_BASE_URL` and `VALHALLA_BASE_URL` origins, plus a versioned `MAPS_ARCHIVE_URL` under `/map-data/`. No request supplies a service URL. Empty settings keep text-only rides available; map configuration is enabled only when both providers and the archive path are configured. GIS health does not change core application readiness.

`models.MapPoint` stores a confirmed label and latitude/longitude. `models.MapPlan` stores one to ten points, optional GeoJSON LineString geometry in longitude/latitude order, distance in meters and estimated duration in seconds. A single point is a meeting pin. Two to ten points request a road-motorcycle route; clients cannot submit derived geometry. Photon searches request Kazakhstan-only results and normalize multilingual labels. The [Valhalla routing API](https://valhalla.github.io/valhalla/api/route/api-reference/#directions-options) requires `format=osrm` for `shape_format=geojson`; the adapter fixes these options, `costing=motorcycle`, `use_trails=0`, `use_tracks=0`, no alternatives and no directions. It omits the narration language because the plan has no instructions and Valhalla does not provide Kazakh narration. This slice provides route planning rather than turn-by-turn navigation.

The geo manager bounds coordinates to a regional envelope (40.5–55.5 latitude, 46–87.5 longitude), limits a user to 60 search/reverse and 12 routing attempts per minute, and permits four geocoder calls and one route calculation concurrently per application process. Providers have four/eight-second HTTP deadlines, redirects disabled, and response limits of 256 KiB/eight MiB respectively. Normalized routes contain at most 50,000 geometry coordinates. The envelope bounds workload and does not verify political borders or countrywide address coverage.

Ride create/update accepts only optional `map_points`. Omission on update preserves the saved plan; an empty array clears it. The rides manager prepares a requested route before acquiring persistence locks, checks the organizer and plan version before planning, and rechecks active account, organizer, cancellation and version under the write lock before committing. Selected first/last labels synchronize meeting/destination text; manual text-location changes without map points clear a stale saved map. A coordinate-only change is a significant plan update and writes participant notification intent atomically. Unrelated edits retain the saved route without recomputation.

The nullable bounded `rides.map_plan` JSONB column holds the private snapshot. Existing rides retain NULL and their text fields. `RideSummary` remains an explicit public projection; anonymous viewers and pending members never receive points or route geometry. The full map follows the existing organizer/approved-participant access rule, including moderation behavior. Selected POIs require no separate table, PostGIS extension, or provider lookup at display time.

The photo storage choice and numeric limits are implementation defaults for this bounded slice. Filesystem or object storage remains a future proposal in the draft design, not a confirmed product decision or migration schedule.

## Automated check and tests

From the repository root:

```sh
make check-architecture
```

Or, from `apps/backend`:

```sh
go run ./cmd/architecture
go test ./cmd/architecture
```

The checker parses every Go file under `internal`, including tests, generated files, and files excluded by the current build tags. It rejects unknown package paths, nested feature/service packages, and imports outside the dependency table. The queue-client set is currently empty, so workers cannot import the Telegram client. A real queue-client dependency must be recorded with its polling responsibility when implemented.

Tests stay beside their owner. An external `<owner>_test` package can import the package it tests; all other internal dependencies follow the same table. Tests do not have a general exemption. Composed HTTP/PostgreSQL tests live beside `app`; manager tests use package-local contract fakes. There is no shared test-helper package.

The command has positive and negative fixture tests and a test of the actual source tree. As a result, ordinary `go test ./...`, `make check-backend`, and `make check-integration` enforce the internal import rules. Go's compiler and test tools still check import cycles and build correctness.

Import checks do not prove semantic ownership, pure callbacks, absence of external calls under locks, or correct transaction boundaries. Review those properties with the behavior and persistence tests. The checker adds no third-party import bans beyond the approved internal dependency rules.

Reminder scheduling uses the existing aggregate lock order: common trust
advisory lock, then ride and account rows. The repository reads current context
and supplies it to the reminders manager callback, which decides whether to
write an intent. A processed-occurrence receipt commits with any inbox/outbox
rows, preventing duplicates across polls and restarts. Push claim callbacks
share the reminder eligibility rules and recheck per-category consent. The
calendar transport uses an iCalendar library for escaping and CRLF serialization;
the rides manager owns authorization to the private plan snapshot.

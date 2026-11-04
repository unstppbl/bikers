# BIKERS.KZ backend

The backend implements Telegram OTP and cookie sessions, profiles, planned rides
and private Ride Rooms, paginated discovery/My rides, garages and private
maintenance, clubs, processed media, moderation, and durable inbox/opt-in push.
Club membership requests notify the current owner, and approval/rejection/removal
notify the affected rider. These transitions and their inbox/push work commit
atomically. Push payloads remain generic and open `/inbox`.

This executable requires schema 9; its source implementation is
separate from the currently deployed release. Migration is explicit and must be
coordinated with the matching app. See [Operations](../../docs/operations.md).
Photo bytes remain in bounded PostgreSQL `bytea` storage; the persistent media
directory is reserved for future media and checked for writability.

The Go process serves the built React application and `/api/v1` through one HTTP
port. PostgreSQL is private to the Compose network. Run database migrations
explicitly before starting the application; `serve` never changes the schema.

## Package map

The [approved architecture in AGENTS.md](../../AGENTS.md#backend-package-layout)
defines the package boundaries. The [backend architecture guide](../../docs/backend-architecture.md)
records ownership, dependency direction, transaction callbacks, and test placement.

| Internal package | Responsibility |
| --- | --- |
| `models` | Domain records, serialization, vocabularies, value validation, and international phone normalization |
| `config` | Application settings |
| `database` | Repository interfaces and transaction callback types |
| `database/postgres` | Queries, mappings, migrations, transactions, and locks |
| `managers/accounts` | OTP, sessions, profiles, authentication limits, and provider request/phone/status binding |
| `managers/rides` | Plans, visibility, participation, organizer decisions and private calendar access |
| `managers/rooms` | Room access, messages, history, and foreground events |
| `managers/garage` | Bike ownership/sharing, curated lookup, private maintenance, bike-photo access/reservations/covers, validation, and pagination |
| `managers/clubs` | Club visibility, membership and atomic notification intent, ownership transfer/closure, safe ride associations, and club images |
| `managers/notifications` | Preferences, unread counts, durable inbox/read state, push category consent, current eligibility and retry decisions |
| `managers/reminders` | Per-ride reminder consent, current participation eligibility and bounded departure reminder decisions |
| `managers/media` | Profile avatars and participant-scoped ride albums/video links |
| `managers/trust` | Reports, blocks, moderation, administrator actions and badges |
| `managers/photos` | Shared image decoding/orientation/re-encoding, metadata stripping, budget defaults, and the two-slot processing module |
| `managers/system` | Explicit migration execution/timeouts and schema/media readiness |
| `clients/telegram` | Telegram Gateway protocol transport, decoding, and defaults |
| `clients/webpush` | Encrypted provider transport and endpoint/network restrictions |
| `workers` | Push and reminder polling and graceful shutdown |
| `resources/http` | HTTP/WebSocket decoding, routing, middleware, and response mapping |
| `app` | Dependency construction, startup/shutdown, and composed integration tests |

Push polling invokes the notifications manager; network delivery happens after
database claims commit. Helpers and tests remain with their owner. Run `go run ./cmd/architecture` here, or `make check-architecture` from
the repository root. Ordinary `go test ./...` also checks the actual package tree,
including test imports; tests have no general dependency exemption.

## Commands and configuration

From this directory:

```sh
go run ./cmd/api migrate
go run ./cmd/api serve
go run ./cmd/api healthcheck
```

Set `DATABASE_URL`, `BASE_URL`, and `TELEGRAM_GATEWAY_TOKEN` through the process
environment. The token is required for `serve`; it is not required for
`migrate` or `healthcheck`. There is no production fake provider or bypass.

`PORT` defaults to `8080`, `WEB_DIST_DIR` to `../web/dist`, and `MEDIA_DIR` to
`./data/media`. `BASE_URL` is the exact public origin used for Host/Origin checks
and cookie security. HTTPS uses a Secure HttpOnly host-only session cookie.
An explicit loopback HTTP origin enables local development. When using Vite's
proxy, set `BASE_URL=http://localhost:5173` and preserve the incoming Host header.

`TRUSTED_PROXY_CIDRS` optionally lists comma-separated narrow prefixes for actual
proxy peers. Forwarded client addresses are used for OTP source quotas only when
the socket peer is trusted; the chain is checked from right to left. Leave it
empty for direct local connections. Forwarded Host/Proto headers cannot override
the configured public origin. `OTP_DAILY_BUDGET` defaults to 1000 reserved sends.

Initial OTP limits are a five-minute challenge, a sixty-second phone resend
cooldown, three sends per phone and thirty per source in fifteen minutes, and
five verification attempts. Billable sends are never automatically retried after
an uncertain response. The provider status and returned phone/request ID must
match the server-owned challenge; consumption and session creation are atomic.
Provider response phones are compared after international canonicalization,
including Telegram's digit-only representation. National-format guesses,
malformed phones, and different canonical numbers remain rejected.

`GET /health/live` checks that HTTP is running. `GET /health/ready` checks the
exact migration checksums and writable media storage. The `healthcheck` command
uses the loopback listener, independently of the public Host configuration.

Migrations are an ordered, explicit operation: immutable version 1 establishes
accounts/rides/chat; version 2 adds bikes, maintenance, clubs, memberships, and
ride links; version 3 adds bike-photo content, gallery state, and upload attempts;
version 4 adds club images and the current-image reference; versions 5–7 add
avatars/ride media, trust/admin state and notifications/preferences/push; version
8 adds club notification targets; version 9 adds category push preferences and ride reminder state. Versions 1–8 remain unchanged. Applied receipts
are verified before proceeding; unknown versions or changed checksums fail.
Readiness requires all nine receipts.
Run `migrate` with the new executable before serving an older installation.

## API behavior

Errors use `{ "error": { "code": "...", "message": "..." } }`. API responses
are `no-store`. Authenticated mutations require `X-CSRF-Token`; session lookup
returns the current token and `{ "user": null }` when signed out. Cookies and
CSRF values belong to the PWA transport, while domain authorization remains on
the backend for future clients.

Public ride responses contain only ID, city, local date, short description, and
status. Signed-in users can additionally see their own participation state and
management flag. Exact plans, rosters, history, and event subscriptions require
approved participation; organizers are inserted as approved participants.
Ride discovery returns upcoming published summaries in pages of 20. `view` selects
`discover` (default), `organizing`, `joined`, `pending`, or `past`; personal views
require a session. `city`, inclusive ride-local `from`/`to` calendar dates and
literal public-description `q` are applied before paging. `next_cursor` is empty
at the end. The signed cursor binds the caller, filters and clock snapshot, and
contains only the public local date/UUID ordering key, never exact departure
time. Its signing key is process-local; restart invalidates cursors and callers
can restart their view. Organizing/Joined/Pending contain upcoming published
rides, with organizers excluded from Joined/Pending. Past contains earlier or
cancelled rides owned by the caller or with their approved/requested
participation; it does not establish attendance. Past pages sort date/UUID
descending; other pages sort ascending.

`GET /api/v1/cities` returns the suggested Kazakhstan city vocabulary with
English/Russian/Kazakh names and aliases. Profile, ride and club writes normalize
known aliases to one value; custom cities remain valid. City filters match known
aliases in existing stored rows without rewriting them. No migration is needed.

The initial capacity rule includes the organizer. Organizer approval is required;
pending requests do not consume places. Capacity edits, approvals, cancellation,
and messages coordinate on the ride row. Plan edits/cancellation use
`plan_version`; stale writes return `409`. Cancelled rides retain authorized
history and have read-only rooms.

Message writes use a caller-generated UUID `client_message_id`. Retrying the
same ID/text returns the stored message; changed text with that ID returns `409`.
History is ordered by a committed per-room sequence. `?after=N` returns up to 100
messages and `next_sequence`; fetch again until the batch is empty.

`GET /api/v1/events?ride_id=...` upgrades to WebSocket and sends foreground hints
(`message.created`, `ride.updated`, `participation.changed`). Clients refetch the
authoritative HTTP state after hints and reconnects. Hints are not durable push
notifications. Session and participation validity are checked before delivery
and periodically; logout, removal, suspension, and expiry close access.

## Garages and clubs

Bikes are private by default. Owners can explicitly share the basic projection
`id`, `make`, `model`, `model_id`, and `year`; other callers receive `404` for a
private bike. Maintenance stays owner-only even for public bikes. Entries store
a calendar date, integer mileage in kilometres, and work performed. Bike/log
edits and deletion check ownership and `version` under owner-then-bike locks;
stale versions return `409`. Lists page at 50 records. Lookup uses a small
curated Latin/Cyrillic alias vocabulary, with freeform makes/models available. Model
metadata includes public reference-image paths, verified year ranges, dimensions,
and source/author/license credits. Unsupported years have no image; bike DTOs
and maintenance permissions stay unchanged. See the [coverage and sources](../../docs/model-reference-images.md).

Owners can upload JPEG, PNG, or WebP bike photos as raw binary bodies, choose a
cover explicitly, restore the catalogue, and delete photos. Gallery and content
endpoints inherit bike visibility; maintenance stays private, and the existing
Bike/PublicBike fields are unchanged. A fresh oriented JPEG replaces each source,
stripping EXIF/GPS. Content, metadata, cover changes, and deletion commit in
PostgreSQL; there are no filesystem photo blobs, cleanup workers, or GC queues.
Database backups now include uploaded photos. See the [photo API, processing,
quota defaults, and backup boundary](../../docs/development.md#shared-bike-and-club-image-processing).

Club previews expose name, city, description, status, and the caller's own
membership/management flags. Owners approve membership requests; rosters are
private to owners/approved members. Ownership can transfer only to another
active approved member, retaining the previous owner as approved. An active
owner must transfer or close before leaving. Closed clubs are read-only except
for leaving; a departed historical owner gets only the public preview.

Club-image metadata and current JPEG content are public for active and closed
clubs. Only the current active owner with an active account can upload, replace,
or remove an image, and closed clubs reject those writes. Successful replacement
or removal is atomic and makes the old URL return `404`; failed replacement keeps
the old image. Transfer preserves a ready image, but transfer (including away and
back) or closure during upload rejects finalization. Club DTOs and roster
permissions stay unchanged. Bike and club images share processing slots, upload
attempts, and payload quotas; stored club bytes remain attributed to the original
uploader after transfer. See the [club-image API](../../docs/development.md#club-images).

Club owners can link their own published rides and unlink any association in
their club. Linked results contain public ride summaries only, up to 100 with
upcoming published rides first. A link, club membership, or club ownership
never grants private ride details, roster, chat, photos, or participation.

## Club membership inbox and push

Club notices use `club.membership.requested`, `.approved`, `.rejected` and
`.removed`. They contain `club_id`; ride/participation notices contain `ride_id`.
The schema permits exactly one target and checks that it matches the kind.
Creation is part of the same locked transaction as the actual state change, so
unchanged retries do not duplicate notices and insert failure rolls back the
membership. Existing requests are not backfilled.

Push claims load current club status, owner and recipient membership.
The notifications manager requires active-club ownership for request alerts and
active-club approved membership/ownership for approval alerts. Rejection/removal
outcomes may reach former members. Account/session eligibility still applies;
provider calls stay outside database locks. Lock-screen text and `/inbox` remain
generic. The inbox works without push configuration or browser permission.

## Verification

The integration tests use real PostgreSQL with a separate random schema per
test. A fake Gateway is injected only from `_test.go`; tests never contact
Telegram or deliver codes.

```sh
TEST_DATABASE_URL='postgres://test-user:test-password@127.0.0.1:55432/test-db?sslmode=disable' \
  go test -race ./...
go vet ./...
```

Without `TEST_DATABASE_URL`, PostgreSQL tests explicitly skip. Set it for the
required journey/security checks. Coverage includes invalid/missing provider
states, phone binding, expiry, replay, concurrent consumption, quotas across UTC
midnight, pending/removed-user privacy, concurrent approval capacity, message
idempotency and ordering, plan versions, WebSocket revocation, and public HTML.

Garage/club regressions cover public bike projections, private log ownership and
cross-bike IDs, cursor pages, model aliases, club approval/transfer/closure,
concurrent transfer versus leaving, and public-only ride associations. Migration
tests cover historical upgrades through version 8, replay, receipt drift, and unsupported versions.
Photo regressions cover visibility/content authorization, cover reset, deletion
cascades, processing limits/metadata stripping, competing global reservations,
uploader quotas, lazy expiry, and failed-attempt limits across application restart.
Club-image checks also cover public closed-club reads, atomic replacement/removal,
transfer and closure during uploads, and combined bike/club slots, quotas, and
attempts.

An application-replacement regression also closes the HTTP server and database
pool, constructs fresh instances, and verifies persisted sessions, outstanding
OTP challenges, approvals, history, retries, and room sequence continuity.

For local browser verification only, build the frontend and run:

```sh
BROWSER_HARNESS=1 TEST_DATABASE_URL='postgres://test-user:test-password@127.0.0.1:55432/test-db?sslmode=disable' \
  go test ./internal/app -run '^TestBrowserHarness$' -v -timeout 20m
```

The test binary listens on `127.0.0.1:8081` for ten minutes, uses an isolated test
schema, and accepts OTP `123456` for valid test phone numbers. It is excluded from
the production binary. Optional `BROWSER_HARNESS_PORT`,
`BROWSER_HARNESS_BASE_URL`, `BROWSER_HARNESS_SECONDS`, and `WEB_DIST_DIR` configure
that test process. Do not expose the test listener publicly.

Use `BROWSER_HARNESS_CLUB_NOTICES=1` with the opt-in harness to seed a disposable
owner (`+77011239902`), applicant (`+77011239903`) and club membership inbox
history. The fake code is `123456`; the latest request stays pending for manual
approval. This fixture is test-only and does not send real OTPs or push.

## Club discovery, reminders and calendar exports

Club listing accepts `view=discover|organizing|joined|pending`, `city`, literal
name substring `q` and `cursor`; it returns pages of 20 with unchanged public
summary fields. Filters apply before pagination. Personal views require a
session and exclude owners from Joined/Pending. Organizing/Joined retain
authorized closed clubs; Discover/Pending include active clubs. Signed cursors
bind the actor, view and filters and expire when the process restarts.

`GET /me/notifications/unread-count` returns only `{unread_count}`. Preferences
add `push_categories` booleans for `ride_updates`, `participation`,
`club_membership` and `ride_reminders`; nested patches can change one category.
All default to true to retain existing push behavior. Category consent is
rechecked while claiming queued push jobs; the durable inbox is unaffected.

`GET/PUT /rides/{id}/reminders` use `{day_before,hour_before}` and require current
approved participation or organizer access. Both default false. Consent binds
to the participation generation, so removal/reapproval requires renewed consent.
Each lead retains its own consent timestamp when the other choice changes.
The reminder worker runs every 30 seconds and processes bounded batches, with
a ten-minute grace window. Occurrence receipts and inbox/push writes commit
atomically. Current departure, participation, consent, visibility, account and
block state are revalidated; outdated jobs cannot send after cancellation,
rescheduling, access loss or opt-out. No reminder is backfilled for a rider who
opts in after its due time.

`GET /rides/{id}/calendar` returns a private, `no-store` iCalendar attachment to
the organizer/current approved participants. It includes the real UTC departure
instant and private meeting point, without inventing an end time or including
roster/account identifiers. The ride UUID and plan version identify refreshes;
cancelled rides export `STATUS:CANCELLED`. The download is a snapshot, not a
public feed or automatically synchronized calendar subscription.

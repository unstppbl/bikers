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
| `managers/rooms` | Room access, message decisions, idempotency, history, and foreground event behavior. |
| `managers/garage` | Bike ownership/public projections, model lookup, private maintenance, bike-photo visibility, quotas, reservations, cover selection, optimistic versions, and cursor pagination. |
| `managers/clubs` | Club previews/roster permissions, membership decisions, ownership transfer/closure, public ride-association rules, and club-image access, reservations, and replacement. |
| `managers/photos` | Shared image decoding/orientation/re-encoding, metadata stripping, budget defaults, and the two-slot processing module used by garage, clubs and media. |
| `managers/media` | Self/context avatar permissions and replacement, participant-only ride albums, external video links and upload finalization decisions. |
| `managers/trust` | Reporting/evidence access, blocks, administrator roles, moderated visibility, account restrictions and audited community badges. |
| `managers/notifications` | Independent preferences, durable inbox/read state, push consent/subscription limits and retry/eligibility decisions. |
| `managers/system` | Explicit migration execution and timeouts, plus schema and media readiness checks. |
| `clients/telegram` | Telegram Gateway protocol requests and response decoding, including transport settings and defaults. |
| `clients/webpush` | Web Push encryption/VAPID through webpush-go, provider validation and bounded HTTPS transport with public-IP dialing. |
| `workers` | Push polling cadence and shutdown; delivery behavior runs through the notifications manager. |
| `resources/http` | Request decoding, API routing, response mapping, cookies, CSRF, HTTP middleware, static assets, and WebSocket transport. Its Go package name is `httpapi` to distinguish it from `net/http`. |
| `app` | Constructing and injecting dependencies, startup, shutdown, and composed integration tests. |

`workers` contains the actual push polling loop; it calls an injected dispatcher interface. Keep feature and service packages one level below `managers` and `clients`; helpers stay with their owner. Commands remain under `cmd/<tool>`: `api` owns server/migration execution, audited administrator bootstrap and private VAPID key-file generation; `architecture` checks source boundaries.

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

The explicit migration runner applies versions 1–8 in order, verifies stored checksums, and rejects unknown versions. Version 3 adds bike photos, version 4 club images, version 5 avatars/ride media, version 6 trust/reporting/admin state, version 7 preferences/announcements/notifications/push, and version 8 club notification targets. Applied versions 1–7 remain unchanged. Readiness requires all eight receipts. Schema mechanics remain in `database/postgres`; `managers/system` owns execution timeouts and readiness decisions.

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

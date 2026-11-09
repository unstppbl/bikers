# Local development and API

The internal implementation covers all agreed [MVP feature slices](mvp-and-roadmap.md), including avatars, ride albums/videos, organizer announcements, the durable inbox and optional Web Push, reporting/blocking, administrator review/manual badges, and independent locale preferences. [Operations](operations.md) covers deployment configuration, role bootstrap, push keys and restore/rollback. Homelab/VPS and real-device push acceptance remain operational checks.

The current deployment is [https://bikers.kz](https://bikers.kz), running the verified Linux/amd64 image on Jarvis behind the VPS HTTPS route. The existing account is explicitly granted administrator access and Web Push credentials are configured. Public TLS, assets, API privacy boundaries and certificate-renewal checks passed. The owner signed in, administrator navigation was verified, and one browser subscription was enabled. The provider accepted one diagnostic push, and the owner confirmed that it appeared on this Mac and clicking it opened the inbox. See [current deployment](operations.md#current-deployment).

The owner-approved Night Ride visual default uses semantic tokens in `apps/web/src/theme.css`, component-owned rules in `styles.css`, and locally bundled condensed type. The palette, forms, garages, club/media controls, admin, inbox and settings share the same surfaces, text, focus and semantic status colors. The browse hero uses the existing credited KTM 390 Adventure 2024 reference; model selection and photo-upload defaults retain their existing behavior. The [design review](design-review.md) records the subsequent navigation and motion polish without changing catalogue scope.

## Runtime boundary

The Go application serves the built React PWA, the API under `/api/v1`, and foreground WebSocket events. PostgreSQL stores accounts, OTP challenges, sessions, rides, participation, chat, bikes, maintenance, clubs, memberships, ride associations, and processed bike/club image content and metadata. Telegram Gateway is the only production authentication provider.

The [backend architecture guide](backend-architecture.md) maps the package ownership approved in `AGENTS.md`. Managers own application decisions and limits; database contracts expose local decision callbacks, and PostgreSQL owns locks, transactions, and stored-record mapping. HTTP/WebSocket resources handle transport, while `app` constructs dependencies and owns startup/shutdown. `make check-architecture` and the regular Go test suite enforce internal imports, including tests and build-tagged files.

The production image publishes one plain HTTP port. The existing VPS terminates TLS and forwards requests to the homelab over its private connection. Preserve the original `Host` header and support WebSocket upgrades with an appropriate idle timeout. `BASE_URL` must be the browser's exact public HTTPS origin; forwarded headers do not override it. PostgreSQL is private to the Compose network.

Source-based OTP quotas use the socket peer by default. Set `TRUSTED_PROXY_CIDRS` only to the actual, narrow proxy peers observed at the application boundary when enabling forwarded client addresses. The app walks the forwarded chain from the trusted side. The VPS must replace or safely append the client address; broad private-network trust would let unrelated callers supply their own quota identity. This setting does not change Host, Origin, or cookie security.

## Accounts and permissions

An OTP challenge binds the original normalized phone number to a server-side Telegram request. Verification accepts the challenge ID and code only. A Telegram API success envelope is insufficient: the result must match that request and phone and explicitly report `code_valid`. Challenges expire, have durable send/attempt limits, and can be consumed once.

The session cookie is opaque and HttpOnly; its digest is stored in PostgreSQL. Initial session defaults are seven days of inactivity and a thirty-day absolute lifetime. Public HTTPS uses a Secure `__Host-` cookie. Explicit loopback HTTP is supported for local development. Authenticated writes require an `X-CSRF-Token` header obtained from the session endpoint. Keep this token in memory. Neither session tokens nor OTP codes belong in browser storage or logs.

Public ride responses contain only ID, city, calendar date, description, and publication status. Authenticated callers can additionally see their own participation state and whether they manage the ride. Title, exact time and meeting details, destination, route, expectations, capacity, organizer identity, roster, and chat require approval or ownership. Pending requests do not reserve a place. The initial capacity default includes the organizer; this is an implementation default subject to product review.

The organizer approves or rejects requests and removes participants. Approval and capacity checks share a database lock. Cancellation and edits use `plan_version` to reject stale writes. Removing a participant revokes private HTTP and realtime access.

Phone verification does not award a profile badge. Administrators manually grant or revoke the audited Community verified badge; it does not establish identity, licence status, skill or safety.

## HTTP contract

JSON field names use `snake_case`. Errors have the form:

```json
{"error":{"code":"authentication_required","message":"Sign in to continue"}}
```

The web client translates error codes into its selected language; the server message is a diagnostic fallback. API responses use `Cache-Control: no-store`. The PWA caches public application assets only, with no offline cache of ride details, chat, sessions, or other API responses.

| Method | Path under `/api/v1` | Purpose |
| --- | --- | --- |
| GET | `/session` | `{user: null}` or `{user, csrf_token}` |
| POST | `/auth/challenges` | Send Telegram OTP for `{phone}` |
| POST | `/auth/challenges/verify` | Consume `{challenge_id, code}` and establish session |
| DELETE | `/session` | Revoke the current session |
| GET / PATCH | `/me/profile` | Read or update the current rider profile |
| GET | `/cities` | Suggested Kazakhstan cities with `value`, localized `names` and `aliases` |
| GET / POST | `/rides` | Paginated discovery/personal summaries or create a ride |
| GET / PATCH | `/rides/{id}` | Read the authorized view or edit an owned ride |
| POST | `/rides/{id}/cancel` | Cancel with the current `plan_version` |
| PUT / DELETE | `/rides/{id}/participation` | Request to join or leave |
| GET | `/rides/{id}/participants` | Approved roster; organizer also sees requests |
| POST | `/rides/{id}/participants/{userID}/decision` | Organizer `approve`, `reject`, or `remove` |
| GET / POST | `/rides/{id}/messages` | Read ordered history or persist a message |
| GET (WebSocket) | `/events?ride_id={id}` | Foreground hints to refresh authorized state |
| GET | `/bike-models?query=...` | Curated model/alias lookup with verified model-year image metadata, up to 20 results |
| GET / POST | `/me/bikes` | Page the owner's garage or create a private-by-default bike |
| GET | `/bikes` | Page explicitly public, non-hidden bikes; literal make/model search with `q`, UUID `cursor` |
| GET | `/bikes/{id}` | Owner record or explicitly public basic projection |
| PATCH / DELETE | `/me/bikes/{id}` | Edit/delete with `version` |
| GET / POST | `/me/bikes/{bikeID}/maintenance` | Page or create private maintenance entries |
| PATCH / DELETE | `/me/bikes/{bikeID}/maintenance/{entryID}` | Edit/delete a scoped entry with `version` |
| GET | `/bikes/{bikeID}/photos` | Authorized gallery; only the owner receives its separate `version` |
| POST | `/me/bikes/{bikeID}/photos` | Upload raw JPEG/PNG/WebP bytes; returns `{photo, gallery}` |
| PUT | `/me/bikes/{bikeID}/photo-cover` | Choose a ready photo or restore catalogue with `{version, photo_id}` |
| DELETE | `/me/bikes/{bikeID}/photos/{photoID}` | Delete with gallery `version`; returns `{gallery}` |
| GET / HEAD | `/bikes/{bikeID}/photos/{photoID}/content` | Authorized, processed JPEG bytes; always `no-store` |
| GET / POST | `/clubs` | Filter and page `view=discover|organizing|joined|pending` by `city`, name `q`, signed `cursor`, or create |
| GET / PATCH | `/clubs/{id}` | Authorized view or owner edit with `version` |
| POST | `/clubs/{id}/close` | Close explicitly with `version` |
| PUT | `/clubs/{id}/ownership` | Transfer with `{version, user_id}` |
| PUT / DELETE | `/clubs/{id}/membership` | Request membership or leave |
| GET | `/clubs/{id}/members` | Authorized roster |
| POST | `/clubs/{id}/members/{userID}/decision` | Owner `approve`, `reject`, or `remove` |
| GET | `/clubs/{id}/rides` | Public ride summaries, up to 100, upcoming published first |
| PUT / DELETE | `/clubs/{id}/rides/{rideID}` | Link/unlink a public preview association |
| GET | `/clubs/{clubID}/image` | Direct `{image, version?}` metadata; active and closed club images are public |
| POST | `/clubs/{clubID}/image?version=N` | Current active owner uploads/replaces with raw image bytes; `201` direct metadata |
| DELETE | `/clubs/{clubID}/image` | Current active owner removes with JSON `{version}`; direct `{image: null, version}` |
| GET / HEAD | `/clubs/{clubID}/image/{imageID}/content` | Current public JPEG for active or closed clubs; old/missing images return `404` |

### Ride lists and cities

`GET /rides` returns `{rides: [...], next_cursor: "..."}` with 20 summaries per
page and an empty terminal cursor. `view` is `discover` (default), `organizing`,
`joined`, `pending` or `past`; personal views require authentication. Organizing
means the caller's upcoming published rides; Joined means approved upcoming
rides organized by someone else; Pending means upcoming requested places.
Past includes earlier or cancelled rides the caller organized or has an
approved/requested relationship with. It is not an attendance record.

Optional filters are `city`, `from` and `to` (`YYYY-MM-DD`, inclusive in each
ride's timezone), and `q` (a literal case-insensitive substring of the public
description, at most 200 bytes). Known city spellings/aliases match the same
city, including existing stored localized names. Filters precede pagination.
Unknown views, invalid ranges and invalid cursors fail with `422` errors.

Pages order by the public ride-local date then UUID, ascending except Past,
which is descending. A cursor binds its filters, user and clock snapshot and
is authenticated with a process-local key. It never contains the exact ride
departure time. A restart invalidates existing cursors; start again without
`cursor`. Live edits or membership changes can still change which rows are
visible; pagination is not a database snapshot.

`GET /cities` exposes a suggested, non-exhaustive Kazakhstan vocabulary as
`{cities: [{value, names: {en, ru, kk}, aliases: [...]}]}`. Known values normalize
on profile/ride/club writes. Other trimmed city names remain valid, and no
geolocation or external lookup service is used. The same selector appears in
all three forms and discovery. Existing data needs no rewrite or migration.

Profile fields are `display_name`, `city`, `languages`, `interests`, and UI `locale` (`en`, `ru`, or `kk`). Display name, city, and at least one spoken language complete onboarding.

A full ride uses `title`, `city`, `description`, `starts_at` (RFC 3339), `timezone` (IANA name), `meeting_point`, `destination`, `route_url`, `expectations`, and `capacity`. Public `date` is calculated in the ride's timezone. Editing includes `plan_version` from the last authorized response.

A message uses a caller-generated UUID `client_message_id` and `body`. Retrying the same ID with the same body returns the existing message; a different body conflicts. Database-assigned room sequences determine ordering. Use `GET .../messages?after={sequence}` and follow `next_sequence` to catch up in batches of up to 100. Persisted messages remain the source of truth after reconnects; WebSocket events are hints, not durable background delivery. Cancelled rooms retain their history and reject new messages.

Bikes use `make`, `model`, optional curated `model_id`, `year`, and `visibility` (`private` by default or explicitly `public`). An owner sees the full record; everyone else sees only `id`, `make`, `model`, `model_id`, and `year` for a public bike. Private and missing bikes both return `404`. Maintenance is never included in public bike payloads. A selected model ID supplies canonical make/model values; freeform entries cover bikes outside the small curated vocabulary. Latin/Cyrillic aliases ignore case and punctuation.

The public `/bikes` catalogue uses `GET /api/v1/bikes?q=...&cursor=...` and returns
`{bikes, next_cursor}` with 20 results per page in stable UUID order. Search is a
case-insensitive literal make/model substring, bounded to 200 bytes. Visibility,
moderation and search filters apply before pagination. Every viewer, including
the owner, receives only the same five public bike fields in this collection.
Published-content blocking rules remain unchanged. No database migration is
required.

The catalogue reuses existing authorized photo covers and verified model
references. Search and cursors live in the URL; changing search or session resets
paging. Client cache entries whitelist the public projection and remain scoped
to the resolved viewer. Owner edits, sharing/hiding and deletion invalidate the
catalogue. Publishing confirmation explains that the bike and its public photos
can be discovered here; new bikes remain private by default.

Maintenance uses `service_date` (`YYYY-MM-DD`), integer `mileage_km` from 0 to 10,000,000, and trimmed `work_performed` up to 4,000 bytes. Dates must be valid and no later than today in Asia/Almaty. Backfilled entries do not require monotonic mileage. Owner-only bike pages use ascending UUID cursors; private maintenance pages use an opaque date/UUID cursor in descending date/ID order. Both return 50 records and `next_cursor` (empty at the end). Wrong bike/account entry IDs return `404`. Bike/entry edits and deletions require the current `version`; stale writes return `409 stale_version`.

Club summaries contain `id`, `name`, `city`, `description`, and `status`, plus the signed-in caller's own membership/management flags. Approved members and active owners can see `owner_id`, `version`, `member_count`, and the authorized roster. Pending requests do not expose the roster. The owner approves requests and transfers ownership only to an active approved member; the previous owner stays approved. An active owner must transfer or close before leaving. Closure increments the version and makes the club read-only except for leaving. A historical owner who leaves a closed club retains no private roster access; `can_manage` is false for all closed views.

Club owners link only their own published rides and may unlink any association in their club. Links expose public ride summaries and never confer ride approval or private plans, rosters, chat, or photo access. Club pages use ascending UUID cursors, 50 records per page, and `next_cursor`.

Health endpoints are `/health/live` and `/health/ready`. Readiness checks all seven migration receipts and writable reserved media storage. The explicit migration command applies versions 1–7, verifying checksums and rejecting unknown versions. Versions 5–7 add avatars/ride media, trust/admin state, and preferences/announcements/notifications. Applied versions 1–4 remain unchanged. Older installations must run the new migration command before readiness. Server startup does not apply migrations.

## Avatars albums announcements and inbox

| Method | Path under `/api/v1` | Purpose |
| --- | --- | --- |
| GET / POST / DELETE | `/me/avatar` | Self metadata, raw image replacement with query `version`, or removal with body `version` |
| GET / HEAD | `/me/avatar/{imageID}/content` | Self-only processed bytes |
| GET | `/rides/{id}/avatars/{userID}` or `/clubs/{id}/avatars/{userID}` | Avatar only when viewer and target have current approved context membership |
| GET / HEAD | Context avatar URL returned above | Reauthorize both users for current bytes |
| GET | `/rides/{id}/album` | Approved participants' album and optimistic version |
| POST | `/rides/{id}/album/photos` | Raw participant image upload |
| DELETE | `/rides/{id}/album/photos/{photoID}` | Uploader or organizer deletion with album `version` |
| GET / HEAD | `/rides/{id}/album/photos/{photoID}/content` | Current approved-participant image access |
| POST / DELETE | `/rides/{id}/album/videos` or `/rides/{id}/album/videos/{videoID}` | HTTPS external link creation or scoped removal |
| GET / POST | `/rides/{id}/announcements` | Approved read; organizer-only idempotent publication |
| GET / PATCH | `/me/preferences` | Independent locale settings and nested `push_categories` booleans |
| GET | `/me/notifications` | Descending cursor inbox, `unread_count` and `next_cursor` |
| GET | `/me/notifications/unread-count` | Actor-owned `{unread_count}` for global badges |
| GET / PUT | `/rides/{id}/reminders` | Current approved rider/organizer `{day_before,hour_before}` opt-in |
| GET | `/rides/{id}/calendar` | Private `no-store` iCalendar attachment for the current approved rider/organizer |
| PUT | `/me/notifications/{notificationID}/read` or `/me/notifications/read` | Idempotent read state for one or all |
| GET | `/me/push/settings` | Optional availability and public VAPID key |
| GET / POST / DELETE | `/me/push/subscriptions` or `/me/push/subscriptions/{subscriptionID}` | Consent registration and account-owned device removal |

An avatar has one current image and one pending replacement. Context endpoints expose no self-edit version and there is no global avatar directory. Album defaults are 24 photos per ride/six per uploader and twelve video links per ride/three per uploader. Removed members lose album and avatar access. Upload finalization rejects removal/reapproval, plan changes and account restriction during processing. Cancelled rides keep readable albums and allow deletion but reject new media. External videos are validated HTTPS links opened with safe browser attributes; the server does not fetch them.

Announcements use `client_announcement_id` and plain `body` up to 4,000 bytes, with UUID/body-bound retries and descending `before` pagination. They remain separate from chat and readable after cancellation. Significant plan changes, cancellations, announcements and participation transitions write inbox/outbox intent atomically. Inbox entries contain a kind and exactly one optional target field: `ride_id` for ride/participation events, or `club_id` for club membership events. They contain no applicant names, club names, meeting details or message bodies. Push payloads remain generic and always open `/inbox`. Push is optional and uses bounded leased retries outside locks; see [Operations](operations.md#optional-web-push).

### Club membership notifications

New membership requests create `club.membership.requested` for the current
owner. Actual approval, rejection and removal create
`club.membership.approved`, `club.membership.rejected` and
`club.membership.removed` for the affected rider. The inbox action opens
`/clubs/{club_id}` using the normal club permissions. A membership notice does
not grant roster or ride access.

Membership state, inbox insertion and optional push jobs commit together.
Repeating a request or a decision without changing state creates no duplicate
notice. A failed notification write rolls back the membership transition.
Withdrawal followed by a new request is a new event. Existing requests are not
backfilled, and ownership transfer does not move historical inbox entries to
the new owner; subsequent requests notify the current owner.

Push delivery retains account/session checks. Request alerts additionally
require an active club and current owner; approval alerts require an active
club and current approved membership or ownership. Rejection/removal notices
can reach the affected rider after membership access ends. These are
application decisions in `managers/notifications`; the repository loads current
club facts while claiming jobs, then network delivery occurs after commit.
The existing generic localized lock-screen text and `/inbox` destination stay
unchanged.

Migration 8 adds nullable `club_id` and enforces exactly one kind-matching
ride/club target while preserving old ride notices and push jobs. Versions 1–7
remain immutable. Apply the explicit schema upgrade as a coordinated rollout
before activating the matching executable. The current deployment helper coordinates
schema 8 to 9 with `--migrate-to 9`; the schema-7 to schema-8 rollout is historical.
See [the coordinated rollout](operations.md#community-follow-up-schema-9).

## Reporting blocking and administration

Signed-in users report accessible profiles/content through `POST /reports`; reports are bounded to ten per hour and duplicate open submissions are reused. `/me/trust`, `/me/blocks`, and `/me/blocks/{userID}` provide administrator status and bilateral block controls. `/users/{userID}/badge` returns the separately assigned community signal. `/admin/reports`, scoped report evidence, `/admin/actions` and `/admin/audit` require a persisted administrator role. [Operations](operations.md#administrator-access) describes the operator-only role command.

Blocking prevents new ride join/approval with the organizer and club join/approval with the owner. Existing approved memberships retain coordination plans, roster, cancellation and organizer announcements. Ordinary messages, contextual avatars and private album media from blocked users are filtered for either side. Public basic bikes/clubs and their public images remain public, including anonymously; blocking is not a privacy boundary for published content. The UI explains this policy.

Hidden rides/clubs leave public discovery/previews; existing approved members retain coordination. Hidden image bytes return `404` even to their author. Owners/organizers retain deletion placeholders where appropriate. Reports retain bounded immutable text/metadata and reference existing bytes for administrator review without duplicating images. Later owner deletion can make those bytes unavailable. Restriction revokes sessions atomically and wakes foreground access checks. Admin hide/restore, restrict/restore and badge actions require reasons, audit entries and UUID/payload-bound idempotency. Trust change hints clear mounted protected client caches before authorized refetches.

## Model reference images

The public model lookup returns an `images` array for each canonical model. Each image has `url`, inclusive `year_from`/`year_to`, dimensions, source URL, author, license, and license URL. Empty arrays mean no verified coverage. The picker, garage cards, owner page, and public bike page resolve the exact canonical `model_id` and selected year; freeform names and unsupported years never receive an approximate image. These catalogue assets are independent of rider uploads and private ownership records. Source credits and the current 25 exact-year entries are listed in [Model reference photographs](model-reference-images.md).

Model-reference validation passed PostgreSQL-backed model/garage/HTTP race checks, vet, architecture enforcement, TypeScript/ESLint, all 43 frontend tests, and web/container builds. The expanded catalogue has 22 canonical models and 25 exact-year references. Every model remains searchable beyond the 20-item suggestion limit. Browser review verified Cyrillic lookup, the transparent KTM 390 Adventure 2024 PNG, no image for unsupported 2023, and CFMOTO 700MT 2024 beyond the first suggestions. Earlier review covered the owner/public views, EN/RU/Kazakh captions, mobile layout, and credits. The final flows had no console errors or warnings. The local image serves all 25 assets with hashes matching the source manifest; pictures load on demand and are excluded from the 12-entry PWA precache. This change needs no database migration and sent no real OTPs.

## Bike photo uploads

The catalogue remains the cover until an owner explicitly chooses an uploaded photo. Uploading alone never changes the cover; an empty `photo_id` restores the catalogue, and deleting the selected cover restores it atomically. Unsupported catalogue years still have no reference image. Uploads also work for freeform models. Gallery visibility and every content GET/HEAD follow the bike's current visibility: owners can read private photos; other callers receive `404` until the bike is explicitly public. Missing photos also return `404`. Club membership grants no photo access. The existing Bike/PublicBike fields and owner-only maintenance boundary are unchanged.

The separate gallery returns `{photos, cover_photo_id}` and an owner-only `version`. Each safe photo contains exactly `id`, `url`, `width`, `height`, and `created_at`; storage content and owner identifiers are absent. Cover/delete writes use the gallery version, independently of bike edits. Upload uses an authenticated CSRF-protected raw body with `image/jpeg`, `image/png`, `image/webp`, or `application/octet-stream`; multipart, JSON, and base64 are unsupported. Content URLs are authorized same-origin API routes with no conditional-cache bypass. The PWA does not cache photo content; logout/account changes discard gallery state and abort pending uploads.

## Club images

Each club has one optional public image, shown on its card and detail page with initials as the fallback. Both active and closed club pages expose the current image; closure does not make it private. Only the current owner with an active account can upload, replace, or remove it while the club is active; upload also requires a completed profile. Membership grants no image-edit rights. Closed clubs reject all image writes. Existing Club/ClubSummary/member fields and roster permissions stay unchanged.

The separate metadata endpoint returns `{image: null}` or `{image: Photo}`; the safe Photo has exactly `id`, `url`, `width`, `height`, and `created_at`. Only the current owner of an active club, with an active account, receives the separate image `version`. Upload uses a CSRF-protected raw body with the same MIME rules as bike photos and one positive decimal `?version=N`; successful upload returns `201` direct metadata and selects the new club image immediately. Removal uses JSON `{version}` and returns direct metadata with `image: null` and the next version. Stale writes return `409 stale_version`; an existing pending replacement returns `409 club_image_busy`.

Replacement/removal atomically updates the current reference and deletes old ready content. Old image URLs then return `404`; failed, cancelled, stale, or expired replacement preserves the previous image. Transfer preserves the ready image. Upload finalization checks the club and image versions captured at reservation, so transfer away and back or closure during processing rejects the upload. The original uploader remains charged for stored bytes after transfer until replacement/removal; quota attribution never grants image access or edit rights.

Content GET/HEAD serves only the current ready JPEG, with `no-store`, `nosniff`, the correct length, and no cache validators. The PWA never caches image API/content. The editor aborts and discards late upload results when the actor, club, ownership, or active status changes; a failed session cannot restore protected editor state.

## Shared image processing

Garage, clubs and media retain separate authorization and lifecycle rules. `app` injects one `managers/photos` module into all three, sharing the codec, metadata stripping, processing slots, payload reservations, and upload-attempt limits. Current implementation defaults:

| Limit | Value |
| --- | --- |
| Source image | JPEG/PNG/WebP, at most 12 MiB, 12,000 pixels per edge, and 32 million decoded pixels |
| Stored image | Fresh JPEG quality 85, at most 1,920 pixels per edge and 4 MiB; no upscaling |
| Processing | Two shared slots per application instance; uploads have a 45-second request context and the server has 60-second read/write timeouts |
| Photo count | Six pending + ready photos per bike; one ready image and at most one pending replacement per club |
| Payload accounting | Combined bike/club/avatar/ride payload: 100 MiB per original uploader and 1 GiB globally, including reservations |
| Reservation | 4 MiB pending capacity, expires after ten minutes |
| Attempts | Twelve combined image reserved attempts per uploader per rolling hour, including invalid-image failures; attempts older than 24 hours are pruned |

Processing validates still-image containers, applies validated orientation, flattens transparency onto white, and re-encodes pixels without EXIF/GPS. Original bytes, filenames, and source metadata are discarded. Animated PNG/WebP, malformed/truncated images, GIF/SVG, and HEIC/AVIF are rejected; export HEIC as JPEG before uploading. A shared slot is acquired before reservation/body reading. Processing runs outside database locks, and the permit stays held until synchronous codec/metadata work has returned.

PostgreSQL `bytea` holds processed content with metadata. A shared global advisory lock precedes owner → bike locks for bike photos and club → actor account locks for club images. All reservation paths count and lazily expire pending rows across the four image tables. A common exclusive trust advisory lock precedes the photo lock, serializing moderation and blocks with finalization. Ready content, image/gallery versions, references, and deletion commit atomically. Interrupted uploads leave at most invisible pending reservations. Failure cleanup uses an independent three-second context to delete only the uploader's pending row, preserving ready content after a lost successful response. Refresh gallery/image metadata after an uncertain result before retrying. There are no filesystem photo blobs, GC queues, or cleanup workers; `MEDIA_DIR` remains reserved for future media.

Database backups include all processed images, gallery/cover state, current avatar/club-image references, and ride albums. Include them in restore verification; the application asset manifest covers curated reference images separately. Payload caps do not bound total database/WAL/backup disk usage. The `bytea` choice and numeric caps above are implementation defaults inferred from the current bounded slice, not confirmed product policy. A future filesystem/object-storage design remains a proposal; no migration to it or schedule is approved.

## Release boundary

### Motorcycle visual refresh verification

On 4 November 2026 the refresh passed all 88 frontend tests, TypeScript, ESLint and production builds for Linux/arm64 and Linux/amd64. Browser checks covered desktop and 390-pixel English/Russian/Kazakh layouts, a 320-pixel Kazakh headline, mobile navigation, settings/admin controls, and selecting the correct 2024 catalogue image without saving a motorcycle. The bundled font rendered the Kazakh-specific glyphs without a system-font fallback. Production artifacts omit development preview controls and sample rides, and the PWA precaches the new font.

An existing cache-guard test was timing-dependent because its unobserved sentinel query used `gcTime: 0`. The test now retains that specific query and explicitly yields a timer turn before asserting the late-request behavior. Twenty isolated repetitions and the full suite passed; production session behavior was unchanged. Both deployed backend binaries matched their previous platform releases, and all 46 public artifact hashes matched the new image over normal HTTPS. Current release identifiers are in [Operations](operations.md#current-deployment).

### Remaining pilot work

The application is deployed at [bikers.kz](https://bikers.kz). Public HTTPS, homelab/VPS routing and the retained owner account have been verified; current push acceptance is recorded in [Operations](operations.md#current-deployment). Before a public pilot, complete human translation review, moderator/badge criteria, account lifecycle/retention policy and operational limits. The owner declined backup work for this visual-refresh pass. The [technical build plan](technical-design-and-build-plan.md) records the wider phases and open design decisions.

Do not use seeded accounts or an authentication bypass in the deployed server. Automated tests inject a fake Gateway into test builds; they never send real OTPs.

## Initial MVP verification recorded on 3 November 2026

The complete MVP passed the full PostgreSQL-backed Go race suite, vet, architecture checks, 88 frontend tests, TypeScript/ESLint and production/container builds. Browser journeys verified two-rider approval/inbox flows, avatar and album uploads, external video links, separate announcements, reports/evidence/hiding, audited manual badges, block/unblock filtering with retained coordination, independent settings, three languages, PWA updates and a 390-pixel phone layout. The mounted block-list cache race found during browser review has a regression and passed the final browser recheck.

A disconnected PostgreSQL 18.6 upgrade/replay and second backup/restore matched all 28 tables/two sequences, all four image hashes, current-image selections, notification read state, push leases and moderation/audit relationships. The initial MVP image `bbe5fb969b47ca7859bd09210b2d6cea5c3197af4fafa33912fb94a1626ee89b` was verified healthy at `http://localhost:33043`, with receipts 1–7, unprivileged/read-only execution and a loopback-only app port. A fresh private backup was taken before the actual upgrade; all original records matched before app replacement, and the existing authenticated browser session was verified afterward. The root `.env` stayed unchanged. Private rollout configuration and backups are under `/private/tmp/bikerskz-mvp-rollout/`; the independent restore evidence is under `/private/tmp/bikerskz-mvp-backup-20261103/`.

At that initial MVP checkpoint, administrator roles and Web Push were available through the documented operator setup; no live role, credentials or permissions had been granted, and no real OTP or push was sent in that implementation pass. Subsequent owner-authorized administrator activation and public deployment are recorded in [Operations](operations.md#current-deployment).

### Earlier slice checks

The club-image slice passed the full PostgreSQL 17-backed Go race suite, vet, architecture enforcement, TypeScript/ESLint, all 71 frontend tests, and the production web/image build. Regressions cover public active/closed reads, owner/CSRF restrictions, atomic replacement/removal, old-URL rejection, transfer/closure during upload, and shared bike/club quotas, attempts, expiry, and processing slots. Browser checks verified file-picker upload, card/detail images, remove-confirmation cancellation, EN/RU/Kazakh, anonymous closed-club reads, and closed-club editing restrictions, with no console errors or warnings. The final CSS adjustment and checks passed; the 390-pixel Kazakh layout has no horizontal overflow.

A private PostgreSQL 18.6 backup/restore rehearsal applied migration 4 and two replays while preserving existing records. Synthetic bike JPEG bytes, selected cover, and the current 960 × 720 club JPEG survived a second backup/restore and replay. The final local image is healthy at `http://localhost:33043` with receipts 1,2,3,4 and the existing account/session retained. No real OTPs were sent for this slice. Homelab/VPS deployment remains unverified.

The earlier bike-photo milestone passed the PostgreSQL-backed Go race suite, vet, architecture enforcement, and 56 frontend tests. Regressions cover private/public content, manual cover/reset, atomic deletion and restart persistence, metadata stripping/orientation, malformed input and size bounds, competing global reservations, per-owner limits, lazy expiry, and failed-attempt limits after application restart. Final metadata processing is synchronous and bounded; processor/HTTP/quota race checks passed after review. Browser checks verified real file selection, upload retaining the catalogue default, explicit cover/reset, confirmed deletion restoring the default, mobile layout, EN/RU/Kazakh controls, and the anonymous public gallery, with no console errors or warnings. A private PostgreSQL 18.6 restore rehearsal preserved existing accounts/sessions through migration 3 and replay; synthetic JPEG bytes, dimensions, and selected cover survived a second database backup/restore. That image was verified healthy with receipts 1,2,3 and the existing account/session retained. This slice sent no real OTPs.

The garage/club backend adds PostgreSQL-backed regressions for private/public bike separation, maintenance ownership and cursor pages, model lookup, approved club membership, ownership transfer/closure, and concurrent transfer versus leaving. These additions do not imply that the remaining media/admin release gates or homelab deployment are complete.

The garage/club slice passed the full PostgreSQL-backed Go race suite, vet, architecture enforcement, TypeScript/ESLint, all 30 frontend tests, and the production web/image build. Browser checks used disposable accounts for model aliases, private maintenance editing, explicit basic-page sharing, membership requests/approval, ownership transfer, closure/leave, confirmation cancellation, EN/RU/Kazakh core screens, and mobile layout. The final flow had no console errors or warnings. Regressions also cover session-failure cache privacy, delayed mutations after logout/account changes, and upcoming club rides remaining visible after more than 100 historical links.

A private backup of the existing local PostgreSQL 18.6 database was restored to an isolated container. Migration 2 and its replay preserved account/session records, and the new image passed readiness with an unprivileged user, read-only root filesystem, and private database networking. That migration and image were then applied to the loopback-only local app at `http://localhost:33043`; readiness and the retained authenticated session were verified. Existing user tabs were left untouched. The PWA offers an explicit Update app action to load the new screens. This slice sent no real Telegram codes.

The first slice passed PostgreSQL-backed HTTP/WebSocket integration tests with the Go race detector, Go vet, 17 frontend tests, TypeScript and ESLint checks, and the production web/container builds. Browser review covered two-rider onboarding, creation, pending access, organizer approval, private plans/chat, English/Russian/Kazakh display, and a 390-pixel mobile layout. The browser console reported no errors or warnings in the final flow.

An application-replacement regression preserves sessions, an outstanding OTP challenge, approvals, chat history, idempotent retries, and the next room sequence after replacing the HTTP application and database pool. An isolated Compose smoke test with PostgreSQL 18.6 confirmed unprivileged execution, private database networking, explicit migration, built assets/PWA icons, and database/media persistence across container replacement. Integration tests used PostgreSQL 17. A subsequent live test confirmed real Telegram delivery and OTP sign-in, with the session retained after replacement by the clean application image. Homelab/VPS deployment was not exercised.

Production npm dependencies reported no audit vulnerabilities at this verification. Development/build tooling reported 21 low/moderate transitive advisories and no high/critical findings; these are still maintenance work. The production image contains built browser assets and the Go executable, with no Node dependency tree.

Live testing exposed Telegram returning an international phone number without a leading `+`. The initial literal comparison rejected that successful response. Provider phone values are now canonicalized as international numbers before matching the server-owned phone; national-number guesses, malformed values, different phones, mismatched request IDs, and non-valid verification statuses still reject. HTTP regressions reproduced both challenge-creation and code-verification failures before the fix, then passed with the full PostgreSQL race suite and Go vet. Temporary diagnostic probes were removed from the final image.

The local real-provider test runs at `http://localhost:33043` in the separate `bikerskz-telegram-test` Compose project, bound to loopback with a private PostgreSQL database. The original root `.env` was unchanged. Generated test defaults and the loopback Compose override are under `/private/tmp/bikerskz-live-telegram/`; the verified account and session remain in that project's database volume. This is a local test instance, separate from the planned homelab deployment.

## Browser review without real OTP delivery

After building the web app, run the opt-in Go test harness against a disposable PostgreSQL database:

```sh
cd apps/backend
BROWSER_HARNESS=1 TEST_DATABASE_URL='postgres://user:password@127.0.0.1:5432/test_db?sslmode=disable' go test ./internal/app -run '^TestBrowserHarness$' -v -timeout 20m
```

Open `http://127.0.0.1:8081`. In this test binary only, use a valid phone number and code `123456`. Each run creates an isolated schema, and the harness stops after ten minutes by default. Set `BROWSER_HARNESS_SECONDS` and the Go test timeout together for longer reviews. Cleanup removes the test schema on normal exit; do not use this against a production database. For the club-notification browser fixture,
set `BROWSER_HARNESS_CLUB_NOTICES=1`. This seeds a disposable club, owner
`+77011239902` and applicant `+77011239903`, with actual request/decision inbox
history; both use the fake code `123456`. The latest request remains pending so
an owner can approve it through the UI, then sign in as the applicant to inspect
the new notice. All fixture generation is compiled only into the test binary.


For frontend hot reload, run Vite on a fixed port (`npm run dev -- --port 5173 --strictPort`) and run the API on port 8080 with `BASE_URL=http://localhost:5173`. The browser origin, including hostname and port, must match. The Go CLI reads exported environment variables; it does not load `.env` automatically. Compose loads the root `.env` and supplies the internal paths and database URL.

## Community and schedule follow-up — 4 November 2026

The owner approved three additions: filtered club discovery/My clubs, unread
badges and push-category settings, and opted-in ride reminders/private calendar
exports. The implementation requires explicit migration 9; versions 1–8 remain
immutable. Category preferences apply only to push, with every category enabled
by default and all inbox updates retained. Per-ride reminder consent defaults
to false and binds to the current participation generation.

The reminders manager decides 24-hour/1-hour eligibility with a ten-minute grace
window. PostgreSQL serializes occurrence receipts and inbox/outbox writes under
the existing trust/ride locks. A 30-second worker calls the manager; provider
network calls remain in the existing push dispatcher after commit. Rescheduling,
cancellation, account restriction, removal/reapproval, hidden rides, blocks and
opt-out invalidate stale reminder delivery. Downloaded calendar files use stable
UIDs and plan sequences, UTC departure, private classification and cancellation
status, and remain snapshots. This development change does not perform a
production migration, OTP request or live notification test.


## Self-hosted maps

`make up` creates the local internal `bikerskz_gis` network if needed. Leave
`PHOTON_BASE_URL`, `VALHALLA_BASE_URL` and `MAPS_ARCHIVE_URL` empty to develop
text-only rides. A map-enabled environment uses private Photon and Valhalla
origins plus a versioned same-origin archive path such as
`/map-data/2583113c45ca/kazakhstan.pmtiles`.

The Vite development server forwards `/map-data/` to `BIKERSKZ_DEV_MAPS_TARGET`
(or the API target by default). Address queries and route plans stay online-only
and private; the service worker does not cache them. The test-only browser harness
can opt into explicitly supplied GIS origins with `BROWSER_HARNESS_GIS=1`.

See [GIS operations](operations.md) for the durable Jarvis services and guarded,
serial data staging/refresh commands. Development must use disposable databases;
production accounts and sessions are never copied into a local map fixture.

## Useful places

The public `/places` directory shares the self-hosted basemap and adds bounded
category, text, city and viewport filters. The public map configuration is
separate from authenticated address/route APIs. Precise ride plans are never
used as directory data or copied into public caches. Opening the directory
does not request device location.

Schema 11 adds public places and private moderation submissions; versions 1–10
are unchanged. Active signed-in riders submit additions, corrections or closure
reports. Publication requires an active administrator and current place/submission
versions under the transaction locks. A stale correction requires a fresh
proposal. Contributor/reviewer identities and pending details are absent from
public places. Contributor history and administrator queues use private queries.

The CLI `api places import --file FILE --sha256 DIGEST` consumes a bounded,
fully validated local NDJSON artifact atomically. Replays keep IDs stable;
community edits and closures survive later OSM refreshes. There is no public
bulk-import endpoint or network fetch inside the import transaction.
See [Places data](places-data.md) and the
[coordinated schema rollout](operations.md#useful-places-schema-and-data).

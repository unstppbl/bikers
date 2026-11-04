# BIKERS.KZ operations

The supported deployment is one Go/PWA container and a private PostgreSQL service. The homelab serves plain HTTP; the owner-operated VPS terminates public HTTPS and forwards over WireGuard. Current rollout evidence is recorded below; an off-host backup destination and recovery targets remain operational decisions.

## Current deployment

On 3 November 2026 the application was deployed to Jarvis under `/home/user/apps/bikerskz`, using its verified Linux/amd64 platform and free host port 3050. Port 3042 belongs to another application. The private database restore matched all 28 tables and two sequences, including the existing account/session and the explicitly requested, audited owner administrator grant. Both containers are healthy; PostgreSQL has no published port. Hoster can reach the application over WireGuard at `10.0.0.2:3050`.

The production origin is [https://bikers.kz](https://bikers.kz), with `10.0.0.1/32` as the verified trusted proxy peer. At the owner's explicit request, hoster now serves the dedicated nginx route and matching Let's Encrypt certificate, valid through 1 February 2027. HTTP redirects to HTTPS. Independent public checks passed certificate chain/hostname validation, readiness, built asset hashes, PWA assets, unauthenticated API protection/no-store and foreign-origin write rejection.

The owner subsequently signed in at the public origin with administrator navigation visible. Arc notification permission was allowed, Settings confirmed notifications enabled, and the database confirmed one active owner-owned browser subscription. One selected-subscription diagnostic push was accepted by the provider (CLI exit 0), without retries or synthetic rides, inbox items or delivery jobs. The owner confirmed that the notification appeared on this Mac and clicking it opened `/inbox`. The private receipt is `.deploy/owner-push-test-20261103.json` on Jarvis.

Production releases now build from the Git checkout on Jarvis; the current
revision and image are recorded in Docker's OCI labels and
`.deploy/current-git-release.json`. Use the [repository deployment commands](#repository-deployments-on-jarvis)
below to inspect the exact running release. The last pre-Git image was
`bikerskz:nightride-d7beed053035-amd64`, ID
`8b5c86714811a5fb26078d45373cfb3826412e762db2f715dd106daefc71c131`.
The VPS route preserves WebSocket upgrades, allows 13 MiB requests and uses a 3,600-second idle read timeout with proxy caching disabled. Existing unrelated nginx configuration fingerprints were unchanged. Private pre-change edge backups are under `/root/bikerskz-edge-backups/20261103T154900Z` on hoster; app/import and prior-environment backups remain under the homelab app's private `backups` directory.

On 4 November 2026 the owner-requested motorcycle visual refresh replaced only the app image. The owner approved Night Ride, with charcoal surfaces, yellow accents, local condensed typography and a credited model photograph. The subsequent [Unslop/Stageshow pass](design-review.md) refined navigation spacing, mobile keyboard/focus handling, route scroll behavior and reduced motion. Its synthetic walkthrough is development-only, with separate exported clips.

For that pre-Git visual release, all 46 built public artifacts matched the image over validated HTTPS, including the font, font licence, icons and service worker. The font remains in the 19-entry PWA precache. Both platform builds shared index SHA-256 `a6de69963bff7871a2215be038650624969a524ba41019ea5563ec69477a4105`; the production bundle excluded the walkthrough. The backend executable matched the previous release byte-for-byte; schema 7, account/admin/session/subscription fingerprints, protected configuration and the database container were unchanged. Counts remained one account, one administrator, two sessions and one active push subscription. No migration, database dump, backup job, edge change or additional OTP/push send formed part of this update. Safe receipts are in `.deploy/nightride-d7beed053035/` on Jarvis and `/private/tmp/bikerskz-homelab-staging/nightride-d7beed053035/` locally. Initial visual-refresh receipts remain under `nightride-6014b23af8bc`. The prior images remain available for a same-schema rollback.

The certificate renewal timer is enabled and active. A domain-scoped `certbot renew --dry-run --cert-name bikers.kz` succeeded using the existing staging account, without changing account contacts or accepting new terms. Deployment receipts are retained in the homelab app's `.deploy/edge` directory and locally under `/private/tmp/bikerskz-homelab-staging/`.

The stable VAPID pair is stored in ignored, mode-0600 `.env.webpush` and deployment environment files, using `https://bikers.kz` as the owner-provided operator contact. Never regenerate this pair during ordinary deployments. The homelab's private `.env` has its own database password and the existing Telegram token. No secret values belong in these notes.

The independent local review instance remains on `http://localhost:33043`, with durable `.env.local` and a loopback-only Compose override:

```sh
docker compose --env-file .env.local -f docker-compose.yml -f docker-compose.local.yml -p bikerskz-telegram-test up -d --no-build
```

The local and homelab databases are independent after the initial import. A browser must sign in and enable notifications separately at the public origin; localhost cookies and push subscriptions are not transferred between origins.

The local review image is `bikerskz:skills-polish-20261104-arm64`, ID `1f657849c2fbf08782f1439752feaace98722ac9a2b3fc454d70226356982479`. It has the same frontend index and assets as the homelab release, with its previous backend executable, private database container, environment and loopback port retained. The existing local authenticated administrator session was verified after using the normal **Update app** prompt. Both `.env.local` and `.env.homelab` select their corresponding current platform images and remain mode 0600. The local guard receipt is `/private/tmp/bikerskz-skills-local-verification.json`.

## Repository deployments on Jarvis

[GitHub Actions deployments](github-actions-deployment.md) describes the public CI, private deployment repository, homelab runner and automatic release gates. Manual releases use the same host lock and deployment helper.

The owner requires production images to be built on Jarvis from pushed Git
commits. The source checkout lives at `/home/user/apps/bikerskz/repo`; private
runtime settings remain in `/home/user/apps/bikerskz/.env`, outside Git.
The Compose project remains `bikerskz`, retaining `bikerskz_postgres`,
`bikerskz_media` and `bikerskz_default`. The repository Compose file is now the
source of service configuration. No local image transfer is part of this path.

Initial checkout, once the intended commit has been pushed:

```sh
ssh user@100.64.0.3
cd /home/user/apps/bikerskz
git clone https://github.com/unstppbl/bikers.git repo
cd repo
make deploy-jarvis
```

Subsequent releases use the existing SSH access and checkout:

```sh
cd /home/user/apps/bikerskz/repo
git pull --ff-only
make deploy-jarvis
```

`scripts/deploy-jarvis.py` requires a clean checkout matching `origin/main`, a
private runtime `.env` and the existing healthy app/database. It serializes
deployments with a host-local lock, builds natively on Linux/amd64, and tags the
image `bikerskz:git-<full-commit>`. The image's
`org.opencontainers.image.revision` label records that exact commit.

Activation atomically changes only `APP_IMAGE` in the private runtime file and
runs Compose with an explicit environment path, checkout directory and project
name. The default app-only mode replaces the app with `--no-deps --no-build --wait`
and never runs migrations or starts/recreates the database. The explicit
[schema-9 mode](#community-follow-up-schema-9) coordinates its schema cutover. It verifies retained environment,
ports and media mounts, an unchanged database container/start time, and a
healthy non-root/read-only app. App-only failures, or migration failures with
confirmed unchanged schema 8, restore the previous environment bytes and app
image using the resolved configuration held in memory. A committed or unknown
schema upgrade keeps the app stopped for roll-forward recovery. It does
not create environment copies, database dumps or backups.

After success, `.deploy/current-git-release.json` records the commit, image,
previous image and verification outcome without secrets. Inspect the live
release rather than inferring it from a local branch:

```sh
cd /home/user/apps/bikerskz/repo
docker inspect bikerskz-app-1 --format '{{.Config.Image}}'
docker inspect bikerskz-app-1 --format '{{index .Config.Labels "org.opencontainers.image.revision"}}'
git log -1 --oneline
curl --fail --silent https://bikers.kz/health/ready
```

Configuration or schema changes need their own reviewed rollout. Migrations
require an explicit supported target; the default helper never applies them. For a same-schema rollback, select a
retained previous image in the private runtime `.env`, then use the repository
Compose file while keeping the database running:

```sh
cd /home/user/apps/bikerskz/repo
docker compose --project-directory "$PWD" \
  --env-file /home/user/apps/bikerskz/.env -p bikerskz \
  -f docker-compose.yml up -d --no-deps --no-build --wait app
```

Use the schema-generation constraints in [Backup and restore](#backup-and-restore)
when considering older images. Secrets, sessions, push keys and database state
stay outside the checkout.

### Club notification schema upgrade

The completed club-notification release upgraded schema 7 to 8 with a
coordinated app pause. It preserved existing ride notices/read state, push
subscriptions/jobs and membership data without backfilling alerts. That release
is the source generation immediately before the current schema-9 upgrade.
The current helper supports the explicit 8-to-9 transition below.

### Community follow-up schema 9

Club discovery/My clubs, unread badges, push categories and explicit ride
reminders/private calendar downloads require migration 9. Use the clean pushed
checkout on Jarvis:

```sh
cd /home/user/apps/bikerskz/repo
git pull --ff-only
make deploy-jarvis DEPLOY_ARGS='--migrate-to 9'
```

This mode accepts only source migrations 1–9 and exact matching existing
receipts 1–8. It builds before stopping only the app, runs the verified image's
`migrate` command without HTTP/push/reminder workers, confirms receipt 9, then
activates and waits for health. PostgreSQL and persistent volumes stay running.
A deployment lock prevents concurrent releases. A uniquely named migrator is
confirmed terminated before an uncertain exit can be classified as uncommitted.

Migration 9 retains existing inbox/read state, sessions, subscriptions, queued
push work and prior receipts. With app writes paused, the helper verifies all
28 prior tables and both sequences, excluding only the four new users preference
columns, the two new nullable notification reminder fields and receipt 9.
Existing `club_id` values are retained in the comparison. It separately requires
both new reminder tables to be empty and every existing push category enabled;
no rider is automatically enrolled. Safe proof is saved in the release receipt.

Explicit per-ride consent stores independent enabled-at timestamps for 24-hour
and 1-hour choices, so changing one cannot suppress the other. Removing and
reapproving a rider requires new consent. The reminder worker polls every
30 seconds, with a ten-minute grace window and durable occurrence receipts.

If migration or activation fails, the old image is restored only after schema 8
is confirmed unchanged and the migrator is settled. Committed or unreadable
schema 9 keeps the app stopped and the target selected for roll-forward recovery;
schema-8 code is never silently restored against schema 9. The default helper
remains app-only. No mode creates dumps/backups, copies private environment
files, regenerates credentials or sends diagnostic notifications/OTPs.

## Configure and start

Keep `.env` private and outside version control. Set `BASE_URL` to the exact browser origin, `APP_PORT` to a free homelab port, a unique URL-safe `POSTGRES_PASSWORD`, and `TELEGRAM_GATEWAY_TOKEN`. Do not enable fake authentication in the deployed executable; test authentication exists only in Go test binaries.

```sh
make config
make build
make migrate
make up
```

Migrations are explicit. Version 5 adds avatars and ride media, version 6 adds moderation, blocks, roles and badges, version 7 adds preferences, announcements, inboxes and push delivery, version 8 adds club membership notification targets, and version 9 adds push categories and explicit ride reminders. All nine receipts and their checksums are checked by readiness. Previously applied SQL is immutable. Unknown versions and mismatched checksums fail closed.

## Administrator access

Signup never grants administrator access. After the intended administrator signs in, obtain their user UUID from their own `/api/v1/session` response. Grant the role through the operator command:

```sh
docker compose run --rm app admin grant USER_UUID --reason 'Initial project administrator'
# To remove the role:
docker compose run --rm app admin revoke USER_UUID --reason 'Administrator handover'
```

The command requires a real active account and records a CLI audit entry. It never takes a phone number. The signed-in administrator can then open `/admin` to review reports and evidence, hide or restore content, restrict or restore accounts, and grant or revoke **Community verified** badges. Every action requires a reason and is audited. Repeating an action UUID with different content conflicts. Account restriction revokes sessions; restoring the account requires a fresh login.

The badge is a manual community signal. It does not establish identity, licence status, riding skill or safety. Badge criteria and moderator operating responsibility remain product/operations decisions.

## Optional Web Push

The durable inbox works without browser permissions or push credentials. Generate one stable VAPID pair into a new private file using the executable or a locally built CLI:

```sh
cd apps/backend
go run ./cmd/api push-keys --output /private/tmp/bikerskz-push.env --subject mailto:operator@example.com
```

Use an actual operator contact URI. This command creates a new mode-0600 file and refuses to replace it; it does not print keys. Transfer its `WEB_PUSH_PUBLIC_KEY`, `WEB_PUSH_PRIVATE_KEY`, and `WEB_PUSH_SUBJECT` values into the deployment environment through the existing secret-management process. Keep the matching pair stable, back it up securely, and recreate the app after configuration changes. Never commit the generated file or publish its private key.

Users enable notifications in Settings. Permission is requested only after their button press; denied, unsupported and unconfigured states have guidance. iPhone/iPad users need a supported Home Screen web app. Test supported devices through the real HTTPS origin. [Push API](https://developer.mozilla.org/en-US/docs/Web/API/Push_API), [Apple Web Push guidance](https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers)

Subscriptions are tied to a session, with at most five per account and device-removal controls. Delivery rechecks account, session and applicable ride/club membership or ownership. Club request alerts require the current active-club owner; club approvals require current active-club membership/ownership. Rejection and removal alerts can reach the affected rider after membership access ends. Rebinding an endpoint clears queued deliveries from its previous session. The worker leases durable jobs, makes network calls outside transactions, retries transient failures up to five attempts, and removes expired endpoints on 404/410. Delivery is at least once; notification tags reduce duplicates. Push uses generic localized text and a fixed `/inbox` link, with no rider name, club name, ride title, meeting place, phone number or announcement body on the lock screen.

Provider HTTPS endpoints are allowlisted and DNS results must be public addresses. Redirects and proxy environment variables are ignored. Unsupported push providers require an explicit allowlist update. The client uses the established [webpush-go library](https://github.com/SherClockHolmes/webpush-go) for encryption and VAPID.

After a browser opts in, an operator in the deployed Compose directory can send one generic diagnostic to its selected subscription without creating a ride, inbox item or delivery job:

```sh
docker compose exec -T app /app/api push-test USER_UUID --subscription SUBSCRIPTION_UUID
```

This checks active account ownership and the current session binding, then makes one bounded provider call outside the database transaction. Provider acceptance does not prove that the device displayed the notification. Confirm its display and that clicking it opens `/inbox`. Routine ride notices continue through the durable worker rather than this diagnostic.

For cross-platform builds, build frontend assets on the builder's native platform and cross-compile Go with BuildKit's target OS/architecture. The runtime image targets the deployment host. [Docker multi-platform builds](https://docs.docker.com/build/building/multi-platform/)

## VPS contract

- Preserve the original host and configure `BASE_URL` to the external HTTPS origin; forwarded headers do not override origin/cookie checks.
- Forward WebSocket upgrades for `/api/v1/events`, with reconnect-friendly idle timeouts.
- Permit raw JPEG/PNG/WebP uploads up to 12 MiB plus protocol overhead; allow at least 60 seconds for image requests. Disable response buffering where it interferes with realtime connections.
- Set `TRUSTED_PROXY_CIDRS` only to the actual narrow proxy peers observed by the app. Replace or safely append client addresses. Keep PostgreSQL unexposed.
- Exclude `/api/`, authenticated responses, and protected content from intermediary caches. The service worker caches only public application assets.

## Backup and restore

PostgreSQL contains processed images as `bytea`, along with profiles, sessions, maintenance, memberships, chat, albums, reports, role/badge audit, notifications and push state. A consistent `pg_dump` includes these together. Curated catalogue images ship with the application image. `MEDIA_DIR` is reserved for future storage and currently holds no uploaded originals.

Use private backup paths and encrypted off-host storage. Example from the Compose directory:

```sh
umask 077
docker compose exec -T db pg_dump -U bikerskz -d bikerskz -Fc > /private/backup/bikerskz.dump
```

Restore first into a **separate empty PostgreSQL instance**, using its container name:

```sh
docker exec -i RESTORE_TEST_DB pg_restore -U bikerskz -d bikerskz --exit-on-error < /private/backup/bikerskz.dump
```

Run the matching image's migration command, replay it, and verify readiness, retained sessions, private/public permissions, image byte hashes, selected covers, moderation audit, inbox/read state and push bindings. Only then plan a production restore. A restore creates a point-in-time copy of queued push jobs; keep push credentials disabled during rehearsal to avoid duplicate real deliveries.

Before upgrades, retain the previous image tag, a private database dump and the deployment environment. Additive migrations still make older executables fail their exact schema check. When schema generations differ, rolling back an executable alone is insufficient: stop writes, restore the matching database generation and previous image together, then verify health and access. An image rollback within the same schema generation may retain the current database after compatibility checks. Restore replaces later writes, so record the accepted recovery point before any real rollback.

## Monitoring and limits

Monitor `/health/live`, `/health/ready`, app/container restarts, private disk/WAL/backup capacity, database health, tunnel/proxy reachability, Telegram budget/delivery, and `push_dispatch_failed`. Container logs rotate. Do not log session tokens, OTP codes, phone numbers, push endpoints, key material or report evidence.

Image payload defaults are 100 MiB per original uploader and 1 GiB globally across bike photos, club images, avatars and ride albums, including reservations. Two shared processing slots and twelve reserved attempts per uploader per hour limit image work. A ride permits 24 photos (six per uploader) and twelve external video links (three per uploader). Images are normalized to fresh JPEG up to 1,920 pixels per edge and 4 MiB, with metadata discarded. These caps do not bound database indexes, WAL, text records or backups; monitor total storage and define retention before a public pilot.

Country, currency and UI language are independent preferences, defaulting to KZ/KZT/en. The city selector suggests localized Kazakhstan cities and also accepts manual entry without location permission. Distances/mileage use metric units. There is no commerce or exchange-rate behavior.

Phone-number changes, lost-Telegram recovery, account deletion/retention, administrator reauthentication policy, translation review, badge criteria and moderation staffing remain explicit release decisions. They do not add alternative login channels or deferred catalogue features to the MVP.

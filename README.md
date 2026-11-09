# BIKERS.KZ

An all-in-one motorcycle platform, starting in Kazakhstan with English, Russian, and Kazakh support.

The MVP launches as a mobile-first web/PWA using React, TypeScript, Vite, Go, and PostgreSQL. User signup and login use Telegram Gateway OTP only. Deployment is a Docker Compose stack on Jarvis behind the existing public VPS.

## Current implementation

The application implements Telegram OTP sign-in, profile onboarding, paginated ride discovery with city/date/description filters, My rides views for Organizing/Joined/Pending/Past, organizer-approved joining, participant-only ride details, and persistent Ride Room text chat with foreground updates. A shared city selector suggests Kazakhstan cities in English, Russian and Kazakh while allowing other cities. The UI supports English, Russian, and Kazakh. A PWA caches application assets; private/API data stays online-only.

Structured garages support multiple bikes, curated model/alias lookup with freeform entry and verified model-year reference photos, explicit public sharing, private maintenance logs, and processed owner-uploaded bike photos. Catalogue pictures remain the default until an owner explicitly chooses an uploaded cover; deleting that cover restores the catalogue. Photos follow the bike's visibility. Basic clubs support owner-approved membership, ownership transfer, closure, linked public ride previews, and one optional public club image. Club images remain public after closure; closed clubs reject image changes. Club membership never grants private ride access.

The public bikes catalogue at `/bikes` lets guests and members browse explicitly shared garage bikes, search by make/model, and page through results. Cards use the existing selected cover or verified model reference and link to the public bike page. Private and moderation-hidden bikes stay out of discovery; ownership identifiers and maintenance remain private.

The agreed MVP also includes profile avatars, participant-only ride albums and external video links, separate organizer announcements, a durable notification inbox for ride updates and club membership requests/decisions, opt-in Web Push, reporting, bilateral blocks, an administrator review queue, audited manual community badges, and independent country/language/currency settings. Telegram phone verification does not award a badge. Public pilot operations and real-device push acceptance remain separate from local software verification.

The community follow-up adds city/name club discovery and My clubs views for Organizing, Joined and Pending. Navigation shows unread inbox counts; push categories can be configured independently while the inbox retains every update. Approved riders and organizers can opt into 24-hour and 1-hour departure reminders and download a private calendar event. Calendar downloads are snapshots with stable event IDs, plan versions and cancellation status; they do not automatically synchronize later changes.

## Run with Docker Compose

Requirements: Docker with Compose, and a Telegram Gateway token for real signup/login. The example port is a development default; check availability before deployment.

```sh
test -f .env || cp .env.example .env
# Set POSTGRES_PASSWORD, TELEGRAM_GATEWAY_TOKEN, and the matching BASE_URL/APP_PORT.
make config
make build
make migrate
make up
```

Open the origin configured by `BASE_URL` (the example is `http://localhost:3042`). `make logs` follows application logs; `make down` stops services while preserving volumes. Migration is explicit. PostgreSQL has no published host port. The application runs as an unprivileged user with persistent database and media volumes.

The migration command applies versions 1–11 and verifies applied checksums. Versions 5–7 add avatars/ride media, moderation/admin state, and announcements/notifications/preferences; version 8 adds club membership notification targets; version 9 adds push category preferences and opted-in ride reminders; version 10 adds private ride map plans; version 11 adds useful places and moderated submissions. Previously applied SQL stays unchanged. Upgrade an existing database explicitly before expecting readiness. All uploaded images share bounded PostgreSQL `bytea` storage and processing limits, so database backups include them; `MEDIA_DIR` remains reserved for future media.

See [Operations](docs/operations.md) for audited administrator bootstrap, optional VAPID configuration, backup/restore and rollback. Users opt into push in Settings; the inbox works without push credentials or permissions.

The application is live at [bikers.kz](https://bikers.kz), on Jarvis port 3050 behind the VPS's verified HTTPS route. The existing account/admin data was restored and PostgreSQL remains private. The app serves plain HTTP; `BASE_URL=https://bikers.kz` governs public-origin and cookie checks. The owner signed in, confirmed administrator navigation and enabled browser notifications; the provider accepted one diagnostic push. The owner confirmed that the notification appeared on this Mac and clicking it opened the inbox. See [current deployment](docs/operations.md#current-deployment) for the durable local/homelab configuration and operating commands.

The current design refresh keeps the Night Ride charcoal and lime identity with a locally bundled Geist typeface, Phosphor icons, grouped navigation and consistent forms, cards and collection tabs. Ride discovery has one primary action and responsive city/date filters. The shared font covers English, Russian and Kazakh. This source refresh is separate from the deployed application; rollout evidence belongs in Operations.

The [design review](docs/design-review.md) records the Unslop usability polish and a Stageshow ride-flow illustration. Run `npm run prototype` in `apps/web` and open `http://localhost:5173/preview/ride-flow` to play the localized example or inspect its static reduced-motion view. The preview is development-only and contains authored example content.

## Deploy from Git on Jarvis

Push reviewed changes to `main`, then run on Jarvis:

```sh
cd /home/user/apps/bikerskz/repo
git pull --ff-only
make deploy-jarvis
```

The deployment builds the clean checkout on Jarvis, tags the image with the full
Git commit and records the revision in an OCI label. It replaces only the app;
the existing private `/home/user/apps/bikerskz/.env`, database and media volumes
remain in place. App-only activation failure restores the previous app image. The schema-8 to schema-9 upgrade uses
`make deploy-jarvis DEPLOY_ARGS='--migrate-to 9'`; after a committed upgrade, recovery rolls forward with schema-9 code. See [repository deployments](docs/operations.md#repository-deployments-on-jarvis)
for initial cloning, verification and rollback. Production images are built on
Jarvis rather than built locally and copied there.

## GitHub Actions

Pull requests and main pushes run hosted frontend, backend, PostgreSQL integration and deployment-tooling checks. Successful main CI dispatches the private deployment repository, whose isolated homelab runner requests a validated release on Jarvis. See [GitHub Actions deployments](docs/github-actions-deployment.md) for credentials, runner boundaries, pause/retry and recovery.

## Development checks

Use Node 26 and Go 1.27.3 (the image versions are pinned in the Dockerfile).

```sh
cd apps/web
npm ci
cd ../..
make check-web
make check-backend
TEST_DATABASE_URL='postgres://user:password@127.0.0.1:5432/test_db?sslmode=disable' make check-integration
```

Use a disposable test database. Integration tests isolate their schemas and exercise HTTP behavior against PostgreSQL. Test fakes are compiled only into Go tests; the production application has no fake login mode.

Backend checks also enforce the package ownership and dependency rules in `AGENTS.md`. See the [backend architecture guide](docs/backend-architecture.md) for the package map and transaction contracts.

## Documentation

Start with the [documentation index](docs/README.md) or [Project Home](docs/project-home.md). The repository documentation is maintained alongside the native Space pages. The technical design and build order remain draft recommendations where marked.

See the [development and API guide](docs/development.md) for permission boundaries, sessions, chat ordering, and the current release boundary.

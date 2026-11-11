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
[schema-11 places mode](#useful-places-schema-and-data) coordinates its schema cutover. It verifies retained environment,
ports and media mounts, an unchanged database container/start time, and a
healthy non-root/read-only app. App-only failures, or migration failures with
confirmed unchanged schema 10, restore the previous environment bytes and app
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

The completed club-notification release upgraded schema 7 to 8. The subsequent
community follow-up upgraded schema 8 to 9, adding preferences, reminders and
calendar behavior while preserving all prior data. These transitions are
historical; the current deployment helper supports only the reviewed 10-to-11
places transition described below. It never silently applies an older or future
migration.

## Configure and start

Keep `.env` private and outside version control. Set `BASE_URL` to the exact browser origin, `APP_PORT` to a free homelab port, a unique URL-safe `POSTGRES_PASSWORD`, and `TELEGRAM_GATEWAY_TOKEN`. Do not enable fake authentication in the deployed executable; test authentication exists only in Go test binaries.

```sh
make config
make build
make migrate
make up
```

`make up` and `make migrate` create or validate the internal `bikerskz_gis`
network used by the app. This does not start GIS services or enable map APIs.

Migrations are explicit. Version 5 adds avatars and ride media, version 6 adds moderation, blocks, roles and badges, version 7 adds preferences, announcements, inboxes and push delivery, version 8 adds club membership notification targets, version 9 adds push categories and explicit ride reminders, version 10 adds nullable typed ride map plans, and version 11 adds useful places and moderated submissions. All eleven receipts and their checksums are checked by readiness. Previously applied SQL is immutable. Unknown versions and mismatched checksums fail closed.

## GIS serving and data refresh

The GIS rollout uses the completed [Kazakhstan component benchmark](osm-kazakhstan-benchmark.md).
The following commands are the reviewed deployment procedure, not evidence that
the rollout has already happened. First acceptance must include simultaneous
map/search/route requests and public/browser checks; the benchmark did not test
combined production traffic or certify motorcycle route safety.

Keep GIS state under `/home/user/apps/bikerskz-gis`, outside the checkout. The
separate Compose project is `bkgis`, configured by
[`deploy/gis/compose.yml`](../deploy/gis/compose.yml). Its private internal network
is `bikerskz_gis`; the app joins it alongside its existing default network.
Photon and Valhalla publish no host ports. Only the static Nginx service uses
the separate `bkgis_ingress` bridge and binds **`10.0.0.2:3052`**, reachable by
the existing VPS over WireGuard. Port 3051 belongs to another application.

| Service | Hard memory / CPU limit | Persistent access |
| --- | --- | --- |
| Valhalla 3.9.1, one service worker | 1 GiB / 1.5 CPU | Versioned graph read-only |
| Photon 1.3.0, Java heap 512 MiB–2 GiB | 3 GiB / 2 CPU | Its selected index writable; JRE/JAR read-only |
| Nginx static maps | 128 MiB / 0.5 CPU | Public map/glyph/sprite tree read-only |

All three use UID/GID 1000, read-only root filesystems, dropped capabilities,
no privilege escalation, bounded temporary storage/processes, and rotating
logs. The read allowance is 5 MiB/s per service on `/dev/sda`; Photon also has
a 5 MiB/s write allowance. Memory plus swap is capped at the same amount.
**Valhalla's 1 GiB serving cap is a new tuning choice:** the component queries
used a 3 GiB cap. Validate it under the guarded combined pilot; any increase
requires a reviewed Compose change and new headroom check. The 4.125 GiB sum
of hard caps does not establish a measured combined working set.

### Adopt completed data and start without another import

The first bulk-copy setup attempt failed the strict application-readiness guard
and was stopped. The app recovered, original benchmark data stayed intact,
and that job's partial copy/receipts remain for review. The bulk-copy command
has been removed. Initial setup now uses same-filesystem atomic adoption.

Pause automatic deployment using **both controls** in
[GitHub Actions deployments](github-actions-deployment.md#pause-retry-and-recovery)
before changing the app's runtime configuration or schema. Use the clean pushed
`main` checkout on Jarvis. The pinned third-party images must already be local;
GIS stages and activation forbid implicit pulls.

```sh
cd /home/user/apps/bikerskz/repo
python3 scripts/gis.py adopt \
  --benchmark /home/user/benchmarks/bikers-osm-kz-20261105-16 \
  --id kz-20261105
python3 scripts/gis.py activate
python3 scripts/gis.py verify
```

`verify` is the combined serving acceptance step before enabling the core app
integration. It discovers the running private container IPs from Docker and
requires the actual **1 GiB Valhalla memory/swap cap**. It sends 96 fixed mixed
requests: 32 road-motorcycle routes in OSRM/GeoJSON format without directions,
32 Kazakh/Russian/English Almaty/Astana searches with Kazakhstan filtering,
and 32 map-header/random 64 KiB range checks against the selected local archive.
Concurrency is bounded at one route, two searches and two map requests, five
workers total. Each request has at most an eight-second timeout; the query
window is 85 seconds, its worker is forcibly bounded at 90 seconds, and an
independent 120-second lease covers the guarded phase. Preflight and cleanup
add wall time. This is a small mixed pilot, not a stress or capacity test.

Every response must have the expected type, coordinates/country, plausible
fixture distance, and exact range status/length/Content-Range/local bytes.
Any provider/guard error stops the three GIS services. The private
`/home/user/apps/bikerskz-gis/verification.json` records per-provider latencies,
counts/errors, before/after cgroup memory/CPU/I/O, container identity/OOM checks,
and references to the core-app guard receipts. Cgroup peaks include file cache
and earlier activity; no caches are evicted. After publishing a future map
candidate, explicitly select its version for this check with
`python3 scripts/gis.py verify --archive-url /map-data/CONTENT_HASH/kazakhstan.pmtiles`;
file sizes and range checks come from that archive, rather than initial benchmark
metadata. The default uses the GIS selected URL or its adoption receipt.

Adoption validates completed benchmark receipts, original cgroup exit counters,
source ownership/device and every source tree entry. It refuses running
benchmark-labelled or GIS containers and rechecks quiescence before and after
the moves. A read-only, I/O-capped preparation job verifies the immutable
archive/JAR hashes and prepares only the small font/sprite/license tree. To
reuse verified downloads from the retained failed job, add
`--assets-from /absolute/retained-job/inputs`; size/SHA256 checks still run.
Without that option, acquisition is serial with pinned URLs/hashes, size/time
limits and curl's 3 MiB/s cap. Extraction rejects traversal/symlinks and resolves
validated tar hardlinks into regular glyph files. New assets are flushed inside
the capped job before publication.

Completed `data/valhalla3`, `data/photon-stream4`, the PMTiles file, JRE, Photon
JAR and PMTiles CLI are then **renamed**, not bulk-copied, into the new release
on the same filesystem. No hardlink aliases into the writable Photon index are
created. Original cgroup-counter bytes stay available at the old paths, with
small `PROMOTED.json` references in the old data/runtime locations and parents
of moved files. Those references contain paths, not symlinks to mutable live
data. The benchmark's logs, receipts and retained failure evidence remain.

Each rename has an fsynced intent/identity record under
`/home/user/apps/bikerskz-gis/adoptions/<id>.json`. Source/target directory
entries are flushed around the rename. This includes the final release,
promotion references and the last publication of `adoption.json` and `.env`.
The environment appears only after the full strict guard succeeds. Caught
failures reverse completed or interrupted renames without overwriting existing
data. A launcher/power interruption leaves a recoverable journal; with source
and GIS containers stopped, restore the original paths using:

```sh
python3 scripts/gis.py recover-adoption --id kz-20261105
```

Recovery reconciles source/target identities when a rename completed before its
journal update and is idempotent after rollback. It refuses conflicting data
and committed journals. After rollback, use a new adoption ID for a retry.
Activation and later GIS mutations require a committed adoption journal, so a
partly published environment cannot start Photon before recovery. No PBF map
generation, graph build, search import or deletion of original data is part of
adoption. Retained partial-copy jobs are never removed automatically.

The private GIS `.env` contains only explicit selected release paths.
`releases/<id>/public/map-data/2583113c45ca/kazakhstan.pmtiles` is the initial
archive; fonts are under `map-data/fonts/noto_sans_regular`, matching sprites
under `map-data/sprites/base*`, and notices under `map-data/licenses`.
Changing a host symlink alone does not retarget an existing Docker bind mount.

### Guard and cleanup semantics

Every adoption/recovery/build/start/switch/restart runs under the GIS `.refresh.lock` and
the existing application's `.deploy.lock`, so imports cannot overlap each
other or an app build/release. A fresh job requires twelve app readiness
responses within 250 ms, at least **8 GiB available RAM** and **25 GiB free
filesystem space**. The running guard aborts immediately on a failed/slow
readiness probe or an original container identity/health regression; it also
stops after sustained RAM below 3 GiB, free disk below 20 GiB, swap growth over
256 MiB, unavailable telemetry or a 12 GiB job-directory budget breach.

Watchdog/lease records and samples use tmpfs during work, then are retained
privately under `jobs/bikers-osm-kz-<id>`. The independent lease detects a dead
launcher and stops only the job's labelled build containers. Failed initial
activation stops the three GIS services; a failed component cutover stops
only its named GIS service and restores the previous selected path. It does
not restart a heavy service while the main application is regressing. After
recovery, retry the selected dataset with `python3 scripts/gis.py restart
--component photon` (or `valhalla`/`maps`) under a new guard.

There is no continuous global watchdog, automatic import schedule or rebuild
on service startup. Per-job guards end before the separate schema/app cutover;
normal serving uses Docker healthchecks and resource caps. Those guards must
not mistake an authorized app replacement or migration pause for interference.
App readiness remains independent of optional GIS availability.

### Public map prefix and app schema/configuration

The existing VPS `/etc/nginx/sites-available/bikers.kz` keeps its normal app
route to `10.0.0.2:3050`. Add only the candidate
[`hoster-location.conf`](../deploy/gis/hoster-location.conf) inside its HTTPS
server, exposing `/map-data/` through `10.0.0.2:3052`. This preserves Range,
If-Range and validators and leaves API caching disabled. No new domain or
certificate is required. The static service serves only the explicit archive,
glyph, sprite and license paths; it exposes neither Photon nor OpenSearch.

Before the edge write, record all nginx configuration fingerprints, preserve
the original site bytes privately, and compare the intended single-file diff.
The 5 November read-only baseline site SHA256 was
`a1c994d93ad57df5df5bf94f6a0d17e875367a8ea35567771b99d9829a6158a3`;
recheck it immediately before applying. Run `nginx -t` before reload; restore
the original site if validation fails. After reload, verify every unrelated
configuration fingerprint, app readiness and HTTPS map/glyph/sprite requests.
Require exact HTTP 206/Content-Range/bytes for sample archive ranges, including
a later offset; verify 416 and HEAD/validators as well. Recheck that Jarvis port
3052 remains free before initial activation; the helper refuses a listener.

Set only the three reviewed additions in the private app environment:

```dotenv
PHOTON_BASE_URL=http://photon:2322
VALHALLA_BASE_URL=http://valhalla:8002
MAPS_ARCHIVE_URL=/map-data/2583113c45ca/kazakhstan.pmtiles
```

The initial schema-9 to schema-10 GIS cutover completed on 5 November with the
reviewed source at `13774c43f276f74a964e08a422e1069f0d24159e`. Its exact receipts,
retained-state parity and mixed-service/browser acceptance are recorded in
[#18](https://github.com/unstppbl/bikers/issues/18). Current source no longer
offers that historical migration mode; the supported upgrade is schema 10 to
11 below. An archive URL change at the current schema still uses the explicit
`--configure-gis` flag after both automatic-deployment controls are paused.

The GIS flag permits only the two exact private service URLs, a versioned
same-origin `/map-data/<hash>/kazakhstan.pmtiles` path, and addition of the
`bikerskz_gis` network. Every other environment value, port, mount, network,
database identity/start and non-root/read-only condition stays checked. Without
the flag, configuration changes remain forbidden. The automatic gate still
refuses Compose/schema changes. Install the reviewed `deploy-tested-main.py`
and `deploy-jarvis.py` together in the host's existing `bin` directory after
this rollout; an older helper cannot validate a newer source migration.
Verify main CI, private deployment status, live revision/receipts and public
readiness before resuming both automatic-deployment controls.

### Explicit serial refresh and rollback

Acquire a new country-scoped input with a recorded timestamp and checksum;
the helper never follows a mutable `latest` URL automatically. Build one
component at a time from a local, bounded, explicitly supplied source:

```sh
python3 scripts/gis.py refresh --component photon \
  --source /private/path/new-photon.jsonl.zst --sha256 FULL_INPUT_SHA256 \
  --id photon-YYYYMMDD
python3 scripts/gis.py switch --component photon \
  --candidate /home/user/apps/bikerskz-gis/releases/photon-YYYYMMDD
```

Use `valhalla` for a country PBF or `maps` for prebuilt Shortbread MBTiles. All
build containers are offline with no restart policy, at most 2 CPU/5 GiB RAM,
5/5 MiB/s I/O and explicit timeboxes. Photon streams at most 16 GiB expanded
bytes, filters Kazakhstan, and requires verified stream EOF plus successful
import completion. PMTiles conversion uses disk-backed temporary space, verifies
the archive and records its digest. A successful build publishes a new candidate
by directory rename; it never overwrites the active dataset.

Switching Photon/Valhalla atomically updates the selected path and recreates
only that component, with a short feature outage and retained prior data.
There is no claim of a zero-downtime blue/green cutover. A map switch publishes
an immutable content-hash directory first and prints its new URL; activate that
URL through a separately reviewed app `--configure-gis` rollout. Retain old map
versions for existing clients: reusing a map URL for different bytes can mix
PMTiles directories and tile ranges. Never delete the previous index/graph or
archive before successful acceptance and the agreed retention period.

Keep at least 10 GiB of data working headroom plus images/backups. If the
preflight fails, recover capacity or schedule the build elsewhere; do not
weaken thresholds or allocate the host's unused LVM space implicitly. Retain
the original readiness-impact incident and every later abort receipt when
reporting acceptance; a healthy final check does not erase earlier impact.

## Useful places schema and data

Current population policy is **Map / Карта**, curated through reviewed user
contributions ([#24](https://github.com/unstppbl/bikers/issues/24)). The initial
OSM place seed is withdrawn. Fresh directories start empty; do not run place
extraction/import to repopulate them without a new owner instruction. The
self-hosted OSM basemap, address search and routing are separate and remain in
use. The schema rollout and initial seed procedure below record the historical
#22 release; retain its dataset and receipts as recovery evidence.

[#22](https://github.com/unstppbl/bikers/issues/22) adds public useful-place
discovery and private moderated submissions. Schema 11 creates only `places`
and `place_submissions`; existing SQL migrations are immutable. Suggestions
are not published until an active administrator approves them. Approval checks
the current source version, administrator role and submitter status under the
repository locks. Public responses never include pending proposals or review
identities. Imported OSM entries retain their source IDs/date and are not
community verification; later OSM imports preserve community corrections and
closures.

For the existing schema-10 production database, pause **both** automatic release
controls, preserve a private `.env`/database/image backup, and validate the dump
before this explicit clean-main rollout:

```sh
make deploy-jarvis DEPLOY_ARGS='--migrate-to 11'
```

The helper requires exact source-matching receipts 1–10 before building. It
builds before stopping only the app, then fingerprints every retained table
row and sequence, **including existing ride map plans**. Only the two new tables
and receipt 11 are excluded from parity, and the new tables must initially be
empty. A confirmed unchanged schema 10 permits old-image recovery; a committed
or unknown schema-11 outcome keeps the target selected for explicit roll-forward
recovery. Do not restore a schema-10 binary against schema 11.

Install `deploy-tested-main.py` and `deploy-jarvis.py` together in the existing
host `bin` directory after the reviewed rollout. Verify public readiness,
retained data/container state, main CI and the private deployment check before
resuming both automatic controls. No GIS environment, network, edge or service
restart is needed for this feature.

The historical initial country data was extracted from the retained verified PBF, using pinned
Python wheels in the existing parser image. The explicit `extract-places` and
`import-places` commands share the existing serial locks, health guard and
independent lease. They use at most 512 MiB/1 CPU and 5 MiB/s disk I/O. No PBF is
copied, no new image is pulled, and no public Overpass/geocoding service is
bulk-queried. Extraction is offline after the small verified wheels are staged.
See [Places data](places-data.md) for the pins, category mapping and invocation.

The retained artifact is `places-datasets/<id>/places.ndjson`, with its digest,
record count and source date in `dataset.json`. Import validates the whole
bounded file before one transaction. A successful guarded import records
`last-import.json`; failures retain their private job receipts and can be
retried with the same artifact. The one-off worker uses the exact healthy live
app image, private environment and database network. Its temporary environment
file and stopped import container are removed on normal/lease cleanup. Preserve
the job directory as private if host Docker failures prevent full cleanup.

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

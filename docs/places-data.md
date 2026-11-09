# Map data and the historical Kazakhstan OSM seed

## Current policy: user-curated places

[Issue #24](https://github.com/unstppbl/bikers/issues/24) supersedes the initial
OSM seed on 9 November 2026. The public feature is **Map / Карта**, immediately
after Motorcycles in navigation. Its place directory starts empty: riders
suggest places and administrators review additions, corrections and closures.
There is no scheduled import or automatic seed on startup. Do not repopulate
the directory from OSM or run a bulk import without a new owner instruction.

The self-hosted OSM basemap, address search and motorcycle routing remain in
use. Their map data is separate from the community's place entries. Existing
SQL migration files and source/provenance fields remain unchanged.

The extraction and import details below are historical reproduction records
for #22, not current production population instructions. Keep the retained
dataset and private reset backup for recovery and audit; record execution
evidence and the reset's count/hash checks in #24.

## Historical initial seed

Initial useful places came from the already acquired Kazakhstan OpenStreetMap
extract. No demonstration businesses, bulk public Overpass queries, inferred
meeting spots or user-content exports are part of this process. Extraction and
database import are separate, explicit guarded operations.

## Source and parser provenance

Read-only Jarvis inventory on 9 November 2026 found the retained source at:

```text
/home/user/apps/bikerskz-gis/releases/kz-20261105/valhalla/kazakhstan-261103.osm.pbf
```

The original acquisition receipt is
`/home/user/benchmarks/bikers-osm-kz-20261105-16/results/input-kazakhstan-261103.osm.pbf.json`:

| Field | Recorded value |
| --- | --- |
| Source | [Geofabrik dated Kazakhstan extract](https://download.geofabrik.de/asia/kazakhstan-261103.osm.pbf) |
| Bytes | 225,713,312 |
| SHA256 | `f93c948612fda139ee372165cbe09b0e980f64065cd5e8646985fa0645edfb3a` |
| MD5 | `53403ba99b12bcdf5183316539ef2440` |
| Snapshot timestamp | `2026-11-03T21:24:23Z` |

The Valhalla `file_hashes.txt` sidecar is a scripted-build checksum, **not** the
PBF acquisition checksum. Do not substitute it. The extractor rechecks the PBF
SHA256 inside its capped worker, captures source identity before hashing, and
compares device/inode/size/mtime/ctime after hashing and after both passes.
`source_date` comes from the PBF header's replication/snapshot timestamp;
`--source-date` asserts that date rather than replacing it with today's date.

The pinned Valhalla image already used by GIS has Python 3.12, but its live
inventory contained neither osmium CLI, pyosmium nor pip. The helper therefore
stages a small **pyosmium 4.3.2** wheel and its required pure-Python dependencies
in a private job directory. It uses no host package installation, new runtime
image, compiler or implicit image pull. The six pinned wheels total
**2,358,983 bytes**; exact URLs/sizes/SHA256 values are owned by
[`scripts/places_osm.py`](../scripts/places_osm.py)'s `wheel_manifest()`.
Pins were read from official PyPI metadata for [osmium 4.3.2](https://pypi.org/pypi/osmium/4.3.2/json),
[requests 2.34.2](https://pypi.org/pypi/requests/2.34.2/json),
[urllib3 2.8.0](https://pypi.org/pypi/urllib3/2.8.0/json),
[idna 3.20](https://pypi.org/pypi/idna/3.20/json),
[charset-normalizer 3.5.2](https://pypi.org/pypi/charset-normalizer/3.5.2/json)
and [certifi 2026.10.23](https://pypi.org/pypi/certifi/2026.10.23/json).

The wheel targets Linux/amd64 CPython 3.12 and manylinux glibc 2.27/2.28. A
bounded native probe and real-PBF fixture test must pass on Jarvis before the
country scan; the filename alone is not a benchmark of this older CPU. Wheel
extraction verifies every pin and rejects traversal, symlinks, special files,
duplicates and expanded-runtime budgets above 64 MiB. The resulting package
tree is supplied through `PYTHONPATH`, with bytecode writing disabled.

## Selection policy and output

An object needs a real OSM name, localized name, brand or operator tag. Names
are never invented for otherwise unnamed businesses or parking areas. All
categories reject explicit private/no/resident/permit-only access, prohibited
motorcycle/motor-vehicle access, conditional access, driveways/parking aisles,
and explicitly inactive lifecycle tags.

| Category | Exact selection |
| --- | --- |
| `fuel` | `amenity=fuel` |
| `workshop` | `shop=motorcycle_repair`, or `shop=car_repair` with `motorcycle=yes` or `service:motorcycle:repair=yes` |
| `motorcycle_shop` | `shop=motorcycle`; a repair service tag does not rename the shop into a dedicated repair business |
| `tires` | `shop=tyres`; no motorcycle stock or service availability is inferred |
| `parking` | `amenity=parking` with explicit public/customer access or `motorcycle=yes/designated`; residential/private garage parking and parking restrictions are excluded |
| `meeting_spot` | `amenity=meeting_point` only; community proposals follow the platform's separate moderation path |

This deliberately conservative parking rule omits lots with unspecified
access. General public/customer parking is not advertised as motorcycle-only.
OSM tags are source observations, not business verification or current opening
confirmation. Relevant primary tag documentation is [motorcycle shops](https://wiki.openstreetmap.org/wiki/Tag:shop%3Dmotorcycle),
[tyre shops](https://wiki.openstreetmap.org/wiki/Tag:shop%3Dtyres),
[parking](https://wiki.openstreetmap.org/wiki/Tag:amenity%3Dparking)
and [access restrictions](https://wiki.openstreetmap.org/wiki/Key:access).

Nodes retain their coordinates. Ways use a closed-ring area centroid when it
falls inside the polygon, otherwise a point on its longest boundary segment;
open ways use their length midpoint. These positions are representative,
**not verified entrances**. Incomplete references, degenerate/out-of-country
geometry and shapes spanning more than 0.25 degrees are skipped. Relations
and multipolygons are outside the initial export scope. Shared physical places
represented by both a node and a way retain separate OSM IDs; no speculative
business deduplication changes their provenance.

Each nonblank NDJSON line contains exactly:

```text
source_ref, source_date, name, category, lat, lon,
city, address, description, website, phone, opening_hours
```

`source_ref` remains `node/<id>` or `way/<id>`; the backend derives its public
OpenStreetMap link. Optional city/address/contact/description/opening-hour
values come only from source tags and may be empty. Text is bounded by UTF-8
bytes; website values must be valid HTTP(S) URLs without credentials or malformed
escapes. No verification, availability or publication-status field is supplied
by the extractor. Preserve `© OpenStreetMap contributors`, source IDs and the
ODbL notice alongside the imported data; see [OSM copyright and licence](https://www.openstreetmap.org/copyright).

## Historical guarded extraction and import

The initial seed used the clean pushed `main` checkout and the approved schema-11 release. The
existing GIS wrapper owns the deployment/refresh locks, independent lease,
readiness checks, bounded acquisition and labelled containers. Do not run a
country scan directly in the live Valhalla service.

```sh
cd /home/user/apps/bikerskz/repo
python3 scripts/gis.py extract-places \
  --source /home/user/apps/bikerskz-gis/releases/kz-20261105/valhalla/kazakhstan-261103.osm.pbf \
  --sha256 f93c948612fda139ee372165cbe09b0e980f64065cd5e8646985fa0645edfb3a \
  --source-date 2026-11-03 \
  --id kz-20261103-v1
```

The job serially fetches the pinned wheels, safely prepares their package tree,
probes native parsing, runs the real-PBF fixture tests, then exports offline
with the PBF mounted read-only. Country extraction is capped at **512 MiB,
one CPU and 5 MiB/s read/write**. Pyosmium's explicit one-thread/two-buffer pool
avoids unbounded native prefetch. It scans candidate ways first, then only
retains the node coordinates those ways need while also emitting eligible
standalone nodes. It does not allocate a country-wide location cache or a flat
node-index file. [Pyosmium reader/thread-pool API](https://docs.osmcode.org/pyosmium/latest/reference/IO/)
and [geometry/location-storage documentation](https://docs.osmcode.org/pyosmium/latest/user_manual/03-Working-with-Geometries/)
explain these choices.

The enforced budgets are a 300 MB input, 50,000 candidate ways, 2,000 nodes per
way, two million references, 500,000 retained node IDs, 32 MiB of way metadata,
and **50,000 output rows / 32 MiB**. Oversized individual ways are skipped;
exceeding a global state/output limit fails the
whole export rather than publishing a truncated dataset. These are safety
bounds, not measured minimum resource requirements for every future extract.
The completed NDJSON is published atomically with mode **0644** so the
UID-10001 application importer can read its direct read-only bind mount;
the surrounding host job directories remain private.

After the final guard succeeds, the wrapper retains the dataset and checksum
receipt under `/home/user/apps/bikerskz-gis/places-datasets/kz-20261103-v1`.
Review category/count/skip summaries and sample source links before import.
The completed artifact's checksum and source date are retained with the job.
The separate import command is:

```sh
python3 scripts/gis.py import-places \
  --dataset /home/user/apps/bikerskz-gis/places-datasets/kz-20261103-v1
```

It uses only the healthy receipt-matching live application image and private
environment, and runs `api places import --file ... --sha256 ...` as a bounded
one-off worker. It does not start another web service, run migrations, send
messages or change the app image. Temporary environment copies and the stopped
worker's Docker environment are cleaned up; private job/result receipts remain.
The worker's I/O cap does not cap PostgreSQL's own server-side writes, so retain
the strict application/database guard and reconcile transaction outcome after
an abort before deciding whether to retry.

The backend validates the complete file/checksum before its atomic database
transaction. Stable source IDs permit repeat imports; community-curated and
hidden records are preserved, with original OSM provenance retained. Missing
objects in a later extract are not automatically deleted. Import success is
not proof of address completeness, road access, business status or motorcycle
service availability. Production execution/acceptance is recorded by the
operator separately; the commands above are not a deployment receipt.

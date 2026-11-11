# Kazakhstan OSM benchmark on Jarvis

**Completed component benchmark — 5 November 2026.** Kazakhstan routing, search and map-file delivery succeeded within the stated limits, and final cleanup/readiness checks passed. **The no-impact criterion was not met:** initial shared-SSD work degraded application readiness. Combined production load, browser rendering and route-safety acceptance were outside this test; no production deployment occurred.

The benchmark used `/home/user/benchmarks/bikers-osm-kz-20261105-16`, separate data directories and labelled containers. Raw host receipts remain in that private task directory. The [machine-readable results](osm-kazakhstan-benchmark-results.json) publish measurements and input/harness provenance without the inventory of unrelated services. The [harness README](../scripts/osm-benchmark/README.md) documents interfaces, guards, lifecycle and interpretation. Original map-provider research is tracked separately in [issue #14](https://github.com/unstppbl/bikers/issues/14) and [PR #15](https://github.com/unstppbl/bikers/pull/15).

| Component | Completed evidence | Acceptance state |
| --- | --- | --- |
| Valhalla | Successful build and 33 valid namespace-local queries | Measured for the stated fixtures/limits |
| Photon | Fourth fresh import completed; 216 valid namespace-local queries; three earlier failures retained | Measured for the stated fixtures/limits |
| Map archives | Successful conversion, archive verification and 256 exact HTTP Range responses | Archive/delivery test passed; browser rendering untested |
| Host cleanup and final readiness | Original 41 containers unchanged; benchmark processes stopped; local/public readiness passed | Final checks passed; initial no-impact criterion failed |

## Host and scope

Jarvis is an Intel i7-3615QM Mac mini with four physical cores/eight threads and approximately 15.5 GiB RAM, running Linux x86_64. Existing workloads share its SSD, CPU and memory. These measurements concern a small country-scoped evaluation on this host, not a world dataset or a capacity guarantee for a particular user count.

The evaluated services are a Kazakhstan Valhalla motorcycle graph, a Kazakhstan-filtered Photon index with Kazakh/Russian/English names, and locally served PMTiles. Follow-up import/build stages ran sequentially and offline after bounded acquisition of pinned inputs/images. No application release is included.

## Valhalla: completed isolated build

The successful `valhalla3` attempt ran on **5 November, 14:10:57.451–14:19:37.602 Asia/Almaty (UTC+5)**. Recorded container timestamps are `2026-11-05T09:10:57.450535009Z` and `2026-11-05T09:19:37.601980064Z`.

| Measure | Observed value |
| --- | ---: |
| Container elapsed time | **520.151 seconds**, about 8 min 40 sec |
| Exit status / OOM kill | **0 / false** |
| CPU cap | **1.5 CPU** |
| I/O caps | **20 MiB/s reads, 10 MiB/s writes** |
| Final cgroup memory peak | **1,761,128,448 bytes ≈ 1.640 GiB** |
| Final cgroup CPU time | **257.148064 seconds**, about 0.49 CPU averaged across container elapsed time |
| Physical SSD `8:0` reads | **1,402,122,240 bytes ≈ 1.306 GiB** |
| Physical SSD `8:0` writes | **4,519,817,216 bytes ≈ 4.209 GiB** |
| Final graph tiles | **5,231** |
| Retained `data/valhalla3` directory | **756,252,790 bytes ≈ 721.219 MiB** |

The retained directory includes the **225,713,312-byte** PBF, **265,021,440-byte** graph archive, approximately 255 MB of unpacked tiles, and a **9,895,936-byte** administrative database. Retained files and physical I/O are different measurements; temporary build activity explains why write volume exceeds final disk footprint. The memory figure is **cgroup charged memory including file cache**, not process RSS.

The input is Geofabrik's `kazakhstan-261103.osm.pbf`, with OSM data timestamp `2026-11-03T21:24:23Z` and MD5 `53403ba99b12bcdf5183316539ef2440`. The pinned Valhalla image/configuration are recorded by the [adapter manifest](../scripts/osm-benchmark/valhalla.py). This scope does not validate live traffic, elevation, transit or timezone behavior. [Geofabrik Kazakhstan source](https://download.geofabrik.de/asia/kazakhstan.html)

### Motorcycle queries

All **33 valid namespace-local requests succeeded** against `127.0.0.1:8002` inside the benchmark container. Fixtures covered an Almaty city route, Talgar–Esik rural route and Almaty–Astana long route, returning approximately **4.498 km**, **24.022 km** and **1,227.275 km**, respectively.

| Phase | Requests / successful | Client concurrency | p50 | p95 |
| --- | ---: | ---: | ---: | ---: |
| First observed request per fixture | 3 / 3 | 1 | **137.156 ms** | **198.233 ms** |
| Repeated fixtures | 15 / 15 | 1 | **28.338 ms** | **104.200 ms** |
| Repeated fixtures | 15 / 15 | 2 | **75.339 ms** | **101.764 ms** |

These are a small repeated-fixture latency sample, not a throughput benchmark. No global caches were flushed; first observed requests are not cold-cache measurements. HTTP success and plausible distances do not certify road suitability, route correctness or riding safety.

An earlier **33-request host-port attempt had 33 connection refusals**. Docker's internal network left published host bindings absent (`Ports: null`). Those requests are **excluded** from the table; they test an unavailable network path, not Valhalla route latency. Valid measurements use container-namespace loopback via `docker exec`.

## Observed interference and subsequent controls

**The benchmark affected application readiness.** Shared SSD preparation/import I/O coincided with a roughly **1.23-second** readiness response followed by **five probes reaching their approximately two-second timeout**. I/O `full avg10` reached **60.91%**. No benchmark container was OOM-killed, and available host memory remained above 9 GiB, supporting I/O contention as the explanation rather than memory exhaustion.

The guard stopped benchmark work. The original **41 container IDs, start times and restart counts remained unchanged**, and application readiness recovered to approximately **2–5 ms without a restart**. These facts establish recovery and retained container identity; they do not erase the observed latency impact or prove that every user-facing request was unaffected.

Two early monitor aborts also recorded stale disk-backed heartbeats, separately from the readiness incident. Follow-up work was serial with a **5 GiB import cap** and a strict **250 ms readiness limit with immediate abort**. Initial follow-up I/O limits were **5/2 MiB/s read/write**; the completed fresh Photon import used **2.0 CPU and 5/5 MiB/s**, as described below. Safety heartbeat/control records and samples moved to tmpfs symlinks, while slow Docker telemetry became asynchronous. An independent systemd lease and stop hook covered watchdog failure/expiry. Keep the different attempts' actual limits attached to their measurements.

## Photon: completed streamed import

The successful attempt streamed the verified [Central Asia Photon snapshot](https://download1.graphhopper.com/public/asia/central-asia/photon-dump-central-asia-1.0-latest.jsonl.zst) through a bounded decoder into Java stdin, importing only Kazakhstan with `kk,ru,en` names. It uses the [Photon 1.3.0 import interface](https://github.com/komoot/photon/blob/1.3.0/docs/usage.md), without a permanent Nominatim service. The materialized JSON intermediate was discarded to avoid an approximately 2 GB write. An extracted, read-only Java 21 runtime was mounted into the Ubuntu 24 Valhalla image; no host JRE installation was part of this test.

| Streamed import attempt | CPU cap | Read/write caps | Observed status |
| --- | ---: | ---: | --- |
| First attempt | 0.75 CPU | 5/2 MiB/s | Failed at the stock 30-second bulk timeout; no OOM |
| Second attempt | 2.0 CPU | 5/2 MiB/s | Failed at the same timeout around 450,000 records after 805.27 seconds; no OOM or readiness-guard regression |
| Third fresh stock attempt | 2.0 CPU | 5/5 MiB/s | Failed at the decoder's 4 GiB expanded-source bound after reporting 639,820 documents; incomplete index, no query test |
| Fourth fresh stock attempt | 2.0 CPU | 5/5 MiB/s | Completed successfully in 1,513.809345 seconds with a 16 GiB expanded-stream budget; exit 0, no OOM |

All four attempts used a **5 GiB memory cap** and the strict **250 ms readiness guard**. The second attempt's final cgroup memory peak was **2,494,255,104 bytes**; its 450,000-record progress is not a completed-index count. The write allowance changed only for the fresh third attempt, so the second attempt must not be described as a 5 MiB/s write run.

The third attempt ran **15:05:26.737–15:16:59.469 Asia/Almaty**, with recorded Docker timestamps `2026-11-05T10:05:26.736582154Z` to `2026-11-05T10:16:59.468803175Z`: **692.732221 seconds**. Its cgroup memory peak was **2,519,265,280 bytes**, with no OOM or application-readiness guard regression. Strict pipeline failure handling returned exit **1** when the decoder hit its expanded-source budget; no completed import date was recorded. The 639,820-document progress therefore does **not** establish a complete index, and this failed index was **not query-tested**.

The fourth attempt used a fresh index and raised only the decoder's expanded-source budget to **16 GiB**. Decoding streamed into Java stdin without a JSON intermediate; this limit is neither an allocated RAM amount nor a retained-file size. It consumed the source through EOF: **7,149,041,740 expanded bytes ≈ 6.658 GiB**. This explains why the earlier 4 GiB decoder bound could not produce a complete index.

The successful container ran on **5 November, 15:18:53.523–15:44:07.332 Asia/Almaty (UTC+5)**. Recorded Docker timestamps are `2026-11-05T10:18:53.52268851Z` and `2026-11-05T10:44:07.332033907Z`.

| Measure | Observed value |
| --- | ---: |
| Container elapsed time | **1,513.809345 seconds**, about 25 min 14 sec |
| Exit status / OOM kill | **0 / false** |
| CPU / memory caps | **2.0 CPU / 5 GiB** |
| Physical SSD read/write caps | **5/5 MiB/s** |
| Final cgroup memory peak | **2,504,134,656 bytes ≈ 2.332 GiB** |
| Final cgroup CPU time | **1,531.506068 seconds** |
| Physical SSD reads | **413,528,064 bytes ≈ 0.385 GiB** |
| Physical SSD writes | **3,545,706,496 bytes ≈ 3.302 GiB** |
| Retained Photon directory, apparent / allocated | **576,087,384 / 576,729,088 bytes**, about 0.537 GiB allocated |
| Final accepted import records | **1,497,241** |
| Logical index count | **1,497,308** |

The final import-record and logical-index counts are separate reported counters. Photon **1.3.0** returned status **`Ok`**, import date **`2026-10-31T23:59:58Z`**, and languages **`kk,ru,en`**. As with Valhalla, memory is cgroup charged memory rather than process RSS.

### Search and reverse queries

All **216 namespace-local HTTP/GeoJSON requests succeeded** against `127.0.0.1:2322`. Each concurrency setting used twelve first-observed fixture requests plus 96 repeated requests. The concurrency-2 phase followed concurrency 1, so its first pass already benefited from earlier work.

| Phase | Requests / successful | Client concurrency | p50 | p95 | Repeated-phase throughput |
| --- | ---: | ---: | ---: | ---: | ---: |
| First observed fixtures | 12 / 12 | 1 | **758.874 ms** | **2,609.840 ms** | — |
| Repeated fixtures | 96 / 96 | 1 | **35.626 ms** | **87.532 ms** | **20.453 req/s** |
| First pass after concurrency-1 phase | 12 / 12 | 2 | **29.433 ms** | **83.944 ms** | — |
| Repeated fixtures | 96 / 96 | 2 | **75.087 ms** | **91.582 ms** | **33.867 req/s** |

The serving container had a **2.0 CPU / 3 GiB** cap. Observed charged-memory peak was **767,369,216 bytes ≈ 0.715 GiB**. At the recorded serving snapshot, charged memory was **750,694,400 bytes**, including **626,712,576 anonymous** and **119,046,144 file-cache bytes**. These serving observations do not measure simultaneous search, routing and map load.

All twelve first-pass fixtures returned results, and all **36 top-three results** carried Kazakhstan country codes. Almaty city names localized in Kazakh, Russian and English; prefix `Алма` ranked Almaty first. The `Abay 150` fixture returned the matching house-number tag first, and a named Baiterek query ranked the monument first. Ranking still needs product review: Astana's administrative boundary preceded the city, Dostyk returned a hotel before the avenue, and the Astana reverse lookup returned the city before the monument. Some English requests fell back to Cyrillic names, and street/node/way representations produced duplicates.

This is a small urban fixture set without independent ground truth or rural-coverage validation. A nonempty GeoJSON response, a configured language list and Kazakhstan country codes do not establish complete addresses, translations or ranking quality. No global caches were flushed; neither first-pass phase is a cold-cache measurement.

## Maps: completed archive conversion and delivery

The input was [Geofabrik's prebuilt Kazakhstan Shortbread MBTiles](https://download.geofabrik.de/asia/kazakhstan-shortbread-1.0.mbtiles), converted with the pinned [PMTiles CLI 1.31.2](https://github.com/protomaps/go-pmtiles/releases/tag/v1.31.2) and served by an isolated Nginx instance. **This is conversion of an existing archive, not map generation from the raw OSM PBF.**

Conversion ran on **5 November, 15:44:50.127–15:55:17.606 Asia/Almaty (UTC+5)**; raw Docker timestamps are `2026-11-05T10:44:50.126949789Z` and `2026-11-05T10:55:17.605703Z`.

| Measure | Observed value |
| --- | ---: |
| Container elapsed time | **627.478754 seconds**, about 10 min 27 sec |
| Exit status / OOM kill | **0 / false** |
| CPU / memory caps | **1.0 CPU / 2 GiB** |
| Physical SSD read/write caps | **5/5 MiB/s** |
| Final cgroup memory peak | **2,147,487,744 bytes**, approximately 2 GiB |
| Final cgroup CPU time | **131.640866 seconds** |
| Physical SSD reads / writes | **2,826,080,256 / 2,277,416,960 bytes** |
| Source MBTiles / output PMTiles | **1,558,294,528 / 1,298,561,630 bytes**, about 1.451 / 1.209 GiB |
| Addressed tiles / zoom range | **1,718,082 / 0–14** |

Conversion reached its charged-memory limit: final `memory.events.max` was **6,470**, while `oom` and `oom_kill` stayed **0**. A near-limit snapshot contained **206,979,072 anonymous bytes** and **1,914,380,288 file-cache bytes**. This was cache-heavy limit/reclaim pressure, not evidence that the process required 2 GiB of anonymous memory or that the cap was untouched.

`pmtiles verify` exited **0**. All **15/15 MBTiles sample locations** were present across Almaty, Astana, Shymkent, Charyn and Aktau at zooms 6, 10 and 14. The full SHA256 values, per-zoom counts, archive metadata and sample hashes are in the machine-readable results. The source hash is `9263abe5ca89acc19ae11b6f60cfd4a026c76a19421cd055ae39d3f3ba2b1c5c`; the output hash is `2583113c45ca582906e9e92c343f1c5a11ff08d6292c1eb8137cb0e764adb25c`.

The range client joined Nginx's network namespace. All **256/256 requests** returned HTTP **206**, exact `Content-Range` values and bytes matching the local archive. Each phase used 64 deterministic ranges, up to 64 KiB per request, with a fresh HTTP connection; local byte comparison was excluded from measured latency.

| Phase | Client concurrency | Verified / requested | p50 | p95 |
| --- | ---: | ---: | ---: | ---: |
| First observed ranges | 1 | 64 / 64 | **0.834 ms** | **6.010 ms** |
| Repeated ranges | 1 | 64 / 64 | **0.592 ms** | **0.749 ms** |
| First pass after concurrency-1 phases | 4 | 64 / 64 | **1.638 ms** | **3.706 ms** |
| Repeated ranges | 4 | 64 / 64 | **1.554 ms** | **2.677 ms** |

Nginx's observed charged-memory peak was **4,734,976 bytes**. No caches were flushed, and later phases reused the same ranges. These results establish archive/file transport, not PMTiles client lookup, browser rendering speed, Internet latency, multilingual-label quality or full map coverage.

## Monitoring, final host checks and cleanup

After conversion, an all-container Docker inspection timed out at **30 seconds**, causing the monitor to abort at approximately **15:55:53 Asia/Almaty (10:55:53 UTC)**. Application probes were healthy immediately beforehand, around **3.8 ms**; this was a separate **monitor failure**, not a failed conversion or another observed application-readiness regression. Phase-4 controls, samples and abort evidence were archived. Twelve fresh readiness probes and the original 41-container invariants passed before a new strict monitor/independent lease supervised read-only archive verification and range requests.

The combined serial follow-up sample set contains **2,769 observations**, with **zero failed readiness probes and zero sampled guard flags**. Readiness p95 was **3.993 ms**, maximum **38.231 ms**. Minimum available RAM was **7,909,765,120 bytes ≈ 7.367 GiB**; minimum free filesystem space was **31,140,925,440 bytes ≈ 29.002 GiB**; maximum sampled benchmark-directory size was **7,277,906,127 bytes ≈ 6.778 GiB**. I/O `full avg10` nevertheless reached **76.97%**. These serial-run observations neither erase the five earlier readiness timeouts nor remove the distinct Docker-telemetry abort.

Final `finish` and process-cleanup records confirm **no running benchmark container, watchdog or lease**, and no changes to the original **41 container identities, start times or restart counts**. Local readiness passed in **37.374 ms**. Public `https://bikers.kz/health/ready` returned **HTTP 200**, body `{"status":"ready"}`, in **236.227 ms**. Available RAM was approximately **9.567 GiB**, free filesystem space **30.201 GiB**, and retained benchmark allocation **6,022,107,136 bytes ≈ 5.609 GiB**. Monitoring records were copied from tmpfs into retained results.

## Receipt index

The public [results JSON](osm-kazakhstan-benchmark-results.json) contains sanitized aggregates. The following raw evidence stays under the private benchmark root; paths below are relative to that root:

| Evidence | Retained path |
| --- | --- |
| Final host/container verification | `finished.json` |
| Combined serial follow-up samples | `results/final-samples.jsonl` |
| Post-conversion Docker-inspection abort | `results/phase4-abort.json` |
| Earlier monitor/guard aborts, including the readiness incident | `results/attempt1-monitor-abort.json`, `results/attempt2-abort.json`, `results/attempt3-abort.json` |
| Valid route queries | `results/valhalla-query-local-measurements.json` |
| Valid search queries and index metadata | `results/photon-query-c1-measurements.json`, `results/photon-query-c2-measurements.json`, `results/photon-index-metadata-measurements.json` |
| Archive inventory, ranges and recovered conversion state | `results/maps-inventory-measurements.json`, `results/maps-ranges-measurements.json`, `results/maps-convert-recovered-state.json` |
| Process cleanup, public readiness and sanitized export | `results/cleanup-processes.json`, `results/public-readiness.json`, `results/published-results.json` |
| Final import/conversion cgroup counters | `data/valhalla3/import-cgroup-final.txt`, `data/photon-stream4/import-cgroup-final.txt`, `data/maps/convert-cgroup-final.txt` |

Failed Photon imports retain separate final counters under `data/photon-stream`, `data/photon-stream2` and `data/photon-stream3`. Phase command/state receipts, input checksums, image digests and logs remain alongside these measurements; unrelated-container inventory is not published.

## Acceptance and proposed operating budget

**Individual component acceptance is complete:** the graph and search index built successfully, valid route/search fixtures succeeded, the map archive verified, exact file-range delivery succeeded, and final host cleanup/readiness passed. **No-impact acceptance failed** because of the initial readiness incident. This does not approve a production rollout or establish combined-stack capacity, browser usability, comprehensive Kazakhstan addresses, multilingual labels or motorcycle-route safety.

A small guarded serving pilot on Jarvis is reasonable, with an **estimated additional 4–5 GiB service-memory budget and at least 10 GiB of data working headroom**; Docker images and backups need additional space. These are planning allowances, **not tested minimums**. Observed serving cgroup charges after warm imports do not establish total resident requirements: cached file pages can remain charged to earlier or root cgroups, and combined search/routing/map traffic was not tested.

Prefer off-host data builds or deliberately scheduled serial rebuilds with readiness monitoring, retained rollback data and capacity for current/next datasets. The results do not justify unattended parallel imports on the shared SSD. Before enabling the platform integration, test combined traffic, rebuild/refresh behavior, browser styling and Kazakh/Russian labels, search ranking/duplicates, and rider-reviewed routes.

# Isolated Kazakhstan OSM benchmark

This harness measures country-scoped search, motorcycle routing and map-file delivery on Jarvis. It invokes no application deployment command. The source PR is unmerged; the completed run is recorded in the [benchmark report](../../docs/osm-kazakhstan-benchmark.md) and [machine-readable results](../../docs/osm-kazakhstan-benchmark-results.json).

The authorized run uses `/home/user/benchmarks/bikers-osm-kz-20261105-16`. Its files, data and labelled containers are separate from the BIKERS.KZ runtime. The benchmark shares physical CPU, memory and storage with existing services, so these controls reduce and detect interference; they do not establish zero impact.

Initial parallel preparation I/O caused a real application-readiness regression. Subsequent work used serial stages and an immediate 250 ms readiness guard. Limits are attempt-specific: two Photon attempts reached the upstream bulk timeout; a third reached its 4 GiB expanded-source limit. The fourth fresh import completed at 2.0 CPU, 5/5 MiB/s read/write and an explicit 16 GiB streamed-source budget. Map conversion and all 256 exact Range-response checks also completed. A later Docker-inspection timeout aborted the monitor separately; fresh readiness/container checks preceded a supervised read-only continuation. Final cleanup/readiness passed. Preserve both failures and the initial readiness incident: successful component tests did not meet the no-impact criterion.

## Components

| File | Interface and responsibility |
| --- | --- |
| [run.py](run.py) | `prepare`, `watch`, `lease`, `fetch`, `exec`, `stage`, `stop`, `finish`; host guards, bounded processes, labelled Docker lifecycle and receipts. |
| [photon.py](photon.py) | `manifest`, `fixtures`, `stream`, `prepare`, `plan`, `benchmark`; pinned input verification, bounded Zstandard decoding, Kazakhstan/language import plans and search queries. The final workflow uses `stream`; `prepare` remains an explicit materialization option. |
| [valhalla.py](valhalla.py) | `manifest`, `fixtures`, `spec`, `query`; pinned image/input description, offline graph-build plans and motorcycle route queries. |
| [maps.py](maps.py) | `inspect`, `range-bench`; MBTiles/PMTiles inventory and verified loopback HTTP Range requests. It does not run a web server. |

Use Python 3 on the benchmark host, Docker with cgroup v2, `curl`, `nice`, `ionice` and `du`. Run `--help` on each helper for its argument positions. The supervisor's global `--root` and the maps helper's global `--output` precede their subcommands.

## Run order

1. **Prepare once.** Check available capacity, the input manifests, image CPU compatibility and unused loopback ports. Record image digests before import. `prepare` refuses to overwrite a baseline and requires at least 8 GiB available RAM, 30 GiB free filesystem space and twelve successful application readiness probes.

   ```sh
   BENCHMARK_ROOT=/home/user/benchmarks/bikers-osm-kz-20261105-16
   python3 run.py --root "$BENCHMARK_ROOT" prepare
   ```

2. **Start both monitoring processes before work.** The run's heartbeat/control files and `samples.jsonl` use symlinks from `ROOT` into `/dev/shm/ROOT_BASENAME`. This tmpfs placement prevents SSD writeback from stalling the safety heartbeat; `save()` follows those symlinks. It is operator setup, not something `prepare` creates. Copy volatile records to persistent results before cleanup or reboot.

   Run `watch --duration 10800` in a managed background process and wait for a fresh `watch.json`. Slow Docker/container/directory telemetry runs asynchronously, so the readiness heartbeat can continue while telemetry runs. Separately run `lease --duration 10800` under a transient systemd service with `RuntimeMaxSec` and an `ExecStopPost` calling `stop`. The lease independently checks the heartbeat; the systemd stop hook covers termination of the lease itself. Creating these units is an operator step, not something `stage` does automatically. Their commands use the absolute path of the copied `run.py`:

   | Unit setting | Command/action |
   | --- | --- |
   | Watch process | `python3 run.py --root ROOT watch --duration 10800` |
   | Lease `ExecStart` | `python3 run.py --root ROOT lease --duration 10800` |
   | Lease `ExecStopPost` | `python3 run.py --root ROOT stop` |
   | Lease `RuntimeMaxSec` | A bounded lifetime covering the phase; the commands above use three hours. Renewed phases need their own explicit bound. |

3. **Fetch and pull under supervision.** `fetch` needs a filename, URL, maximum byte count and timeout; supply the manifest's checksum where available. It limits download rate to `3M`, records SHA256/MD5 and refuses existing targets. Use `exec --name NAME --timeout SECONDS -- COMMAND ...` for bounded image pulls and other preparation commands. Resolve pulled images to digests and retain receipts. Network access is for this acquisition phase; imports use pre-fetched inputs.

   ```sh
   python3 photon.py manifest
   python3 valhalla.py manifest
   python3 run.py --root "$BENCHMARK_ROOT" fetch \
     --name kazakhstan-261103.osm.pbf \
     --url https://download.geofabrik.de/asia/kazakhstan-261103.osm.pbf \
     --max-bytes 300000000 --timeout 900 \
     --md5 53403ba99b12bcdf5183316539ef2440
   ```

4. **Import sequentially, offline.** The initial materialized Photon JSON was discarded. The final import streams bounded Zstandard decoding directly into Java stdin, avoiding that intermediate write. It runs in the Ubuntu 24 Valhalla image with a read-only Java 21 runtime extracted from the pinned Temurin image, without installing a host JRE. The fourth import explicitly uses `photon.py stream --source FILE --max-output-gib 16` and Photon `-import-file -`, filtering to `kz` and `kk,ru,en`; retain both decoder and Java exit results with strict pipeline failure handling. The 16 GiB limit bounds expanded bytes passed through the pipe; it does not materialize a 16 GiB file or allocate that much RAM. The previous 4 GiB limit stopped the third attempt, so an incomplete index from that attempt is not a successful import. The older materializing `prepare`/`plan` path is not the measured final workflow.

   Place the verified PBF in the dedicated Valhalla data directory; `spec --stage config` prepares configuration, then `--stage import` builds the road graph. Start with new index/graph directories. Photon import deletes/recreates its index; a repeated Valhalla import can reuse existing tiles, so it is not automatically a fresh-build measurement. Stop one workload before starting the next.

   The adapters emit plans; they do not execute Docker. Normalize them into reviewed `stage` JSON with `name`, `image`, `args`, `command`, `memory`, `cpus`, `read_bps`, `write_bps`, `network` and `timeout`; set `detach` only for serving. Put required environment overrides in `args` as `--env` entries. Photon returns Docker options separately. Valhalla's `docker_args` also contains the image/command, which must not be duplicated when adapting it. The final streaming Photon spec adapts its entrypoint, runtime mounts and input pipeline explicitly. `stage SPEC.json` launches the reviewed specification and records its command, exit status and Docker resource settings.

5. **Serve and query within benchmark namespaces.** The attempted host bindings were Photon `127.0.0.1:33811`, Valhalla `127.0.0.1:33812` and maps `127.0.0.1:33813`. On this host, Docker's internal network left those published ports absent (`Ports: null`). Host-port query attempts with zero successes are excluded from service latency results.

   Valid route/search queries use inline Python through `docker exec`, targeting `127.0.0.1:8002` for Valhalla and `127.0.0.1:2322` for Photon inside each serving container's network namespace. Retain the same fixtures, response checks and bounded concurrency. For map ranges, a client container can join the Nginx namespace with stage `network: "container:NGINX_BENCHMARK_CONTAINER"`, mount the local archive read-only for byte comparison, and call the server's internal loopback port. This does not require a public listener or modifications to existing Docker networks.

   Valhalla serving bypasses its build entrypoint and mounts graph data read-only. Serve the raw PMTiles archive using the isolated Nginx instance; `pmtiles serve` exposes Z/X/Y tiles, not raw-file Range responses. Run each client through a bounded supervisor phase. The generic namespace invocation is:

   ```sh
   python3 run.py --root "$BENCHMARK_ROOT" exec --name valhalla-query --timeout 240 -- \
     docker exec "$VALHALLA_BENCHMARK_CONTAINER" python3 -c "$REVIEWED_QUERY_CODE"
   ```

6. **Finish and verify.** `finish` stops labelled benchmark containers and verifies none remain running, then compares existing services and readiness with baseline. Inspect `finished.json`, `abort.json` if present, query failures and systemd cleanup status before concluding. The helpers preserve data/receipts for review; they do not delete the benchmark root or prune shared Docker images.

   ```sh
   python3 run.py --root "$BENCHMARK_ROOT" finish
   ```

## Isolation and abort controls

- Containers carry `bkrs.osm-benchmark=ROOT_BASENAME`; cleanup selects that exact label. Existing application containers, settings, database and deployment receipts are outside the task's mounts.
- The initial successful Valhalla build used **1.5 CPU** and **20/10 MiB/s read/write** caps. Photon imports at **0.75 CPU** and then **2.0 CPU**, both with **5/2 MiB/s read/write**, failed at the upstream **30-second bulk timeout without OOM**. The third attempt at **2.0 CPU and 5/5 MiB/s** failed on the decoder's 4 GiB expanded-source bound, not OOM. The fourth fresh import completed at **2.0 CPU, 5/5 MiB/s and 5 GiB RAM**, with an explicit 16 GiB streamed-source budget and strict readiness guard. Its decoder reached source EOF after about **6.658 GiB**; 216 valid query requests succeeded. Map conversion used **1.0 CPU, 2 GiB and 5/5 MiB/s**; its charged memory reached the cap with file-cache reclaim events and no OOM. `stage` defaults to 1.5 CPU and 5/2 MiB/s, so the saved phase command and actual `cpu.max`/`io.max` are the authority for each attempt. Search/routing serving has a **3 GiB** cap; the measured Photon server used **2.0 CPU**. Limits are per container, not a combined-stack allowance.
- Low CPU shares, a PID limit of 256, dropped capabilities and `no-new-privileges` remain enabled. `--memory-swap` equal to memory disables additional container swap; host swap configuration is unchanged. I/O caps target `/dev/sda`; `nice`/`ionice` on the Docker client alone do not establish worker priority. Mounts and temporary files are phase-specific, without a Docker socket or privileged container.
- The watchdog samples host/readiness about every two seconds, retaining asynchronously refreshed Docker/directory telemetry. The strict follow-up baseline sets `readiness_limit: 0.25` and `fail_fast_readiness: true`: readiness failure or latency above **250 ms aborts immediately**. `prepare` alone does not set these overrides. Existing-container invariant changes and benchmark directory above **12 GiB** also cause immediate abort. Other sustained guards include **MemAvailable below 3 GiB**, filesystem free space below **20 GiB**, swap growth above baseline plus 256 MiB, and stale Docker telemetry. New work refuses a watchdog heartbeat older than fifteen seconds.
- Baseline-running containers must retain their IDs, start times and restart counts; previously healthy containers must stay healthy. The independent lease and systemd stop hook stop only benchmark-labelled containers on heartbeat failure, expiry or service termination. Inspect cleanup failures rather than treating a stop request as proof of cleanup.

## Reading the measurements

Retain `baseline.json`, a persistent copy of tmpfs `samples.jsonl`/control records, phase commands/states, input checksums, image digests, logs and final verification. Supervisor phase receipts use `results/NAME-receipt.json`; client measurement JSON is separate. Preserve original aborted-attempt evidence before an explicitly reviewed continuation. Photon and Valhalla import wrappers write final cgroup counters before exiting. **Cgroup charged memory includes file cache and is not process RSS**; use `memory.stat` to separate anonymous/file charges. Sampled running-service counters can miss activity between samples; do not invent precise completed-job totals when final counters are unavailable.

No global caches are flushed. “First pass” means the first observed query in that phase, **not a cold-cache result**. Loopback queries exclude Internet/client-device conditions. A successful geocoder response does not prove address correctness; inspect returned objects and missing translations. Routing fixtures do not certify road suitability or live traffic.

Serving-memory snapshots after imports are observed cgroup charges, not tested minimums or total resident requirements. File pages can remain charged to earlier/root cgroups. The component queries do not measure simultaneous search, routing and map demand.

For maps, converting Geofabrik's prebuilt Shortbread MBTiles to PMTiles measures **archive conversion, not generation from the raw OSM PBF**. Inventory hashes/tile presence and verified HTTP ranges establish file/content delivery, not browser rendering speed, full multilingual labels or map coverage quality. Keep these distinctions in the results report.

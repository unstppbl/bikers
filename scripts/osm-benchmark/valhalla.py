#!/usr/bin/env python3
"""Describe isolated Valhalla containers and query an already running benchmark.

This helper does not run Docker, download data, contact Jarvis, or change a
running service. The launcher owns container limits, names, labels, monitoring,
image verification, input download/checksums and cleanup.
"""

from __future__ import annotations

import argparse
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime, timezone
import json
import math
import os
from pathlib import Path
import time
from urllib.error import HTTPError, URLError
from urllib.parse import urlparse
from urllib.request import Request, urlopen


IMAGE = (
    "ghcr.io/valhalla/valhalla-scripted:3.9.1-amd64@"
    "sha256:31d96eb75f73e5f5b3dc40702edfda16c4e1793a384a8235bcf00dd3eed1b9bc"
)
INPUT = {
    "filename": "kazakhstan-261103.osm.pbf",
    "url": "https://download.geofabrik.de/asia/kazakhstan-261103.osm.pbf",
    "checksum_url": "https://download.geofabrik.de/asia/kazakhstan-261103.osm.pbf.md5",
    "md5": "53403ba99b12bcdf5183316539ef2440",
    "listed_bytes": 225713312,
    "data_timestamp": "2026-11-03T21:24:23Z",
    "maximum_download_bytes": 300000000,
}

# Public approximate coordinates are test inputs, not verified meeting places.
FIXTURES = (
    ("almaty_city", ((43.2410, 76.9120), (43.2600, 76.9410))),
    ("talgar_esik_rural", ((43.3030, 77.2400), (43.3550, 77.4640))),
    ("almaty_astana_long", ((43.2410, 76.9120), (51.1282, 71.4304))),
)

CONFIG_SCRIPT = r'''
import hashlib, json, os, pathlib, subprocess
root = pathlib.Path("/custom_files")
config = json.loads(subprocess.check_output(["valhalla_build_config"], text=True))
config["mjolnir"].update({
    "tile_dir": str(root / "valhalla_tiles"),
    "tile_extract": str(root / "valhalla_tiles.tar"),
    "admin": str(root / "admins.sqlite"),
    "timezone": str(root / "timezones.sqlite"),
    "traffic_extract": "",
    "concurrency": int(os.environ["BENCHMARK_THREADS"]),
    "include_driving": True,
    "include_bicycle": False,
    "include_pedestrian": False,
})
config["additional_data"]["elevation"] = str(root / "elevation_data")
config["loki"]["actions"] = ["route", "status"]
config["service_limits"]["motorcycle"]["max_distance"] = 2000000
config["service_limits"]["motorcycle"]["max_locations"] = 10
config["httpd"]["service"]["timeout_seconds"] = 45
path = root / "valhalla.json"
path.write_text(json.dumps(config, indent=2) + "\n")
print(json.dumps({"config": str(path), "sha256": hashlib.sha256(path.read_bytes()).hexdigest()}))
'''


def manifest() -> dict:
    return {
        "image": IMAGE,
        "platform": "linux/amd64",
        "input": INPUT,
        "sources": {
            "image": "https://github.com/valhalla/valhalla/pkgs/container/valhalla-scripted/1343840011?tag=3.9.1-amd64",
            "entrypoint": "https://github.com/valhalla/valhalla/blob/3.9.1/docker/scripts/docker-entrypoint.sh",
            "configuration": "https://github.com/valhalla/valhalla/blob/3.9.1/scripts/valhalla_build_config",
        },
        "scope": "Static motorcycle routing; no elevation, timezone, traffic or transit acceptance.",
    }


def spec(stage: str, data_dir: Path, threads: int, port: int) -> dict:
    if not data_dir.is_absolute():
        raise ValueError("--data-dir must be an absolute path dedicated to this benchmark")
    mount = f"type=bind,source={data_dir},target=/custom_files"
    environment: dict[str, str] = {}
    ports: list[dict] = []
    if stage == "config":
        entrypoint, command = "python3", ["-c", CONFIG_SCRIPT]
        environment = {"BENCHMARK_THREADS": str(threads)}
    elif stage == "import":
        entrypoint, command = "/valhalla/scripts/docker-entrypoint.sh", ["build_tiles"]
        environment = {
            "server_threads": str(threads),
            "tile_urls": "",
            "serve_tiles": "False",
            "build_tar": "True",
            "force_rebuild": "False",
            "use_tiles_ignore_pbf": "True",
            "update_existing_config": "False",
            "build_admins": "True",
            "build_time_zones": "False",
            "build_elevation": "False",
            "build_transit": "False",
            "use_default_speeds_config": "False",
            "traffic_name": "",
            "path_extension": "",
            "tileset_name": "valhalla_tiles",
        }
    else:
        # Bypass the build entrypoint entirely: serving must never rebuild data.
        entrypoint = "valhalla_service"
        command = ["/custom_files/valhalla.json", str(threads)]
        mount += ",readonly"
        ports = [{"host": "127.0.0.1", "host_port": port, "container_port": 8002}]

    if stage == "import":
        original = [entrypoint, *command]
        entrypoint = "/bin/sh"
        wrapper = '''set +e
"$@"
result=$?
{
  echo metric_kind=cgroup_charged_memory_not_process_rss
  echo workload_exit_status=$result
  for metric in memory.peak memory.stat memory.events cpu.stat io.stat; do
    echo "[$metric]"
    cat "/sys/fs/cgroup/$metric" 2>/dev/null || echo unavailable
  done
} > /custom_files/import-cgroup-final.txt
exit "$result"
'''
        command = ["-c", wrapper, "valhalla-import", *original]

    docker_args = ["--platform", "linux/amd64", "--user", f"{os.getuid()}:{os.getgid()}",
                   "--env", "HOME=/tmp", "--mount", mount]
    for key, value in environment.items():
        docker_args.extend(["--env", f"{key}={value}"])
    for binding in ports:
        docker_args.extend([
            "--publish",
            f"{binding['host']}:{binding['host_port']}:{binding['container_port']}",
        ])
    docker_args.extend(["--entrypoint", entrypoint, IMAGE, *command])
    return {
        "stage": stage,
        "image": IMAGE,
        "entrypoint": entrypoint,
        "command": command,
        "environment": environment,
        "ports": ports,
        "docker_args": docker_args,
        "launcher_requirements": {
            "network": "none for config/import; isolated bridge with loopback-only port for serve",
            "read_only_root": True,
            "tmpfs": "/tmp:rw,nosuid,nodev,size=128m",
            "privileges": "drop all capabilities; no-new-privileges; no Docker socket",
            "resource_caps": "launcher must set CPU, memory=memory-swap, PIDs and I/O bounds",
            "restart": "no",
            "input": "verified pinned PBF must be in the dedicated data directory before import",
        },
        "notes": [
            "Import is for a new, dedicated directory. Existing routing tiles are reused on a repeated import.",
            "Serving bypasses the scripted entrypoint and mounts data read-only; it cannot rebuild or fetch data.",
            "A --version probe is necessary but does not prove every binary path supports this CPU.",
        ],
    }


def request_payload(points: tuple[tuple[float, float], ...]) -> dict:
    return {
        "locations": [{"lat": lat, "lon": lon} for lat, lon in points],
        "costing": "motorcycle",
        "costing_options": {
            "motorcycle": {"use_highways": 0.5, "use_trails": 0, "use_tracks": 0}
        },
        "units": "kilometers",
        "language": "en-US",
    }


def query_once(base_url: str, fixture: tuple, phase: str, concurrency: int,
               repeat: int, timeout: float, deadline: float) -> dict:
    name, points = fixture
    payload = request_payload(points)
    result = {
        "fixture": name, "phase": phase, "concurrency": concurrency,
        "repeat": repeat, "request": payload,
    }
    started = time.monotonic()
    remaining = deadline - started
    if remaining <= 0:
        return {**result, "ok": False, "error": "benchmark duration exhausted", "skipped": True}
    try:
        request = Request(
            f"{base_url.rstrip('/')}/route", data=json.dumps(payload).encode(),
            headers={"Content-Type": "application/json"}, method="POST",
        )
        try:
            response = urlopen(request, timeout=min(timeout, remaining))
        except HTTPError as error:
            response = error
        with response:
            result["http_status"] = response.status
            body = response.read(2 * 1024 * 1024 + 1)
        if len(body) > 2 * 1024 * 1024:
            raise ValueError("response exceeded 2 MiB")
        document = json.loads(body)
        if not isinstance(document, dict):
            raise ValueError("response root was not a JSON object")
        trip = document.get("trip", {})
        if not isinstance(trip, dict):
            raise ValueError("response trip was not a JSON object")
        result.update({
            "ok": result["http_status"] == 200 and trip.get("status") == 0,
            "vendor_status": trip.get("status"),
            "vendor_status_message": trip.get("status_message"),
            "error_code": document.get("error_code"),
            "error": document.get("error"),
            "summary": trip.get("summary"),
            "returned_locations": trip.get("locations"),
        })
    except (URLError, OSError, ValueError, TimeoutError) as error:
        result.update({"ok": False, "error": str(error)})
    result["elapsed_ms"] = round((time.monotonic() - started) * 1000, 3)
    return result


def benchmark(base_url: str, repeats: int, timeout: float, duration: float) -> dict:
    parsed = urlparse(base_url)
    if (parsed.scheme != "http" or parsed.hostname not in {"127.0.0.1", "localhost", "::1"}
            or parsed.path not in {"", "/"} or parsed.query or parsed.fragment
            or parsed.username or parsed.password):
        raise ValueError("--base-url must be an unauthenticated loopback HTTP origin")
    started = time.monotonic()
    deadline = started + duration
    rows = [query_once(base_url, fixture, "first_request", 1, 0, timeout, deadline)
            for fixture in FIXTURES]
    for concurrency in (1, 2):
        jobs = [(fixture, repeat) for repeat in range(1, repeats + 1) for fixture in FIXTURES]
        with ThreadPoolExecutor(max_workers=concurrency) as executor:
            rows.extend(executor.map(
                lambda job: query_once(base_url, job[0], "repeated", concurrency,
                                       job[1], timeout, deadline), jobs,
            ))
    summaries = []
    for phase, concurrency in (("first_request", 1), ("repeated", 1), ("repeated", 2)):
        group = [row for row in rows if row["phase"] == phase and row["concurrency"] == concurrency]
        timings = sorted(row["elapsed_ms"] for row in group if row.get("ok"))
        summaries.append({
            "phase": phase, "concurrency": concurrency, "requested": len(group),
            "successful": len(timings), "errors": sum(not row.get("ok") for row in group),
            "p50_ms": timings[math.ceil(len(timings) * 0.5) - 1] if timings else None,
            "p95_ms": timings[math.ceil(len(timings) * 0.95) - 1] if timings else None,
            "max_ms": max(timings) if timings else None,
        })
    return {
        "measured_at": datetime.now(timezone.utc).isoformat(),
        "manifest": manifest(), "base_url": base_url,
        "elapsed_seconds": round(time.monotonic() - started, 3),
        "method": {
            "repeats_per_fixture_per_concurrency": repeats,
            "timeout_seconds": timeout, "start_budget_seconds": duration,
            "first_request": "First observed request per fixture; OS/file caches are not flushed.",
            "repeated": "Same three fixtures, with at most one or two client calls in flight.",
            "limits": "Latency includes loopback HTTP/JSON; this is not a throughput or route-safety guarantee.",
        },
        "summaries": summaries, "requests": rows,
    }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("manifest")
    commands.add_parser("fixtures")
    plan = commands.add_parser("spec")
    plan.add_argument("--stage", choices=("config", "import", "serve"), required=True)
    plan.add_argument("--data-dir", type=Path, required=True)
    plan.add_argument("--threads", type=int, choices=(1, 2), default=1)
    plan.add_argument("--port", type=int, default=33812)
    queries = commands.add_parser("query")
    queries.add_argument("--base-url", required=True)
    queries.add_argument("--repeats", type=int, default=3)
    queries.add_argument("--timeout", type=float, default=30)
    queries.add_argument("--duration", type=float, default=180)
    queries.add_argument("--output", type=Path)
    args = parser.parse_args()
    try:
        if args.command == "manifest":
            result = manifest()
        elif args.command == "fixtures":
            result = [{"name": name, "request": request_payload(points)} for name, points in FIXTURES]
        elif args.command == "spec":
            if not 1024 <= args.port <= 65535:
                raise ValueError("--port must be between 1024 and 65535")
            result = spec(args.stage, args.data_dir, args.threads, args.port)
        else:
            if not 1 <= args.repeats <= 20 or not 0 < args.timeout <= 60 or not 0 < args.duration <= 600:
                raise ValueError("repeats must be 1–20, timeout at most 60 seconds, duration at most 600 seconds")
            result = benchmark(args.base_url, args.repeats, args.timeout, args.duration)
        output = json.dumps(result, indent=2) + "\n"
        if args.command == "query" and args.output:
            args.output.write_text(output)
        print(output, end="")
    except ValueError as error:
        parser.error(str(error))


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Photon benchmark adapter; host monitoring and Docker resource limits are external."""

from __future__ import annotations

import argparse
import concurrent.futures
import ctypes
import ctypes.util
import hashlib
import json
import math
import os
from pathlib import Path
import shutil
import statistics
import sys
import time
import urllib.error
import urllib.parse
import urllib.request


VERSION = "1.3.0"
JAR_URL = "https://github.com/komoot/photon/releases/download/1.3.0/photon-1.3.0.jar"
JAR_SHA256 = "a89707c0045e4807b2a1180e132e68e108d998709f48b6c94b98a6e281f571a5"
DUMP_URL = "https://download1.graphhopper.com/public/asia/central-asia/photon-dump-central-asia-1.0-latest.jsonl.zst"
DUMP_MD5 = "7d0ddcbee9f54d5f5b36d9912058968e"
DUMP_BYTES = 191650136
MIB = 1024**2
GIB = 1024**3
IMPORT_WRAPPER = """java "$@"
java_status=$?
{
    printf 'metric_kind=cgroup_charged_memory_not_process_rss\\n'
    printf 'java_exit_status=%s\\n' "$java_status"
    for metric in memory.peak memory.stat memory.events cpu.stat io.stat; do
        printf '\\n[%s]\\n' "$metric"
        if [ -r "/sys/fs/cgroup/$metric" ]; then
            cat "/sys/fs/cgroup/$metric"
        else
            printf 'unavailable\\n'
        fi
    done
} > /data/import-cgroup-final.txt
exit "$java_status"
"""

# These are public search examples, not assertions that every house/translation exists.
FIXTURES = [
    {"id": "almaty_en", "path": "/api/", "params": {"q": "Almaty", "lang": "en"}},
    {"id": "almaty_ru", "path": "/api/", "params": {"q": "Алматы", "lang": "ru"}},
    {"id": "almaty_kk", "path": "/api/", "params": {"q": "Алматы", "lang": "kk"}},
    {"id": "astana_en", "path": "/api/", "params": {"q": "Astana", "lang": "en"}},
    {"id": "astana_ru", "path": "/api/", "params": {"q": "Астана", "lang": "ru"}},
    {"id": "dostyk_kk", "path": "/api/", "params": {"q": "Достық Алматы", "lang": "kk", "lat": 43.24, "lon": 76.95}},
    {"id": "dostyk_ru", "path": "/api/", "params": {"q": "Достык Алматы", "lang": "ru", "lat": 43.24, "lon": 76.95}},
    {"id": "abai_house", "path": "/api/", "params": {"q": "Абай 150 Алматы", "lang": "ru", "lat": 43.24, "lon": 76.9}},
    {"id": "baiterek", "path": "/api/", "params": {"q": "Бәйтерек Астана", "lang": "kk", "lat": 51.1283, "lon": 71.4307}},
    {"id": "prefix", "path": "/api/", "params": {"q": "Алма", "lang": "ru", "lat": 43.24, "lon": 76.95}},
    {"id": "reverse_almaty", "path": "/reverse", "params": {"lat": 43.2389, "lon": 76.8897, "lang": "ru"}},
    {"id": "reverse_astana", "path": "/reverse", "params": {"lat": 51.1283, "lon": 71.4307, "lang": "kk"}},
]


def digest(path: Path, algorithm: str) -> str:
    value = hashlib.new(algorithm)
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(MIB), b""):
            value.update(chunk)
    return value.hexdigest()


def paths(root: Path) -> dict[str, Path]:
    return {
        "jar": root / "inputs" / "photon.jar",
        "compressed": root / "inputs" / "photon.jsonl.zst",
        "jsonl": root / "inputs" / "photon.jsonl",
        "data": root / "photon-data",
    }


def manifest() -> dict:
    return {
        "version": VERSION,
        "sources": [
            {"url": JAR_URL, "path": "inputs/photon.jar", "sha256": JAR_SHA256, "bytes": 98219380},
            {"url": DUMP_URL, "path": "inputs/photon.jsonl.zst", "md5": DUMP_MD5, "bytes": DUMP_BYTES},
        ],
        "source_timestamp": "2026-10-31T23:59:58.000+00:00",
        "countries": ["kz"],
        "languages": ["kk", "ru", "en"],
        "docs": "https://github.com/komoot/photon/blob/1.3.0/docs/usage.md",
    }


def decode_zstd(source: Path, output, max_bytes: int, on_chunk=None) -> dict:
    """Decode a bounded zstd stream to a binary writer without materializing JSON."""
    with source.open("rb") as compressed:
        if compressed.read(4) != b"\x28\xb5\x2f\xfd":
            raise ValueError("Input does not have the Zstandard frame magic")
    library = ctypes.util.find_library("zstd")
    if library is None:
        raise ValueError("libzstd is unavailable; provide a bounded decompression container")
    lib = ctypes.CDLL(library)

    class Input(ctypes.Structure):
        _fields_ = [("src", ctypes.c_void_p), ("size", ctypes.c_size_t), ("pos", ctypes.c_size_t)]

    class Output(ctypes.Structure):
        _fields_ = [("dst", ctypes.c_void_p), ("size", ctypes.c_size_t), ("pos", ctypes.c_size_t)]

    lib.ZSTD_createDStream.restype = ctypes.c_void_p
    lib.ZSTD_freeDStream.argtypes = [ctypes.c_void_p]
    lib.ZSTD_initDStream.argtypes = [ctypes.c_void_p]
    lib.ZSTD_initDStream.restype = ctypes.c_size_t
    lib.ZSTD_decompressStream.argtypes = [ctypes.c_void_p, ctypes.POINTER(Output), ctypes.POINTER(Input)]
    lib.ZSTD_decompressStream.restype = ctypes.c_size_t
    lib.ZSTD_isError.argtypes = [ctypes.c_size_t]
    lib.ZSTD_isError.restype = ctypes.c_uint
    lib.ZSTD_getErrorName.argtypes = [ctypes.c_size_t]
    lib.ZSTD_getErrorName.restype = ctypes.c_char_p

    def check(code: int) -> int:
        if lib.ZSTD_isError(code):
            raise ValueError(lib.ZSTD_getErrorName(code).decode())
        return code

    context = lib.ZSTD_createDStream()
    if not context:
        raise MemoryError("Cannot create Zstandard decoder")
    total = 0
    header = None
    header_bytes = bytearray()
    output_buffer = ctypes.create_string_buffer(256 * 1024)
    started = time.monotonic()
    try:
        check(lib.ZSTD_initDStream(context))
        remaining = 1
        with source.open("rb") as compressed:
            while chunk := compressed.read(256 * 1024):
                if on_chunk is not None:
                    on_chunk()
                input_buffer = ctypes.create_string_buffer(chunk)
                incoming = Input(ctypes.cast(input_buffer, ctypes.c_void_p), len(chunk), 0)
                while True:
                    outgoing = Output(ctypes.cast(output_buffer, ctypes.c_void_p), len(output_buffer), 0)
                    remaining = check(lib.ZSTD_decompressStream(context, ctypes.byref(outgoing), ctypes.byref(incoming)))
                    block = output_buffer.raw[:outgoing.pos]
                    if total + len(block) > max_bytes:
                        raise RuntimeError("Decompressed source exceeds configured output budget")
                    if header is None:
                        end = block.find(b"\n")
                        header_bytes.extend(block if end < 0 else block[:end])
                        if len(header_bytes) > MIB:
                            raise ValueError("Photon header exceeds the bounded header budget")
                        if end >= 0:
                            header = json.loads(header_bytes)
                            if header.get("type") != "NominatimDumpFile":
                                raise ValueError("Unexpected Photon dump header")
                            header_bytes.clear()
                    output.write(block)
                    total += len(block)
                    if incoming.pos == incoming.size and (remaining == 0 or outgoing.pos < len(output_buffer)):
                        break
            if remaining:
                raise ValueError("Truncated Zstandard input")
            if header is None:
                raise ValueError("Missing Photon dump header")
            output.flush()
        return {"bytes": total, "elapsed_seconds": time.monotonic() - started, "header": header}
    finally:
        lib.ZSTD_freeDStream(context)


def decompress(source: Path, destination: Path, max_bytes: int, min_free_bytes: int) -> dict:
    """Materialize only when explicitly requested; the import benchmark uses stream."""
    if destination.exists():
        raise ValueError(f"Refusing to replace existing output: {destination}")
    temporary = destination.with_name(destination.name + ".partial")

    def disk_guard() -> None:
        if shutil.disk_usage(destination.parent).free < min_free_bytes:
            raise RuntimeError("Disk free-space stop condition reached during decompression")

    try:
        with temporary.open("xb") as output:
            result = decode_zstd(source, output, max_bytes, disk_guard)
            os.fsync(output.fileno())
        os.link(temporary, destination)
        temporary.unlink()
        return result
    finally:
        if temporary.exists():
            temporary.unlink()


def stream(args: argparse.Namespace) -> dict:
    return decode_zstd(args.source, sys.stdout.buffer, args.max_output_gib * GIB)


def prepare(args: argparse.Namespace) -> dict:
    location = paths(args.root)
    if digest(location["jar"], "sha256") != JAR_SHA256:
        raise ValueError("Photon JAR checksum mismatch")
    if location["compressed"].stat().st_size != DUMP_BYTES or digest(location["compressed"], "md5") != DUMP_MD5:
        raise ValueError("Photon input changed or is incomplete; record/review a new source manifest")
    return decompress(location["compressed"], location["jsonl"], args.max_output_gib * GIB, args.min_free_gib * GIB)


def plan(args: argparse.Namespace) -> dict:
    root = args.root.resolve()
    if not root.is_absolute() or root == Path("/"):
        raise ValueError("A dedicated absolute benchmark root is required")
    location = paths(root)
    if args.uid == 0 or args.gid < 0:
        raise ValueError("Photon's embedded OpenSearch must run as a non-root user")
    if digest(location["jar"], "sha256") != JAR_SHA256:
        raise ValueError("Photon JAR checksum mismatch")
    data = location["data"]
    if args.stage == "import":
        if not location["jsonl"].is_file():
            raise ValueError("Run prepare before import")
        if data.exists() and any(data.iterdir()):
            raise ValueError("Photon import deletes indexes: dedicated data directory must be empty")
        data.mkdir(parents=True, exist_ok=True)
    elif not (data / "photon_data").is_dir():
        raise ValueError("Expected an imported photon_data directory")
    docker_args = [
        "--user", f"{args.uid}:{args.gid}", "--read-only", "--entrypoint", "/bin/sh" if args.stage == "import" else "java",
        "--tmpfs", "/tmp:rw,nosuid,size=512m", "--env", "HOME=/tmp", "--workdir", "/data",
        "--mount", f"type=bind,src={location['jar']},dst=/opt/photon.jar,readonly",
        "--mount", f"type=bind,src={data},dst=/data",
    ]
    command = ["-XX:ActiveProcessorCount=2", "-Duser.home=/tmp", "-Xms512m", "-Xmx2g", "-jar", "/opt/photon.jar", args.stage, "-data-dir", "/data"]
    if args.stage == "import":
        docker_args += ["--mount", f"type=bind,src={location['jsonl']},dst=/input/photon.jsonl,readonly"]
        command += ["-import-file", "/input/photon.jsonl", "-country-codes", "kz", "-languages", "kk,ru,en"]
        command = ["-c", IMPORT_WRAPPER, "photon-import", *command]
    else:
        command += ["-listen-ip", "0.0.0.0", "-listen-port", "2322", "-max-results", "5", "-max-reverse-results", "5", "-query-timeout", "3"]
    return {"image": args.image, "docker_args": docker_args, "command": command, "container_port": 2322 if args.stage == "serve" else None, "data_dir": str(data)}


def request(base: str, fixture: dict) -> dict:
    params = {"limit": 5, **fixture["params"]}
    url = base + fixture["path"] + "?" + urllib.parse.urlencode(params)
    started = time.perf_counter()
    try:
        with urllib.request.urlopen(url, timeout=5) as response:
            body = json.load(response)
            if body.get("type") != "FeatureCollection" or not isinstance(body.get("features"), list):
                raise ValueError("Expected a Photon GeoJSON FeatureCollection")
            features = body.get("features", [])
            return {"id": fixture["id"], "ok": True, "status": response.status, "elapsed_ms": (time.perf_counter() - started) * 1000, "feature_count": len(features), "results": [{"properties": feature.get("properties", {}), "geometry": feature.get("geometry", {})} for feature in features[:3]]}
    except (OSError, ValueError, urllib.error.URLError) as error:
        return {"id": fixture["id"], "ok": False, "elapsed_ms": (time.perf_counter() - started) * 1000, "error": str(error)}


def aggregate_latencies(samples: list[dict]) -> dict:
    """Include failed attempts in timings so errors cannot hide slow queries."""
    values = sorted(sample["elapsed_ms"] for sample in samples)
    successes = sum(bool(sample["ok"]) for sample in samples)
    return {
        "count": len(samples),
        "success_count": successes,
        "error_count": len(samples) - successes,
        "p50_ms": statistics.median(values) if values else None,
        "p95_ms": values[math.ceil(len(values) * .95) - 1] if values else None,
        "max_ms": values[-1] if values else None,
        "latency_scope": "all_attempts_including_failures",
    }


def benchmark(args: argparse.Namespace) -> dict:
    base = args.base_url.rstrip("/")
    parsed = urllib.parse.urlsplit(base)
    if parsed.scheme != "http" or parsed.hostname not in {"127.0.0.1", "localhost", "::1"} or parsed.path or parsed.query or parsed.fragment or parsed.username:
        raise ValueError("Benchmark requests must target a loopback HTTP endpoint")
    first_pass = [request(base, fixture) for fixture in FIXTURES]
    jobs = FIXTURES * args.repeats
    started = time.perf_counter()
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.concurrency) as executor:
        samples = list(executor.map(lambda fixture: request(base, fixture), jobs))
    elapsed = time.perf_counter() - started
    summary = {}
    for fixture in FIXTURES:
        values = sorted(sample["elapsed_ms"] for sample in samples if sample["id"] == fixture["id"] and sample["ok"])
        summary[fixture["id"]] = {"count": len(values), "p50_ms": statistics.median(values) if values else None, "p95_ms": values[math.ceil(len(values) * .95) - 1] if values else None}
    return {"fixture_count": len(FIXTURES), "concurrency": args.concurrency, "repeats": args.repeats, "first_pass": first_pass, "first_pass_aggregate": aggregate_latencies(first_pass), "warm_summary": summary, "warm_aggregate": aggregate_latencies(samples), "total_request_count": len(first_pass) + len(samples), "total_error_count": sum(not sample["ok"] for sample in first_pass + samples), "warm_failures": [sample for sample in samples if not sample["ok"]], "warm_elapsed_seconds": elapsed, "warm_requests_per_second": len(jobs) / elapsed, "cache_note": "First-pass means the first request per fixture in this run; process/OS caches may already be warm and host caches are never dropped."}


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="action", required=True)
    commands.add_parser("manifest")
    commands.add_parser("fixtures")
    streamed = commands.add_parser("stream")
    streamed.add_argument("--source", type=Path, required=True)
    # The regional source expands beyond 4 GiB before country filtering. This caps
    # bytes sent through the pipe, not resident memory or a materialized JSON file.
    streamed.add_argument("--max-output-gib", type=int, choices=range(1, 17), default=16)
    prep = commands.add_parser("prepare")
    prep.add_argument("--root", type=Path, required=True)
    prep.add_argument("--max-output-gib", type=int, default=8)
    prep.add_argument("--min-free-gib", type=int, default=20)
    docker = commands.add_parser("plan")
    docker.add_argument("--root", type=Path, required=True)
    docker.add_argument("--image", required=True)
    docker.add_argument("--uid", type=int, required=True)
    docker.add_argument("--gid", type=int, required=True)
    docker.add_argument("--stage", choices=("import", "serve"), required=True)
    queries = commands.add_parser("benchmark")
    queries.add_argument("--base-url", required=True)
    queries.add_argument("--repeats", type=int, choices=range(1, 21), default=8)
    queries.add_argument("--concurrency", type=int, choices=(1, 2), default=1)
    args = parser.parse_args()
    if args.action == "stream":
        try:
            result = stream(args)
        except (OSError, ValueError, RuntimeError, MemoryError) as error:
            print(json.dumps({"stage": "photon-stream", "ok": False, "error": str(error)}), file=sys.stderr, flush=True)
            if isinstance(error, BrokenPipeError):
                os._exit(1)
            raise SystemExit(1)
        print(json.dumps({"stage": "photon-stream", "ok": True, **result}), file=sys.stderr, flush=True)
        return
    if args.action == "manifest":
        result = manifest()
    elif args.action == "fixtures":
        result = FIXTURES
    elif args.action == "prepare":
        result = prepare(args)
    elif args.action == "plan":
        result = plan(args)
    else:
        result = benchmark(args)
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()

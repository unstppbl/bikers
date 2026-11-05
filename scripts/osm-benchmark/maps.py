#!/usr/bin/env python3
"""Inspect country map archives and verify bounded loopback HTTP Range delivery.

This is a benchmark helper, not an application map server. It does not download
data, start containers, or change host settings. HTTP timings include a fresh
connection for every request and exclude local byte verification. No OS caches
are evicted; a first pass must not be reported as a cold-cache measurement.
"""

import argparse
import concurrent.futures
import gzip
import hashlib
import io
import json
import math
from pathlib import Path
import random
import sqlite3
import struct
import time
import urllib.parse
import urllib.request


PLACES = {
    "almaty": (76.945, 43.238),
    "astana": (71.430, 51.128),
    "shymkent": (69.590, 42.320),
    "charyn": (79.080, 43.350),
    "aktau": (51.170, 43.650),
}
HEADER_FIELDS = (
    "root_offset", "root_length", "metadata_offset", "metadata_length",
    "leaf_offset", "leaf_length", "tile_offset", "tile_length",
    "addressed_tiles", "tile_entries", "tile_contents",
)


def sha256_file(path):
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(4 * 1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def pmtiles_header(path):
    with path.open("rb") as source:
        raw = source.read(127)
    if len(raw) != 127 or raw[:7] != b"PMTiles" or raw[7] != 3:
        raise ValueError("Expected a PMTiles v3 archive")
    result = dict(zip(HEADER_FIELDS, struct.unpack_from("<11Q", raw, 8)))
    result.update({
        "version": 3,
        "clustered": bool(raw[96]),
        "internal_compression": raw[97],
        "tile_compression": raw[98],
        "tile_type": raw[99],
        "minzoom": raw[100],
        "maxzoom": raw[101],
        "bounds": [value / 10_000_000 for value in struct.unpack_from("<4i", raw, 102)],
    })
    size = path.stat().st_size
    for prefix in ("root", "metadata", "leaf", "tile"):
        if result[prefix + "_offset"] + result[prefix + "_length"] > size:
            raise ValueError(f"PMTiles {prefix} section exceeds archive size")
    return result


def pmtiles_metadata(path, header):
    if header["metadata_length"] > 4 * 1024 * 1024:
        raise ValueError("Metadata exceeds this benchmark's 4 MiB input limit")
    with path.open("rb") as source:
        source.seek(header["metadata_offset"])
        raw = source.read(header["metadata_length"])
    if header["internal_compression"] == 2:
        with gzip.GzipFile(fileobj=io.BytesIO(raw)) as source:
            raw = source.read(16 * 1024 * 1024 + 1)
    elif header["internal_compression"] != 1:
        raise ValueError("Benchmark metadata supports uncompressed/gzip archives only")
    if len(raw) > 16 * 1024 * 1024:
        raise ValueError("Expanded metadata exceeds 16 MiB")
    return json.loads(raw)


def tile_xy(lon, lat, zoom):
    size = 1 << zoom
    x = int((lon + 180) / 360 * size)
    y = int((1 - math.asinh(math.tan(math.radians(lat))) / math.pi) / 2 * size)
    return x, y


def inspect(args):
    report = {"kind": "country-map-archive-inventory", "files": {}}
    if args.mbtiles:
        path = args.mbtiles.resolve(strict=True)
        with sqlite3.connect(path.as_uri() + "?mode=ro&immutable=1", uri=True) as database:
            metadata = dict(database.execute("SELECT name, value FROM metadata"))
            zooms = list(database.execute(
                "SELECT zoom_level, COUNT(*) FROM tiles GROUP BY zoom_level ORDER BY zoom_level"
            ))
            samples = []
            for name, (lon, lat) in PLACES.items():
                for zoom in (6, 10, 14):
                    x, y = tile_xy(lon, lat, zoom)
                    row = database.execute(
                        "SELECT tile_data FROM tiles WHERE zoom_level=? AND tile_column=? AND tile_row=?",
                        (zoom, x, (1 << zoom) - 1 - y),
                    ).fetchone()
                    sample = {"place": name, "z": zoom, "x": x, "y_xyz": y, "present": row is not None}
                    if row:
                        data = row[0]
                        sample.update(bytes=len(data), sha256=hashlib.sha256(data).hexdigest(), gzip=data[:2] == b"\x1f\x8b")
                    samples.append(sample)
        report["files"]["mbtiles"] = {
            "bytes": path.stat().st_size, "sha256": sha256_file(path),
            "metadata": metadata, "tiles_per_zoom": zooms, "samples": samples,
            "sample_limit": "Presence/hash checks only; not visual or road-quality validation",
        }
    if args.pmtiles:
        path = args.pmtiles.resolve(strict=True)
        header = pmtiles_header(path)
        report["files"]["pmtiles"] = {
            "bytes": path.stat().st_size, "sha256": sha256_file(path),
            "header": header, "metadata": pmtiles_metadata(path, header),
        }
    if not report["files"]:
        raise ValueError("Supply --mbtiles and/or --pmtiles")
    return report


def percentile(values, fraction):
    ordered = sorted(values)
    return ordered[max(0, math.ceil(len(ordered) * fraction) - 1)] if ordered else None


def range_benchmark(args):
    parsed = urllib.parse.urlsplit(args.url)
    if parsed.scheme not in ("http", "https") or parsed.hostname not in ("127.0.0.1", "localhost", "::1"):
        raise ValueError("Only an explicitly supplied loopback HTTP(S) URL is permitted")
    if parsed.username or parsed.password:
        raise ValueError("URL credentials are not permitted")
    if not 4 <= args.requests <= 256 or not 128 <= args.range_bytes <= 262144:
        raise ValueError("Use 4–256 requests and 128–262144 bytes per range")
    if not args.concurrency or any(value < 1 or value > 8 for value in args.concurrency):
        raise ValueError("Concurrency must be between 1 and 8")
    path = args.archive.resolve(strict=True)
    size = path.stat().st_size
    header = pmtiles_header(path)
    if size < args.range_bytes:
        raise ValueError("Archive is smaller than the requested range size")
    ranges = [(0, 127)]
    for prefix in ("root", "metadata"):
        if header[prefix + "_length"]:
            ranges.append((header[prefix + "_offset"], min(args.range_bytes, header[prefix + "_length"])))
    generator = random.Random(args.seed)
    while len(ranges) < args.requests:
        start = generator.randrange(header["tile_offset"], size - args.range_bytes + 1)
        ranges.append((start, args.range_bytes))
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

    def request_range(bounds):
        start, length = bounds
        end = start + length - 1
        request = urllib.request.Request(args.url, headers={"Range": f"bytes={start}-{end}", "Accept-Encoding": "identity"})
        began = time.perf_counter()
        try:
            with opener.open(request, timeout=10) as response:
                status = response.status
                content_range = response.headers.get("Content-Range")
                encoding = response.headers.get("Content-Encoding", "identity")
                data = response.read(length + 1)
            elapsed = (time.perf_counter() - began) * 1000
            if status != 206 or content_range != f"bytes {start}-{end}/{size}" or encoding != "identity":
                raise ValueError(f"Invalid range response: {status}, {content_range}, {encoding}")
            with path.open("rb") as source:
                source.seek(start)
                expected = source.read(length)
            if data != expected:
                raise ValueError("HTTP bytes differ from the local archive")
            return {"ms": elapsed, "bytes": len(data)}
        except Exception as error:
            return {"error": f"{type(error).__name__}: {error}"}

    runs = []
    for concurrency in args.concurrency:
        for phase in ("first-pass", "repeat-pass"):
            began = time.perf_counter()
            with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as executor:
                results = list(executor.map(request_range, ranges))
            elapsed = time.perf_counter() - began
            values = [result["ms"] for result in results if "ms" in result]
            failures = [result["error"] for result in results if "error" in result]
            runs.append({
                "concurrency": concurrency, "phase": phase, "requests": len(results),
                "verified": len(values), "errors": len(failures), "error_examples": failures[:5],
                "wall_seconds": elapsed, "p50_ms": percentile(values, .5),
                "p95_ms": percentile(values, .95), "max_ms": max(values) if values else None,
                "response_bytes": sum(result.get("bytes", 0) for result in results),
            })
    return {
        "kind": "loopback-pmtiles-http-range-delivery", "archive_bytes": size,
        "requests_per_run": len(ranges), "range_bytes": args.range_bytes, "seed": args.seed,
        "limitations": [
            "No OS cache eviction; first-pass does not mean cold cache",
            "Later concurrency levels reuse the same archive ranges",
            "Fresh HTTP connection per request; local byte comparison excluded from latency",
            "Measures file-range transport, not browser render, PMTiles client lookup, or Internet latency",
        ],
        "runs": runs,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path)
    commands = parser.add_subparsers(dest="command", required=True)
    inventory = commands.add_parser("inspect")
    inventory.add_argument("--mbtiles", type=Path)
    inventory.add_argument("--pmtiles", type=Path)
    inventory.set_defaults(run=inspect)
    benchmark = commands.add_parser("range-bench")
    benchmark.add_argument("--archive", type=Path, required=True)
    benchmark.add_argument("--url", required=True)
    benchmark.add_argument("--requests", type=int, default=64)
    benchmark.add_argument("--range-bytes", type=int, default=65536)
    benchmark.add_argument("--concurrency", type=int, nargs="+", default=[1, 4])
    benchmark.add_argument("--seed", type=int, default=20261105)
    benchmark.set_defaults(run=range_benchmark)
    args = parser.parse_args()
    result = args.run(args)
    rendered = json.dumps(result, ensure_ascii=False, indent=2) + "\n"
    if args.output:
        args.output.write_text(rendered)
    else:
        print(rendered, end="")
    if any(run.get("errors") for run in result.get("runs", [])):
        raise SystemExit(1)


if __name__ == "__main__":
    main()

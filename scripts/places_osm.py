#!/usr/bin/env python3
"""Export a bounded, attributed POI snapshot from an already verified OSM file.

Run inside the existing capped/offline GIS job. This command does not download,
contact Overpass, modify the input, run containers or import a database.
"""
import argparse
from array import array
from collections import Counter
from datetime import datetime, timezone
import hashlib
import importlib.metadata
import json
import math
import os
from pathlib import Path, PurePosixPath
import platform
import re
import shutil
import stat
import sys
import tempfile
from urllib.parse import urlsplit
import zipfile

PARSER_VERSION = '4.3.2'
MAX_BYTES = 32 * 1024**2
MAX_ROWS = 50000
MAX_INPUT_BYTES = 300000000
MAX_WAYS = 50000
MAX_WAY_NODES = 2000
MAX_REFERENCES = 2000000
MAX_NEEDED_NODES = 500000
MAX_WAY_METADATA = 32 * 1024**2

_WHEELS = (
    ('osmium-4.3.2-cp312-cp312-manylinux_2_27_x86_64.manylinux_2_28_x86_64.whl',
     'fc/9e/b16036f4e63ba8fddaf149668299659cab1b872b4fa1712863d702c6cf02',
     '604a3145aa755790b91e3eefac7ddbb74d31f188fddbbbb988877cba50e88eb7', 1876654),
    ('requests-2.34.2-py3-none-any.whl', 'a0/f4/c67b0b3f1b9245e8d266f0f112c500d50e5b4e83cb6f3b71b6528104182a',
     '2a0d60c172f83ac6ab31e4554906c0f3b3588d37b5cb939b1c061f4907e278e0', 73075),
    ('urllib3-2.8.0-py3-none-any.whl', '92/9d/c4e665119135114480843e7ab388fa94d8480650450e6f8e26b70d323a4c',
     '0cf3cae568d36aa9576b28dfb35f11328f1cb974ca7647d9475ebb86c75ac6e3', 135717),
    ('idna-3.20-py3-none-any.whl', '58/a2/bb081bab032533a855d44de1d56f8e8426114ff1ba5d1f07a438a0a654f8',
     'ab7ae7122974553370f0bdb919e1a960b2cd1bc1ef0276416d896db81c14582c', 69583),
    ('charset_normalizer-3.5.2-py3-none-any.whl', 'fc/ad/d07d7862a62ffa6d79d68074d14823243dd235a77c45262acbf6adeb28bf',
     'b6b751274acb69d77b3323d6b7dbaa3c7fdfc1eb829b7eb61d262f32e1af9685', 68872),
    ('certifi-2026.10.23-py3-none-any.whl', '32/d1/a3652fe2c19134d0d15922177821d863cbd7e842d53a21bbe7e8e3a1b5ac',
     'e9b54de76e530bb6cf0ccd036787103acb58fd42ba930ea8bf948ddbaa2f61d1', 135082),
)


def wheel_manifest():
    return [{'filename': name, 'url': 'https://files.pythonhosted.org/packages/' + prefix + '/' + name,
             'sha256': sha, 'bytes': size} for name, prefix, sha, size in _WHEELS]


def sha256(path):
    result = hashlib.sha256()
    with path.open('rb') as source:
        for block in iter(lambda: source.read(1024 * 1024), b''):
            result.update(block)
    return result.hexdigest()


def require_runtime():
    if sys.version_info[:2] != (3, 12) or platform.system() != 'Linux' or platform.machine() not in ('x86_64', 'amd64'):
        raise RuntimeError('Pinned parser requires Linux/amd64 CPython 3.12')


def prepare_site(wheels_dir, site_dir):
    require_runtime()
    if site_dir.exists() or site_dir.is_symlink():
        raise ValueError('Refusing to replace an existing parser runtime')
    manifest = wheel_manifest()
    for item in manifest:
        path = wheels_dir / item['filename']
        if path.is_symlink() or not path.is_file() or path.stat().st_size != item['bytes'] or sha256(path) != item['sha256']:
            raise ValueError('Parser wheel does not match its pinned size/SHA256')
    staging = Path(tempfile.mkdtemp(prefix='.' + site_dir.name + '-', dir=site_dir.parent))
    total, count = 0, 0
    try:
        for item in manifest:
            with zipfile.ZipFile(wheels_dir / item['filename']) as archive:
                for member in archive.infolist():
                    path = PurePosixPath(member.filename)
                    kind = stat.S_IFMT(member.external_attr >> 16)
                    if path.is_absolute() or '..' in path.parts or '\\' in member.filename or kind not in (0, stat.S_IFREG, stat.S_IFDIR):
                        raise ValueError('Wheel contains an unsafe path or non-regular member')
                    if member.is_dir():
                        continue
                    total += member.file_size; count += 1
                    if member.file_size > 16 * 1024**2 or total > 64 * 1024**2 or count > 10000:
                        raise ValueError('Expanded parser runtime exceeds its bounded budget')
                    target = staging / Path(*path.parts)
                    target.parent.mkdir(parents=True, exist_ok=True)
                    with archive.open(member) as incoming, target.open('xb') as outgoing:
                        shutil.copyfileobj(incoming, outgoing, 1024 * 1024)
                    target.chmod(0o755 if member.external_attr >> 16 & 0o111 else 0o644)
        staging.rename(site_dir)
    finally:
        if staging.exists():
            shutil.rmtree(staging)
    return {'site_dir': str(site_dir), 'files': count, 'expanded_bytes': total, 'wheels': manifest}


def load_osmium():
    require_runtime()
    import osmium
    if importlib.metadata.version('osmium') != PARSER_VERSION:
        raise RuntimeError('Unexpected pyosmium version; use the pinned runtime')
    return osmium


def clipped(value, maximum):
    value = ''.join(' ' if ord(char) < 32 or ord(char) == 127 else char for char in str(value or ''))
    value = ' '.join(value.split())
    return value.encode('utf-8')[:maximum].decode('utf-8', errors='ignore').strip()


def category(tags):
    amenity, shop = tags.get('amenity', ''), tags.get('shop', '')
    if amenity == 'fuel': result = 'fuel'
    elif shop == 'motorcycle_repair' or (shop == 'car_repair' and
            (tags.get('motorcycle') == 'yes' or tags.get('service:motorcycle:repair') == 'yes')): result = 'workshop'
    elif shop == 'motorcycle': result = 'motorcycle_shop'
    elif shop == 'tyres': result = 'tires'
    elif amenity == 'parking': result = 'parking'
    elif amenity == 'meeting_point': result = 'meeting_spot'
    else: return None
    for key in ('access', 'motor_vehicle', 'motorcycle'):
        if tags.get(key, '') not in ('', 'yes', 'designated', 'permissive', 'customers', 'destination'):
            return None
        if tags.get(key + ':conditional', ''):
            return None
    if tags.get('service') in ('driveway', 'parking_aisle'):
        return None
    if any(tags.get(prefix) == 'yes' or tags.get(prefix + ':amenity') or tags.get(prefix + ':shop')
           for prefix in ('disused', 'abandoned', 'demolished', 'construction', 'proposed', 'removed')):
        return None
    if result == 'parking':
        if tags.get('parking') in ('private', 'residential', 'residents', 'garage_boxes', 'sheds'):
            return None
        if tags.get('building') in ('house', 'apartments', 'residential', 'dormitory', 'garage', 'garages') or tags.get('landuse') == 'residential':
            return None
        if tags.get('parking:condition:default', '') or tags.get('parking:restriction', ''):
            return None
        if tags.get('access') not in ('yes', 'designated', 'permissive', 'customers') and tags.get('motorcycle') not in ('yes', 'designated'):
            return None
    return result


def fields(tags):
    selected = category(tags)
    name = next((clipped(tags.get(key), 200) for key in ('name', 'name:kk', 'name:ru', 'name:en', 'brand', 'operator')
                 if clipped(tags.get(key), 200)), '')
    if not selected or not name:
        return None
    website = clipped(tags.get('website') or tags.get('contact:website'), 2000)
    try:
        parsed = urlsplit(website)
        if parsed.scheme not in ('http', 'https') or not parsed.hostname or parsed.username or parsed.password or any(c.isspace() for c in website) or re.search(r'%(?![0-9a-fA-F]{2})', website):
            website = ''
        else:
            website = parsed.geturl()
        _ = parsed.port
    except ValueError:
        website = ''
    address = tags.get('addr:full') or ' '.join(filter(None, (tags.get('addr:street') or tags.get('addr:place'), tags.get('addr:housenumber'))))
    return {'name': name, 'category': selected, 'city': clipped(tags.get('addr:city'), 160),
            'address': clipped(address, 500), 'description': clipped(tags.get('description'), 2000),
            'website': website, 'phone': clipped(tags.get('phone') or tags.get('contact:phone'), 100),
            'opening_hours': clipped(tags.get('opening_hours'), 300)}


def valid_coordinate(point):
    return len(point) == 2 and all(math.isfinite(value) for value in point) and 46 <= point[0] <= 87.5 and 40.5 <= point[1] <= 55.5


def representative(points, closed):
    if len(points) < 2 or not all(valid_coordinate(point) for point in points):
        return None
    if max(p[0] for p in points) - min(p[0] for p in points) > .25 or max(p[1] for p in points) - min(p[1] for p in points) > .25:
        return None
    if closed and len(points) >= 4:
        origin = points[0]
        shifted = [(x - origin[0], y - origin[1]) for x, y in points]
        crosses = [a[0] * b[1] - b[0] * a[1] for a, b in zip(shifted, shifted[1:])]
        area = sum(crosses)
        if abs(area) > 1e-14:
            centroid = tuple(sum((a[axis] + b[axis]) * cross for a, b, cross in zip(shifted, shifted[1:], crosses)) / (3 * area) + origin[axis]
                             for axis in (0, 1))
            inside = False
            for (x1, y1), (x2, y2) in zip(points, points[1:]):
                if (y1 > centroid[1]) != (y2 > centroid[1]) and centroid[0] < (x2 - x1) * (centroid[1] - y1) / (y2 - y1) + x1:
                    inside = not inside
            if inside:
                return centroid
            # A concave polygon's centroid can fall outside. Use a point on its
            # longest boundary segment; this is not an inferred entrance.
            first, second = max(zip(points, points[1:]), key=lambda pair: math.dist(*pair))
            return ((first[0] + second[0]) / 2, (first[1] + second[1]) / 2)
    distances = [math.hypot((b[0] - a[0]) * math.cos(math.radians(a[1])), b[1] - a[1]) for a, b in zip(points, points[1:])]
    remaining = sum(distances) / 2
    if remaining == 0:
        return None
    for first, second, length in zip(points, points[1:], distances):
        if length and remaining <= length:
            portion = remaining / length
            return tuple(a + (b - a) * portion for a, b in zip(first, second))
        remaining -= length
    return None


def snapshot_date(header, expected=None):
    if header.has_multiple_object_versions:
        raise ValueError('History/diff files are not supported for a current POI snapshot')
    stamp = header.get('osmosis_replication_timestamp', '') or header.get('timestamp', '')
    try:
        parsed = datetime.fromisoformat(stamp.replace('Z', '+00:00'))
        if parsed.tzinfo is None:
            raise ValueError('Snapshot timestamp requires a timezone')
        date = parsed.astimezone(timezone.utc).date().isoformat()
    except (ValueError, AttributeError) as error:
        raise ValueError('OSM header lacks a valid snapshot timestamp') from error
    if expected and date != expected:
        raise ValueError('Expected source date differs from the OSM snapshot header')
    return date


class NDJSON:
    def __init__(self, output):
        if output.exists() or output.is_symlink():
            raise ValueError('Refusing to replace an existing POI dataset')
        descriptor, name = tempfile.mkstemp(prefix='.' + output.name + '-', dir=output.parent)
        self.path, self.output, self.file = Path(name), output, os.fdopen(descriptor, 'wb')
        self.bytes, self.rows, self.seen, self.categories, self.hash = 0, 0, set(), Counter(), hashlib.sha256()

    def add(self, kind, object_id, data, point, date):
        if not valid_coordinate(point) or object_id <= 0:
            return
        source = f'{kind}/{object_id}'
        if source in self.seen:
            raise ValueError('Duplicate OSM object in snapshot')
        row = {**data, 'source_ref': source, 'source_date': date, 'lon': round(point[0], 7), 'lat': round(point[1], 7)}
        encoded = (json.dumps(row, ensure_ascii=False, allow_nan=False, separators=(',', ':')) + '\n').encode('utf-8')
        if self.rows >= MAX_ROWS or self.bytes + len(encoded) > MAX_BYTES:
            raise ValueError('POI dataset exceeds 50,000 rows or 32 MiB; no partial export is published')
        self.file.write(encoded); self.hash.update(encoded)
        self.bytes += len(encoded); self.rows += 1; self.seen.add(source); self.categories[data['category']] += 1

    def publish(self):
        if not self.rows:
            raise ValueError('No eligible named public POIs found; refusing an empty initial dataset')
        self.file.flush(); os.fchmod(self.file.fileno(), 0o644); os.fsync(self.file.fileno()); self.file.close()
        os.link(self.path, self.output)  # atomic no-overwrite publication of this new output only

    def close(self):
        self.file.close()
        self.path.unlink(missing_ok=True)


def extract(source, output, expected_sha256, expected_date=None):
    if source.is_symlink() or not source.is_file() or source.stat().st_size > MAX_INPUT_BYTES:
        raise ValueError('Expected a regular country extract no larger than 300 MB')
    before = source.stat()
    if not expected_sha256 or len(expected_sha256) != 64 or sha256(source) != expected_sha256:
        raise ValueError('OSM input SHA256 mismatch')
    hashed = source.stat()
    if (before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns, before.st_ctime_ns) != (hashed.st_dev, hashed.st_ino, hashed.st_size, hashed.st_mtime_ns, hashed.st_ctime_ns):
        raise ValueError('OSM source changed while its hash was checked')
    osmium = load_osmium()
    needed, ways = set(), {}
    stats = Counter()
    metadata_bytes = 0
    reference_count = 0
    pool = osmium.io.ThreadPool(1, 2)

    class Ways(osmium.SimpleHandler):
        def way(self, way):
            nonlocal metadata_bytes, reference_count
            if not category(way.tags):
                return
            data = fields(way.tags)
            if data is None:
                stats['unnamed_ways'] += 1; return
            if len(way.nodes) < 2 or len(way.nodes) > MAX_WAY_NODES:
                stats['oversized_or_degenerate_ways'] += 1; return
            references = array('q', (node.ref for node in way.nodes))
            if way.id in ways:
                raise ValueError('Duplicate candidate way in snapshot')
            new_nodes = set(references) - needed
            metadata_bytes += len(json.dumps(data, ensure_ascii=False).encode())
            reference_count += len(references)
            if len(ways) >= MAX_WAYS or len(needed) + len(new_nodes) > MAX_NEEDED_NODES or reference_count > MAX_REFERENCES or metadata_bytes > MAX_WAY_METADATA:
                raise ValueError('Candidate ways/node references exceed the bounded extraction budget')
            needed.update(new_nodes)
            ways[way.id] = (data, references)

    with osmium.io.Reader(str(source), osmium.osm.WAY, thread_pool=pool) as reader:
        date = snapshot_date(reader.header(), expected_date)
        osmium.apply(reader, Ways())
    locations = {}
    writer = NDJSON(output)

    class Nodes(osmium.SimpleHandler):
        def node(self, node):
            if not node.location.valid():
                return
            point = (node.location.lon, node.location.lat)
            if not valid_coordinate(point):
                return
            if node.id in needed:
                if node.id in locations:
                    raise ValueError('Duplicate required node in snapshot')
                locations[node.id] = point
            if category(node.tags):
                data = fields(node.tags)
                if data is None:
                    stats['unnamed_nodes'] += 1
                else:
                    writer.add('node', node.id, data, point, date)

    try:
        with osmium.io.Reader(str(source), osmium.osm.NODE, thread_pool=pool) as reader:
            if snapshot_date(reader.header(), expected_date) != date:
                raise ValueError('OSM snapshot changed between passes')
            osmium.apply(reader, Nodes())
        for object_id, (data, references) in ways.items():
            if any(node_id not in locations for node_id in references):
                stats['ways_missing_nodes'] += 1; continue
            point = representative([locations[node_id] for node_id in references], references[0] == references[-1])
            if point is None:
                stats['ways_invalid_geometry'] += 1; continue
            writer.add('way', object_id, data, point, date)
        after = source.stat()
        if (before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns, before.st_ctime_ns) != (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns, after.st_ctime_ns):
            raise ValueError('OSM source changed during extraction')
        writer.publish()
        return {'output': str(output), 'rows': writer.rows, 'bytes': writer.bytes, 'sha256': writer.hash.hexdigest(),
                'source_sha256': expected_sha256, 'source_date': date, 'categories': dict(writer.categories),
                'candidate_ways': len(ways), 'needed_nodes': len(needed), 'node_references': reference_count, 'skipped': dict(stats),
                'attribution': '© OpenStreetMap contributors; ODbL 1.0; https://www.openstreetmap.org/copyright',
                'limitations': 'Nodes and ways only; way positions are representative, not verified entrances; no availability or business verification inferred'}
    finally:
        writer.close()


def probe():
    osmium = load_osmium()
    seen = []
    class Handler(osmium.SimpleHandler):
        def node(self, node):
            seen.append((node.id, node.location.lon, node.location.lat))
    pool = osmium.io.ThreadPool(1, 2)
    with osmium.io.Reader(osmium.io.FileBuffer(b'<osm version="0.6"><node id="1" lon="76.95" lat="43.24"/></osm>', 'osm'),
                          osmium.osm.NODE, thread_pool=pool) as reader:
        osmium.apply(reader, Handler())
    if seen != [(1, 76.95, 43.24)]:
        raise RuntimeError('Native parser probe did not return expected geometry')
    return {'parser': PARSER_VERSION, 'python': platform.python_version(), 'machine': platform.machine(), 'native_probe': 'passed'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    commands.add_parser('manifest'); commands.add_parser('probe')
    prepare = commands.add_parser('prepare')
    prepare.add_argument('--wheels-dir', type=Path, required=True)
    prepare.add_argument('--site-dir', type=Path, required=True)
    export = commands.add_parser('extract')
    export.add_argument('--input', type=Path, required=True)
    export.add_argument('--output', type=Path, required=True)
    export.add_argument('--sha256', required=True)
    export.add_argument('--source-date')
    args = parser.parse_args()
    if args.command == 'manifest': result = {'parser': PARSER_VERSION, 'wheels': wheel_manifest()}
    elif args.command == 'probe': result = probe()
    elif args.command == 'prepare': result = prepare_site(args.wheels_dir, args.site_dir)
    else: result = extract(args.input, args.output, args.sha256, args.source_date)
    print(json.dumps(result, ensure_ascii=False))


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, RuntimeError, zipfile.BadZipFile) as error:
        print(f'OSM place extraction stopped: {error}', file=sys.stderr)
        sys.exit(1)

#!/usr/bin/env python3
"""Stage verified GIS data serially; activation is an explicit operator command.

No timer, image pull, or implicit rebuild is installed. Private job receipts reuse
the benchmark's tested host guard, with tmpfs controls and an independent lease.
"""
import argparse
from concurrent.futures import ThreadPoolExecutor
from contextlib import contextmanager
import fcntl
import hashlib
import ipaddress
import json
import math
import os
from pathlib import Path, PurePosixPath
import re
import random
import shutil
import signal
import subprocess
import sys
import tarfile
import tempfile
import threading
import time
import urllib.parse
import urllib.request
import uuid

# Source integrity checks must not be invalidated by our own helper imports.
sys.dont_write_bytecode = True
TOOLS = Path(__file__).resolve().parent
sys.path.insert(0, str(TOOLS / 'osm-benchmark'))
import run as guard
import valhalla
import photon

DEFAULT_RUNTIME = Path('/home/user/apps/bikerskz-gis')
COMPOSE = TOOLS.parent / 'deploy/gis/compose.yml'
MAP_SHA256 = '2583113c45ca582906e9e92c343f1c5a11ff08d6292c1eb8137cb0e764adb25c'
ASSETS = {
    'fonts.tar.gz': {
        'url': 'https://github.com/versatiles-org/versatiles-fonts/releases/download/v3.0.0/noto_sans.tar.gz',
        'sha256': 'f58be6be2eb3d127b6ff1c90d828b69ff4807d1d88cd13a43930943fda0e334e',
        'bytes': 31304823},
    'sprites.tar.gz': {
        'url': 'https://github.com/versatiles-org/versatiles-style/releases/download/v8.1.1/sprites.tar.gz',
        'sha256': '2d3266d99a8ce96f56d64f67ba2dbd3cf5d7547a5f9ade3f0e31f949188358de',
        'bytes': 855499},
    'noto-license.txt': {
        'url': 'https://raw.githubusercontent.com/versatiles-org/versatiles-fonts/v3.0.0/fonts/Noto%20Sans/LICENSE',
        'sha256': '788cca515c21cc0870976f29b80e19873c81448caa7fb213c2d37328466516ef', 'bytes': 4302},
    'noto-copyright-ofl.txt': {
        'url': 'https://raw.githubusercontent.com/google/fonts/c92b209a838649c205a1565b5f28c1973e049659/ofl/notosans/OFL.txt',
        'sha256': 'cee9892f9f0cc8fe882c9e9537ee6a89621d86ee7ceaf70b02e2b2b1c25c061a', 'bytes': 4396},
    'sprites-cc0.txt': {
        'url': 'https://raw.githubusercontent.com/versatiles-org/versatiles-style/v8.1.1/icons/LICENSE.md',
        'sha256': '09aa4b5a472a09d5570f706c98b6d2cd106ad774cd1fbe68880c67ca1422a604', 'bytes': 6934},
    'style-mit.txt': {
        'url': 'https://raw.githubusercontent.com/versatiles-org/versatiles-style/v8.1.1/LICENSE.md',
        'sha256': 'e2b5ef416c74e9d66dfa3469ee0a476fd09dd1bdd61a1f8e3e9f4885ba57ff19', 'bytes': 1082},
}


def execute(args, **kwargs):
    result = subprocess.run(args, text=True, capture_output=True, timeout=kwargs.pop('timeout', 60), **kwargs)
    if result.returncode:
        raise RuntimeError(f'{args[0]} failed (exit {result.returncode}); inspect private receipts')
    return result.stdout.strip()


def require_clean_source():
    checkout = TOOLS.parent
    if execute(['git', 'status', '--porcelain', '--untracked-files=all'], cwd=checkout):
        raise RuntimeError('GIS operations require the clean pushed Jarvis checkout')
    if execute(['git', 'rev-parse', 'HEAD'], cwd=checkout) != execute(['git', 'rev-parse', 'origin/main'], cwd=checkout):
        raise RuntimeError('GIS operations require the reviewed origin/main revision')


def digest(path):
    h = hashlib.sha256()
    with Path(path).open('rb') as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b''):
            h.update(chunk)
    return h.hexdigest()


def copy_verified(source, target, expected_sha256, max_bytes=3 * guard.GIB):
    """Hash the bytes actually staged and bound growth while the source is read."""
    h, total = hashlib.sha256(), 0
    with source.open('rb') as incoming, target.open('xb') as outgoing:
        while block := incoming.read(1024 * 1024):
            total += len(block)
            if total > max_bytes:
                raise ValueError('Input grew beyond the bounded refresh budget')
            outgoing.write(block)
            h.update(block)
    if h.hexdigest() != expected_sha256:
        raise ValueError('Staged input checksum mismatch')


def validate_id(value):
    if not re.fullmatch(r'[a-z0-9][a-z0-9-]{0,63}', value):
        raise ValueError('Dataset ID must contain 1–64 lowercase letters, numbers or hyphens')
    return value


def compose(runtime):
    return ['docker', 'compose', '--project-directory', str(COMPOSE.parent), '--env-file',
            str(runtime / '.env'), '-p', 'bkgis', '-f', str(COMPOSE)]


def atomic_env(path, values):
    text = ''.join(f'{key}={value}\n' for key, value in sorted(values.items()))
    if any('\n' in str(value) or '\r' in str(value) or not re.fullmatch(r'[A-Z_]+', key)
           for key, value in values.items()):
        raise ValueError('Invalid runtime environment entry')
    descriptor, name = tempfile.mkstemp(prefix='.env-', dir=path.parent)
    try:
        with os.fdopen(descriptor, 'w') as output:
            output.write(text)
            output.flush()
            os.fsync(output.fileno())
        os.replace(name, path)
    finally:
        if os.path.exists(name):
            os.unlink(name)


def read_env(path):
    if path.is_symlink() or not path.is_file() or path.stat().st_mode & 0o077:
        raise ValueError('GIS environment must be a private regular file')
    return dict(line.split('=', 1) for line in path.read_text().splitlines() if line)


def stop_gis(runtime, services):
    if services:
        execute(compose(runtime) + ['stop', '--timeout', '15', *services], timeout=90)


def install_abort_cleanup(job):
    """An aborted startup/cutover stops only its explicitly named GIS services."""
    original = guard.stop_owned

    def cleanup(root):
        try:
            original(root)
        finally:
            if (root / 'abort.json').exists():
                plan = json.loads((root / 'guard-plan.json').read_text())
                stop_gis(Path(plan['runtime']), plan['abort_services'])
    guard.stop_owned = cleanup
    return original


def interrupted(signum, _frame):
    raise RuntimeError(f'GIS job interrupted by signal {signum}')


def parent_identity(pid):
    # The final ')' terminates comm even if the process name contains spaces.
    return Path(f'/proc/{pid}/stat').read_text().rsplit(')', 1)[1].split()[19]


def child_watch(job, seconds):
    install_abort_cleanup(job)
    guard.watch(job, seconds)


def child_lease(job, seconds, pid, identity):
    install_abort_cleanup(job)
    deadline = time.monotonic() + seconds
    try:
        while not (job / 'finished.json').exists():
            guard.ensure_watch(job)
            if parent_identity(pid) != identity:
                raise RuntimeError('GIS launcher process identity changed')
            if time.monotonic() > deadline:
                raise RuntimeError('GIS independent lease expired')
            time.sleep(2)
    except BaseException as error:
        if not (job / 'abort.json').exists():
            guard.save(job / 'abort.json', {'time': time.time(), 'reason': str(error)})
        raise
    finally:
        guard.stop_owned(job)
        if (job / 'abort.json').exists():
            guard.save(job / 'finished.json', {'lease_cleanup': True, 'failed': True})


def stop_child(process):
    try:
        process.wait(timeout=40)
    except subprocess.TimeoutExpired:
        os.killpg(process.pid, signal.SIGTERM)
        try:
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            process.wait(timeout=10)


@contextmanager
def serial(runtime):
    os.umask(0o077)
    if os.getuid() != 1000:
        raise RuntimeError('Run GIS operations as the existing benchmark UID 1000')
    runtime.mkdir(parents=True, mode=0o700, exist_ok=True)
    with (runtime / '.refresh.lock').open('a') as lock, Path('/home/user/apps/bikerskz/.deploy.lock').open('a') as app_lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        fcntl.flock(app_lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        yield


@contextmanager
def guarded_job(runtime, *, abort_services=(), excluded=(), seconds=7200):
    job = runtime / 'jobs' / ('bikers-osm-kz-' + uuid.uuid4().hex)
    job.mkdir(parents=True, mode=0o700)
    for folder in ('inputs', 'data', 'logs', 'results', 'specs'):
        (job / folder).mkdir(mode=0o700)
    readings = []
    for _ in range(12):
        reading = guard.probe()
        if not reading['okay'] or reading['seconds'] > .25:
            raise RuntimeError('Existing app failed the strict 250 ms baseline')
        readings.append(reading['seconds'])
        time.sleep(1)
    sample = guard.host(job)
    if sample['available'] < 8 * guard.GIB or sample['disk_free'] < 25 * guard.GIB:
        raise RuntimeError('GIS job requires 8 GiB available RAM and 25 GiB free disk before starting')
    originals = {key: value for key, value in guard.containers().items() if key not in excluded}
    guard.save(job / 'baseline.json', {'time': time.time(), 'host': sample, 'containers': originals,
               'readiness_samples': readings, 'readiness_p95': max(readings),
               'readiness_limit': .25, 'fail_fast_readiness': True})
    guard.save(job / 'guard-plan.json', {'runtime': str(runtime), 'abort_services': list(abort_services)})
    controls = Path(tempfile.mkdtemp(prefix='bkgis-', dir='/dev/shm'))
    names = ('watch.json', 'abort.json', 'finished.json', 'samples.jsonl')
    for name in names:
        (job / name).symlink_to(controls / name)
    (controls / 'samples.jsonl').touch()
    processes, handles = [], []
    old_cleanup = install_abort_cleanup(job)
    try:
        watch_log = (job / 'logs/watch.log').open('w'); handles.append(watch_log)
        processes.append(subprocess.Popen([sys.executable, str(Path(__file__).resolve()), '_watch', str(job), str(seconds)],
            stdout=watch_log, stderr=subprocess.STDOUT, start_new_session=True))
        deadline = time.monotonic() + 15
        while not (job / 'watch.json').exists():
            if processes[0].poll() is not None or time.monotonic() > deadline:
                raise RuntimeError('GIS watchdog failed to start')
            time.sleep(.1)
        guard.ensure_watch(job)
        lease_log = (job / 'logs/lease.log').open('w'); handles.append(lease_log)
        processes.append(subprocess.Popen([sys.executable, str(Path(__file__).resolve()), '_lease', str(job),
            str(seconds), str(os.getpid()), parent_identity(os.getpid())],
            stdout=lease_log, stderr=subprocess.STDOUT, start_new_session=True))
        yield job
        guard.ensure_watch(job)
        guard.finish(job)
        if (job / 'abort.json').exists():
            raise RuntimeError('GIS guard aborted before completion; inspect the first abort receipt')
    except BaseException as error:
        if not (job / 'abort.json').exists():
            guard.save(job / 'abort.json', {'time': time.time(), 'reason': str(error)})
        raise
    finally:
        failed = sys.exc_info()[0] is not None
        cleanup_error = None
        try:
            guard.stop_owned(job)
        except BaseException as error:
            cleanup_error = error
            if not (job / 'abort.json').exists():
                guard.save(job / 'abort.json', {'time': time.time(), 'reason': str(error)})
        finally:
            if not (job / 'finished.json').exists():
                guard.save(job / 'finished.json', {'time': time.time(), 'failed': (job / 'abort.json').exists()})
            children_stopped = True
            for process in processes:
                try:
                    stop_child(process)
                except BaseException as error:
                    children_stopped = False
                    cleanup_error = cleanup_error or error
            guard.stop_owned = old_cleanup
            for handle in handles:
                handle.close()
            if children_stopped:
                for name in names:
                    link = job / name
                    content = link.read_bytes() if link.exists() else None
                    link.unlink()
                    if content is not None:
                        link.write_bytes(content)
                shutil.rmtree(controls)
            if cleanup_error and not failed:
                raise cleanup_error


def stage(job, name, command, mounts=(), memory='1g', cpus='1.0', seconds=3600, network='none', environment=None):
    spec = {'name': name, 'image': valhalla.IMAGE, 'memory': memory, 'cpus': cpus,
            'read_bps': '5mb', 'write_bps': '5mb', 'timeout': seconds, 'network': network,
            'args': ['--pull=never', '--read-only', '--user', '1000:1000', '--env', 'HOME=/tmp', '--entrypoint', command[0]],
            'command': command[1:]}
    for source, target, readonly in mounts:
        spec['args'] += ['--mount', f'type=bind,source={source},target={target}' + (',readonly' if readonly else '')]
    for key, value in (environment or {}).items():
        spec['args'] += ['--env', f'{key}={value}']
    path = job / 'specs' / (name + '.json')
    guard.save(path, spec)
    guard.stage(job, path)


def safe_name(name):
    path = PurePosixPath(name)
    if path.is_absolute() or '..' in path.parts:
        raise ValueError('Archive contains an unsafe path')
    return path


def regular_member(archive, member):
    seen = set()
    while member.islnk():
        if member.name in seen or len(seen) >= 16:
            raise ValueError('Archive hardlink cycle')
        seen.add(member.name)
        safe_name(member.linkname)
        member = archive.getmember(member.linkname)
    if not member.isfile() or member.size > 8 * 1024**2:
        raise ValueError('Expected a bounded regular archive member')
    return member


def extract_assets(fonts, sprites, target):
    """Copy validated bytes, including resolved tar hardlinks, into regular files."""
    copied = []
    for kind, filename in [('fonts', fonts), ('sprites', sprites)]:
        total = 0
        with tarfile.open(filename, 'r:gz') as archive:
            for member in archive.getmembers():
                path = safe_name(member.name)
                if member.issym():
                    raise ValueError('Archive symlinks are forbidden')
                relative = None
                if kind == 'fonts' and len(path.parts) >= 2 and path.parts[-2] == 'noto_sans_regular' and re.fullmatch(r'\d+-\d+\.pbf', path.name):
                    relative = Path('fonts/noto_sans_regular') / path.name
                if kind == 'sprites' and path.name in ('base.json', 'base.png', 'base@2x.json', 'base@2x.png'):
                    relative = Path('sprites') / path.name
                if re.fullmatch(r'(LICENSE|OFL|NOTICE|COPYRIGHT)(\.[A-Za-z]+)?', path.name, re.IGNORECASE):
                    relative = Path('licenses') / kind / Path(*path.parts)
                if relative is None:
                    continue
                source = regular_member(archive, member)
                total += source.size
                if total > 512 * 1024**2 or len(copied) >= 8192:
                    raise ValueError('Expanded assets exceed the bounded extraction budget')
                output = target / relative
                output.parent.mkdir(parents=True, exist_ok=True)
                with archive.extractfile(source) as incoming, output.open('xb') as outgoing:
                    shutil.copyfileobj(incoming, outgoing, 1024 * 1024)
                copied.append({'path': str(relative), 'bytes': output.stat().st_size, 'sha256': digest(output)})
    required = ['fonts/noto_sans_regular/0-255.pbf', 'fonts/noto_sans_regular/1024-1279.pbf',
                'sprites/base.json', 'sprites/base.png', 'sprites/base@2x.json', 'sprites/base@2x.png']
    if any(not (target / name).is_file() or not (target / name).stat().st_size for name in required):
        raise ValueError('Latin/Cyrillic glyph blocks or matching sprite files are missing')
    for name in ('base.json', 'base@2x.json'):
        if not isinstance(json.loads((target / 'sprites' / name).read_text()), dict):
            raise ValueError('Invalid sprite index')
    for name in ('base.png', 'base@2x.png'):
        with (target / 'sprites' / name).open('rb') as source:
            if source.read(8) != b'\x89PNG\r\n\x1a\n':
                raise ValueError('Invalid sprite PNG')
    return copied


def seed_copy(source, output, inputs):
    output.mkdir()
    for original, name in [('data/valhalla3', 'valhalla'), ('data/photon-stream4', 'photon'), ('tools/jre', 'runtime/jre')]:
        shutil.copytree(source / original, output / name, symlinks=True)
    shutil.copyfile(source / 'inputs/photon.jar', output / 'runtime/photon.jar')
    shutil.copyfile(source / 'inputs/pmtiles', output / 'runtime/pmtiles')
    (output / 'runtime/pmtiles').chmod(0o755)
    if digest(output / 'runtime/photon.jar') != photon.JAR_SHA256:
        raise ValueError('Copied Photon JAR checksum mismatch')
    maps = output / 'public/map-data'
    archive = maps / MAP_SHA256[:12] / 'kazakhstan.pmtiles'
    archive.parent.mkdir(parents=True)
    shutil.copyfile(source / 'data/maps/kazakhstan.pmtiles', archive)
    if digest(archive) != MAP_SHA256:
        raise ValueError('Copied map archive checksum mismatch')
    assets = extract_assets(inputs / 'fonts.tar.gz', inputs / 'sprites.tar.gz', maps)
    (maps / 'licenses').mkdir(exist_ok=True)
    for name in ASSETS:
        if name.endswith('.txt'):
            shutil.copyfile(inputs / name, maps / 'licenses' / name)
    guard.save(output / 'assets.json', {'archives': ASSETS, 'files': assets})


def seed(runtime, benchmark, dataset_id):
    release = runtime / 'releases' / validate_id(dataset_id)
    if release.exists() or (runtime / '.env').exists():
        raise RuntimeError('Seed requires a new release and an uninitialized GIS runtime')
    if not (benchmark / 'finished.json').is_file() or execute(
            ['docker', 'ps', '-q', '--filter', f'label={guard.LABEL}={benchmark.name}']):
        raise RuntimeError('Seed requires the completed, stopped source benchmark')
    with guarded_job(runtime) as job:
        for name, item in ASSETS.items():
            guard.fetch(job, argparse.Namespace(name=name, url=item['url'], max_bytes=item['bytes'],
                        sha256=item['sha256'], md5=None, timeout=600))
        stage(job, 'seed-copy', ['python3', '/tools/gis.py', '_seed-copy', '/source', '/job/data/release', '/job/inputs'],
              [(TOOLS, '/tools', True), (benchmark, '/source', True), (job, '/job', False)], memory='512m', seconds=3600)
        guard.ensure_watch(job)
    # Only metadata renames/writes follow the final successful host check.
    release.parent.mkdir(exist_ok=True)
    (job / 'data/release').rename(release)
    values = {'VALHALLA_DATA_DIR': str(release / 'valhalla'), 'PHOTON_DATA_DIR': str(release / 'photon'),
              'GIS_RUNTIME_DIR': str(release / 'runtime'), 'GIS_PUBLIC_DIR': str(release / 'public')}
    atomic_env(runtime / '.env', values)
    guard.save(runtime / 'seed.json', {'time': time.time(), 'release': str(release),
               'source_benchmark': str(benchmark), 'map_sha256': MAP_SHA256, 'job': str(job)})
    print(json.dumps({'seeded': str(release), 'archive_url': f'/map-data/{MAP_SHA256[:12]}/kazakhstan.pmtiles'}))


def activate(runtime):
    # This initial bring-up is deliberately separate from application activation.
    if execute(compose(runtime) + ['ps', '-q']):
        raise RuntimeError('Initial activate refuses existing running GIS services; use the documented component cutover')
    read_env(runtime / '.env')
    if execute(['ss', '-H', '-ltn', 'sport', '=', '3052']):
        raise RuntimeError('GIS port 3052 is already in use; do not replace the existing listener')
    with guarded_job(runtime, abort_services=('valhalla', 'photon', 'maps'), seconds=600) as job:
        guard.execute(job, 'activate', compose(runtime) + ['up', '-d', '--no-build', '--pull', 'never',
                      '--wait', '--wait-timeout', '180'], 240)
        probe = '''import urllib.request
q=urllib.request.Request('http://10.0.0.2:3052/map-data/2583113c45ca/kazakhstan.pmtiles',headers={'Range':'bytes=0-126'})
with urllib.request.urlopen(q,timeout=5) as r:
 assert r.status==206 and r.headers.get('Content-Range')=='bytes 0-126/1298561630'
 b=r.read(128)
 assert len(b)==127 and b[:8]==b'PMTiles\\x03'
print('Published range verified')'''
        guard.execute(job, 'map-range', [sys.executable, '-c', probe], 15)
        guard.save(runtime / 'activation.json', {'time': time.time(), 'job': str(job), 'limits': 'deploy/gis/compose.yml'})


def restart(runtime, component):
    """Retry the selected service only after fresh strict host preflight."""
    with guarded_job(runtime, abort_services=(component,), excluded=('bkgis-' + component + '-1',), seconds=600) as job:
        guard.execute(job, 'restart-' + component, compose(runtime) + ['up', '-d', '--no-deps',
                      '--no-build', '--pull', 'never', '--wait', '--wait-timeout', '180', component], 240)


def serving_snapshot():
    names = ['bkgis-' + service + '-1' for service in ('valhalla', 'photon', 'maps')]
    rows = json.loads(execute(['docker', 'inspect', *names], timeout=10))
    result = {}
    for row in rows:
        labels = row['Config'].get('Labels', {})
        service = labels.get('com.docker.compose.service')
        if labels.get('com.docker.compose.project') != 'bkgis' or service not in ('valhalla', 'photon', 'maps'):
            raise ValueError('Unexpected GIS container identity')
        state = row['State']
        if not state['Running'] or state.get('Health', {}).get('Status') != 'healthy':
            raise ValueError(f'{service} is not healthy and running')
        ip = None
        if service != 'maps':
            ip = row['NetworkSettings']['Networks']['bikerskz_gis']['IPAddress']
            address = ipaddress.ip_address(ip)
            if address.version != 4 or not address.is_private or address.is_loopback:
                raise ValueError('Expected a private Docker IPv4 address')
        relative = Path(f"/proc/{state['Pid']}/cgroup").read_text().strip().split('::', 1)[1]
        directory = Path('/sys/fs/cgroup') / relative.lstrip('/')
        metrics = {name: (directory / name).read_text().strip() for name in
                   ('memory.current', 'memory.peak', 'memory.stat', 'memory.events', 'cpu.stat', 'io.stat')}
        host = row['HostConfig']
        result[service] = {'id': row['Id'], 'ip': ip, 'image': row['Config']['Image'],
            'memory_limit': host['Memory'], 'memory_swap_limit': host['MemorySwap'],
            'nano_cpus': host['NanoCpus'], 'cgroups': metrics,
            'memory_kind': 'cgroup charged memory including cache; not process RSS; peak is since container creation'}
    if set(result) != {'valhalla', 'photon', 'maps'}:
        raise ValueError('Three distinct GIS services are required')
    return result


def verification_archive(runtime, values, archive_url=None):
    if archive_url is None:
        archive_url = values.get('MAPS_ARCHIVE_URL')
    if archive_url is None:
        sha = json.loads((runtime / 'seed.json').read_text())['map_sha256']
        if not re.fullmatch('[a-f0-9]{64}', sha):
            raise ValueError('Seed receipt lacks a map archive digest')
        archive_url = f'/map-data/{sha[:12]}/kazakhstan.pmtiles'
    if not re.fullmatch(r'/map-data/[a-f0-9]{12,64}/kazakhstan\.pmtiles', archive_url):
        raise ValueError('Verification requires an explicit versioned same-origin map archive')
    public = Path(values['GIS_PUBLIC_DIR']).resolve(strict=True)
    archive = (public / archive_url.lstrip('/')).resolve(strict=True)
    if not archive.is_relative_to(public) or not archive.is_file() or archive.stat().st_size < 65536:
        raise ValueError('Expected a local selected map archive of at least 64 KiB')
    with archive.open('rb') as source:
        if source.read(8) != b'PMTiles\x03':
            raise ValueError('Selected archive is not PMTiles v3')
    return archive, archive_url


def in_kazakhstan_box(coordinate):
    return (isinstance(coordinate, list) and len(coordinate) == 2 and
            all(isinstance(value, (int, float)) and not isinstance(value, bool) and math.isfinite(value) for value in coordinate)
            and 46 <= coordinate[0] <= 87.5 and 40.5 <= coordinate[1] <= 55.5)


def validate_search(body, city=None):
    if not isinstance(body, dict) or body.get('type') != 'FeatureCollection':
        raise ValueError('Photon must return a GeoJSON FeatureCollection')
    features = body.get('features')
    if not isinstance(features, list) or not 1 <= len(features) <= 8:
        raise ValueError('Photon must return 1–8 fixture results')
    for feature in features:
        geometry = feature.get('geometry', {})
        properties = feature.get('properties', {})
        if (feature.get('type') != 'Feature' or geometry.get('type') != 'Point' or
                not in_kazakhstan_box(geometry.get('coordinates')) or
                str(properties.get('countrycode', '')).upper() != 'KZ' or
                not isinstance(properties.get('name'), str) or not properties['name'].strip()):
            raise ValueError('Photon returned invalid or non-Kazakhstan fixture data')
    if city:
        lon, lat = {'almaty': (76.95, 43.24), 'astana': (71.43, 51.13)}[city]
        if not any(abs(feature['geometry']['coordinates'][0] - lon) < 1 and
                   abs(feature['geometry']['coordinates'][1] - lat) < 1 for feature in features):
            raise ValueError('Photon fixture has no result near its expected city')
    return {'features': len(features), 'first_name': features[0]['properties']['name']}


def road_request(points):
    return {'locations': [{'lat': lat, 'lon': lon, 'type': 'break', 'search_cutoff': 1000} for lat, lon in points],
            'costing': 'motorcycle', 'costing_options': {'motorcycle': {'use_tracks': 0, 'use_trails': 0}},
            'format': 'osrm', 'shape_format': 'geojson', 'directions_type': 'none', 'alternates': 0, 'units': 'kilometers'}


def validate_route(body, points, distance_bounds):
    if not isinstance(body, dict) or body.get('code') != 'Ok' or len(body.get('routes', [])) != 1:
        raise ValueError('Valhalla must return one successful OSRM route')
    route = body['routes'][0]
    geometry = route.get('geometry', {})
    coordinates = geometry.get('coordinates')
    if (geometry.get('type') != 'LineString' or not isinstance(coordinates, list) or
            not 2 <= len(coordinates) <= 50000 or not all(in_kazakhstan_box(point) for point in coordinates)):
        raise ValueError('Valhalla must return bounded Kazakhstan GeoJSON coordinates')
    distance, duration = route.get('distance'), route.get('duration')
    if (not isinstance(distance, (int, float)) or not math.isfinite(distance) or
            not distance_bounds[0] <= distance <= distance_bounds[1] or
            not isinstance(duration, (int, float)) or isinstance(duration, bool) or not math.isfinite(duration) or not 0 < duration < 604800):
        raise ValueError('Valhalla fixture distance or duration is invalid')
    for coordinate, (lat, lon) in zip((coordinates[0], coordinates[-1]), points):
        if abs(coordinate[0] - lon) > .03 or abs(coordinate[1] - lat) > .03:
            raise ValueError('Valhalla geometry endpoints do not match the fixture')
    return {'distance_meters': distance, 'duration_seconds': duration, 'coordinates': len(coordinates)}


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, _request, _fp, _code, _message, _headers, _url):
        return None


def verify_requests(plan):
    """A fixed 96-request mixed pilot, never a configurable stress generator."""
    archive = Path(plan['archive'])
    size = archive.stat().st_size
    deadline = time.monotonic() + 85
    stopped = threading.Event()
    limits = {'route': threading.Semaphore(1), 'search': threading.Semaphore(2), 'map': threading.Semaphore(2)}
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    cities = [('almaty', ['Almaty', 'Алматы', 'Алматы']), ('astana', ['Astana', 'Астана', 'Астана'])]
    searches = [(city + '_' + lang, {'q': query, 'lang': lang, 'limit': 5,
                 'countrycode': 'KZ', 'bbox': '46,40.5,87.5,55.5'})
                for city, names in cities for lang, query in zip(('en', 'ru', 'kk'), names)]
    routes = [(name, points, bounds) for (name, points), bounds in zip(valhalla.FIXTURES,
              ((100, 50000), (1000, 100000), (500000, 2000000)))]
    generator = random.Random(20261105)
    tasks = []
    for index in range(32):
        tasks.extend([('route', *routes[index % len(routes)]), ('search', *searches[index % len(searches)]),
                      ('map', 'header' if index == 0 else 'range-' + str(index),
                       0 if index == 0 else generator.randrange(127, size - 65536 + 1), 127 if index == 0 else 65536)])

    def request(task):
        kind, name, *parameters = task
        began = time.monotonic()
        acquired = False
        row = {'provider': kind, 'fixture': name, 'ok': False}
        try:
            if stopped.is_set():
                raise RuntimeError('Skipped after an earlier verification failure')
            remaining = deadline - time.monotonic()
            if remaining <= 0 or not limits[kind].acquire(timeout=remaining):
                raise TimeoutError('Mixed verification timebox exceeded')
            acquired = True
            if stopped.is_set():
                raise RuntimeError('Skipped after an earlier verification failure')
            headers, payload = {'Accept-Encoding': 'identity'}, None
            if kind == 'search':
                url = plan['photon'] + '/api/?' + urllib.parse.urlencode(parameters[0])
                maximum = 256 * 1024
            elif kind == 'route':
                url = plan['valhalla'] + '/route'
                payload = json.dumps(road_request(parameters[0])).encode()
                headers['Content-Type'] = 'application/json'
                maximum = 8 * 1024**2
            else:
                start, length = parameters
                url = plan['maps']
                headers['Range'] = f'bytes={start}-{start + length - 1}'
                maximum = length
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise TimeoutError('Mixed verification timebox exceeded')
            row['attempted'] = True
            with opener.open(urllib.request.Request(url, data=payload, headers=headers), timeout=min(8, remaining)) as response:
                body = response.read(maximum + 1)
                if len(body) > maximum:
                    raise ValueError('Provider response exceeds the bounded size')
                if kind == 'map':
                    if (response.status != 206 or response.headers.get('Content-Range') != f'bytes {start}-{start + length - 1}/{size}' or
                            response.headers.get('Content-Length') != str(length) or
                            response.headers.get('Content-Encoding', 'identity') != 'identity' or len(body) != length):
                        raise ValueError('Incorrect HTTP range status, headers or length')
                    with archive.open('rb') as source:
                        source.seek(start)
                        if body != source.read(length):
                            raise ValueError('HTTP range bytes differ from the selected local archive')
                    row.update(start=start, bytes=length)
                else:
                    if response.status != 200:
                        raise ValueError('Provider did not return HTTP 200')
                    data = json.loads(body)
                    row.update(validate_search(data, name.split('_')[0]) if kind == 'search' else validate_route(data, *parameters))
            row['ok'] = True
        except Exception as error:
            row['error'] = type(error).__name__ + ': ' + str(error)
            stopped.set()
        finally:
            if acquired:
                limits[kind].release()
            row['elapsed_ms'] = (time.monotonic() - began) * 1000
        return row

    began = time.monotonic()
    with ThreadPoolExecutor(max_workers=5) as pool:
        rows = list(pool.map(request, tasks))
    summaries = {}
    for kind in limits:
        items = [row for row in rows if row['provider'] == kind]
        values = sorted(row['elapsed_ms'] for row in items if row.get('attempted'))
        summaries[kind] = {'planned': len(items), 'attempted': sum(bool(row.get('attempted')) for row in items),
            'successful': sum(row['ok'] for row in items), 'errors': sum(not row['ok'] for row in items),
            'p50_ms': values[math.ceil(len(values) * .5) - 1] if values else None,
            'p95_ms': values[math.ceil(len(values) * .95) - 1] if values else None,
            'max_ms': max(values) if values else None}
    return {'kind': 'guarded-combined-gis-pilot', 'planned': len(tasks), 'error_count': sum(not row['ok'] for row in rows),
        'elapsed_seconds': time.monotonic() - began, 'limits': {'workers': 5, 'route': 1, 'search': 2, 'map': 2,
            'request_timeout_seconds': 8, 'query_window_seconds': 85}, 'summaries': summaries, 'requests': rows,
        'limitations': ['No cache eviction; first requests are not cold-cache measurements',
            'Small repeated fixtures and local-network file transport; not route safety or browser acceptance',
            'Client latency includes semaphore wait and validation; not provider-only latency']}


def verify(runtime, archive_url=None):
    report = {'ok': False, 'started': time.time()}
    job = None
    try:
        values = read_env(runtime / '.env')
        archive, archive_url = verification_archive(runtime, values, archive_url)
        with guarded_job(runtime, abort_services=('valhalla', 'photon', 'maps'), seconds=120) as job:
            before = serving_snapshot()
            if before['valhalla']['memory_limit'] != guard.GIB or before['valhalla']['memory_swap_limit'] != guard.GIB:
                raise ValueError('Combined acceptance requires the selected 1 GiB Valhalla cap')
            plan = {'archive': str(archive), 'archive_url': archive_url,
                    'photon': 'http://' + before['photon']['ip'] + ':2322',
                    'valhalla': 'http://' + before['valhalla']['ip'] + ':8002',
                    'maps': 'http://10.0.0.2:3052' + archive_url}
            guard.save(job / 'specs/verification.json', plan)
            report.update(archive_url=archive_url, archive_bytes=archive.stat().st_size, before=before, job=str(job))
            try:
                guard.execute(job, 'verification', [sys.executable, str(Path(__file__).resolve()),
                              '_verify', str(job / 'specs/verification.json')], 90)
            finally:
                log = job / 'logs/verification.log'
                if log.exists():
                    try:
                        report['queries'] = json.loads(log.read_text())
                    except ValueError:
                        report['query_log_error'] = 'No complete JSON result; inspect the private job log'
            if report.get('queries', {}).get('error_count') != 0:
                raise ValueError('Combined verification has provider or range failures')
            report['after'] = serving_snapshot()
            for service, original in before.items():
                current = report['after'][service]
                if current['id'] != original['id']:
                    raise ValueError('A GIS container changed during combined acceptance')
                def events(value):
                    return {key: int(count) for key, count in (line.split() for line in value['cgroups']['memory.events'].splitlines())}
                previous_events, current_events = events(original), events(current)
                if any(current_events.get(key, 0) > previous_events.get(key, 0) for key in ('oom', 'oom_kill')):
                    raise ValueError('A GIS memory cgroup recorded OOM during combined acceptance')
        report['ok'] = True
    except BaseException as error:
        report['error'] = type(error).__name__ + ': ' + str(error)
        raise
    finally:
        report['ended'] = time.time()
        if job is not None:
            report['core_app_watch_receipts'] = {name: str(job / name) for name in
                ('baseline.json', 'samples.jsonl', 'finished.json', 'abort.json') if (job / name).exists()}
            if (job / 'samples.jsonl').exists():
                samples = [json.loads(line) for line in (job / 'samples.jsonl').read_text().splitlines() if line]
                readings = sorted(row['readiness']['seconds'] for row in samples)
                report['core_app_watch_summary'] = {'samples': len(samples),
                    'failed_probes': sum(not row['readiness']['okay'] for row in samples),
                    'guard_flags': sum(bool(row['reasons']) for row in samples),
                    'readiness_p95_ms': readings[math.ceil(len(readings) * .95) - 1] * 1000 if readings else None,
                    'readiness_max_ms': max(readings) * 1000 if readings else None}
        guard.save(runtime / 'verification.json', report)
    print(json.dumps({'verified': True, 'receipt': str(runtime / 'verification.json'), 'requests': 96}))


def switch(runtime, component, candidate):
    """Select a verified candidate explicitly; keep previous data for rollback."""
    candidate = candidate.resolve(strict=True)
    if candidate.parent != (runtime / 'releases').resolve() or candidate.is_symlink():
        raise ValueError('Candidate must be an immediate release under this GIS runtime')
    receipt = json.loads((candidate / 'candidate.json').read_text())
    if receipt['component'] != component:
        raise ValueError('Candidate component does not match the requested switch')
    values = read_env(runtime / '.env')
    if component == 'maps':
        # Publish immutable bytes first. The application archive URL changes only
        # in a separately reviewed configuration rollout, so clients cannot mix versions.
        archive = candidate / 'kazakhstan.pmtiles'
        sha = receipt['archive_sha256']
        if not re.fullmatch('[a-f0-9]{64}', sha):
            raise ValueError('Candidate is missing its verified archive digest')
        destination = Path(values['GIS_PUBLIC_DIR']) / 'map-data' / sha[:12]
        if destination.exists():
            raise RuntimeError('Refusing to replace an already published map version')
        with guarded_job(runtime) as job:
            stage(job, 'check-publish-map', ['python3', '-c',
                  'import hashlib,sys; h=hashlib.sha256(); f=open("/input/archive","rb"); '
                  '[h.update(b) for b in iter(lambda:f.read(1048576),b"")]; assert h.hexdigest()==sys.argv[1]', sha],
                  [(archive, '/input/archive', True)], memory='256m')
        staging = destination.with_name('.' + sha[:12] + '-' + uuid.uuid4().hex)
        staging.mkdir()
        archive.rename(staging / 'kazakhstan.pmtiles')
        staging.rename(destination)
        receipt.update(published=True, archive_url=f'/map-data/{sha[:12]}/kazakhstan.pmtiles')
        guard.save(candidate / 'candidate.json', receipt)
        print(json.dumps({'published': receipt['archive_url'], 'application_url_changed': False}))
        return
    key = {'valhalla': 'VALHALLA_DATA_DIR', 'photon': 'PHOTON_DATA_DIR'}[component]
    old_values = dict(values)
    values[key] = str(candidate)
    target = 'bkgis-' + component + '-1'
    try:
        with guarded_job(runtime, abort_services=(component,), excluded=(target,), seconds=600) as job:
            atomic_env(runtime / '.env', values)
            guard.execute(job, 'switch-' + component, compose(runtime) + ['up', '-d', '--no-deps',
                          '--no-build', '--pull', 'never', '--wait', '--wait-timeout', '180', component], 240)
            guard.save(job / 'results/selection.json', {'previous': old_values[key], 'selected': str(candidate)})
        receipt.update(activated=True, previous=old_values[key], time=time.time())
        guard.save(candidate / 'candidate.json', receipt)
    except BaseException:
        # A failed guard must not immediately restart an expensive service while
        # the original app is regressing. Restore selection, keep it stopped,
        # then require a healthy new guarded job before retrying.
        atomic_env(runtime / '.env', old_values)
        raise


def refresh(runtime, component, source, expected_sha256, dataset_id):
    """Build a new candidate offline; never overwrite or switch a serving dataset."""
    release = runtime / 'releases' / validate_id(dataset_id)
    if release.exists() or not re.fullmatch('[a-f0-9]{64}', expected_sha256):
        raise ValueError('A new release ID and exact input SHA256 are required')
    values = read_env(runtime / '.env')
    with guarded_job(runtime) as job:
        if source.stat().st_size > 3 * guard.GIB:
            raise ValueError('Compressed input exceeds the 3 GiB refresh budget')
        stage(job, 'input', ['python3', '/tools/gis.py', '_copy-input', '/input/source', '/job/inputs/source', expected_sha256],
              [(TOOLS, '/tools', True), (source, '/input/source', True), (job, '/job', False)], memory='256m')
        output = job / 'data/release'; output.mkdir()
        if component == 'valhalla':
            (job / 'inputs/source').rename(output / 'kazakhstan.osm.pbf')
            for kind in ('config', 'import'):
                spec = valhalla.spec(kind, output, 1, 33812)
                image_index = spec['docker_args'].index(spec['image'])
                path = job / 'specs' / ('refresh-valhalla-' + kind + '.json')
                guard.save(path, {'name': 'refresh-valhalla-' + kind, 'image': spec['image'],
                           'memory': '5g', 'cpus': '1.5', 'read_bps': '5mb', 'write_bps': '5mb',
                           'network': 'none', 'timeout': 3600,
                           'args': ['--pull=never', '--read-only', *spec['docker_args'][:image_index]], 'command': spec['command']})
                guard.stage(job, path)
        elif component == 'photon':
            command = ['bash', '-o', 'pipefail', '-c',
                'python3 /tools/osm-benchmark/photon.py stream --source /job/inputs/source --max-output-gib 16 | '
                '/opt/java/openjdk/bin/java -XX:ActiveProcessorCount=2 -Duser.home=/tmp -Xms512m -Xmx2g '
                '-jar /opt/photon.jar import -data-dir /data -import-file - -country-codes kz -languages kk,ru,en']
            stage(job, 'refresh-photon', command, [(TOOLS, '/tools', True), (job, '/job', False),
                  (output, '/data', False), (Path(values['GIS_RUNTIME_DIR']) / 'jre', '/opt/java/openjdk', True),
                  (Path(values['GIS_RUNTIME_DIR']) / 'photon.jar', '/opt/photon.jar', True)], memory='5g', cpus='2.0', seconds=3600)
            log = (job / 'logs/refresh-photon.log').read_text()
            if 'Database has been successfully set up' not in log or '"stage": "photon-stream", "ok": true' not in log:
                raise RuntimeError('Photon candidate lacks successful import and verified stream EOF')
        else:
            (output / 'tmp').mkdir()
            stage(job, 'refresh-maps', ['/opt/pmtiles', 'convert', '/job/inputs/source', '/data/kazakhstan.pmtiles', '--tmpdir', '/data/tmp'],
                  [(Path(values['GIS_RUNTIME_DIR']) / 'pmtiles', '/opt/pmtiles', True), (job, '/job', False),
                   (output, '/data', False)], memory='2g', seconds=3600,
                   environment={'GOMAXPROCS': '1', 'GOMEMLIMIT': '1GiB'})
            stage(job, 'verify-maps', ['/opt/pmtiles', 'verify', '/data/kazakhstan.pmtiles'],
                  [(Path(values['GIS_RUNTIME_DIR']) / 'pmtiles', '/opt/pmtiles', True), (output, '/data', True)])
            stage(job, 'clean-map-temp', ['python3', '-c', 'import shutil; shutil.rmtree("/data/tmp")'], [(output, '/data', False)], memory='128m')
            stage(job, 'map-digest', ['python3', '-c',
                  'import hashlib,json; h=hashlib.sha256(); f=open("/data/kazakhstan.pmtiles","rb"); '
                  '[h.update(b) for b in iter(lambda:f.read(1048576),b"")]; print(json.dumps({"sha256":h.hexdigest()}))'],
                  [(output, '/data', True)], memory='256m')
        guard.ensure_watch(job)
        receipt = {'component': component, 'input_sha256': expected_sha256,
                   'source': str(source), 'job': str(job), 'activated': False}
        if component == 'maps':
            receipt['archive_sha256'] = json.loads((job / 'logs/map-digest.log').read_text())['sha256']
    release.parent.mkdir(exist_ok=True)
    output.rename(release)
    guard.save(release / 'candidate.json', receipt)
    print(json.dumps({'candidate': str(release), 'activated': False}))


def main():
    if len(sys.argv) > 1 and sys.argv[1] in ('_watch', '_lease', '_seed-copy', '_copy-input', '_verify'):
        action, *args = sys.argv[1:]
        if action == '_seed-copy':
            seed_copy(*map(Path, args)); return
        if action == '_copy-input':
            copy_verified(Path(args[0]), Path(args[1]), args[2]); return
        if action == '_verify':
            report = verify_requests(json.loads(Path(args[0]).read_text()))
            print(json.dumps(report), flush=True)
            if report['error_count']:
                raise SystemExit(1)
            return
        for sig in (signal.SIGTERM, signal.SIGHUP):
            signal.signal(sig, interrupted)
        if action == '_watch': child_watch(Path(args[0]), int(args[1]))
        else: child_lease(Path(args[0]), int(args[1]), int(args[2]), args[3])
        return
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--runtime', type=Path, default=DEFAULT_RUNTIME)
    commands = parser.add_subparsers(dest='action', required=True)
    seed_parser = commands.add_parser('seed')
    seed_parser.add_argument('--benchmark', type=Path, required=True)
    seed_parser.add_argument('--id', required=True)
    commands.add_parser('activate')
    commands.add_parser('status')
    check = commands.add_parser('verify')
    check.add_argument('--archive-url', help='Versioned map-data URL; defaults to GIS selection or seed receipt')
    retry = commands.add_parser('restart')
    retry.add_argument('--component', choices=('valhalla', 'photon', 'maps'), required=True)
    build = commands.add_parser('refresh')
    build.add_argument('--component', choices=('valhalla', 'photon', 'maps'), required=True)
    build.add_argument('--source', type=Path, required=True)
    build.add_argument('--sha256', required=True)
    build.add_argument('--id', required=True)
    select = commands.add_parser('switch')
    select.add_argument('--component', choices=('valhalla', 'photon', 'maps'), required=True)
    select.add_argument('--candidate', type=Path, required=True)
    args = parser.parse_args()
    for sig in (signal.SIGTERM, signal.SIGHUP):
        signal.signal(sig, interrupted)
    runtime = args.runtime.resolve()
    if args.action != 'status':
        require_clean_source()
    with serial(runtime):
        if args.action == 'seed': seed(runtime, args.benchmark.resolve(strict=True), args.id)
        elif args.action == 'activate': activate(runtime)
        elif args.action == 'refresh': refresh(runtime, args.component, args.source.resolve(strict=True), args.sha256, args.id)
        elif args.action == 'switch': switch(runtime, args.component, args.candidate)
        elif args.action == 'restart': restart(runtime, args.component)
        elif args.action == 'verify': verify(runtime, args.archive_url)
        else: print(execute(compose(runtime) + ['ps', '--format', 'json']))


if __name__ == '__main__':
    try:
        main()
    except (OSError, RuntimeError, ValueError, KeyError) as error:
        print(f'GIS operation stopped: {error}', file=sys.stderr)
        sys.exit(1)

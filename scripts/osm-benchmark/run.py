#!/usr/bin/env python3
"""Bounded, isolated OSM benchmark supervisor. Run only on the benchmark host."""
import argparse
from concurrent.futures import ThreadPoolExecutor
import hashlib
import json
import os
from pathlib import Path
import signal
import statistics
import subprocess
import sys
import time
import urllib.request

GIB = 1024 ** 3
LABEL = 'bkrs.osm-benchmark'


def command(args, **kwargs):
    return subprocess.run(args, text=True, capture_output=True, timeout=kwargs.pop('timeout', 30), **kwargs)


def save(path, value):
    # Follow benchmark control symlinks into tmpfs: disk writeback must not stall
    # the safety heartbeat. Ordinary result paths remain on persistent storage.
    path = path.resolve()
    temp = path.with_suffix('.new')
    temp.write_text(json.dumps(value, indent=2) + '\n')
    temp.replace(path)


def containers():
    listing = command(['docker', 'ps', '-q'])
    listing.check_returncode()
    ids = listing.stdout.split()
    if not ids:
        return {}
    result = command(['docker', 'inspect', *ids])
    result.check_returncode()
    return {x['Name'].lstrip('/'): {'id': x['Id'], 'started': x['State']['StartedAt'],
            'restarts': x['RestartCount'], 'running': x['State']['Running'],
            'health': x['State'].get('Health', {}).get('Status'),
            'benchmark': x['Config'].get('Labels', {}).get(LABEL)}
            for x in json.loads(result.stdout)}


def probe():
    started = time.monotonic()
    try:
        with urllib.request.urlopen('http://127.0.0.1:3050/health/ready', timeout=2) as r:
            okay = r.status == 200 and json.loads(r.read()).get('status') == 'ready'
        return {'okay': okay, 'seconds': time.monotonic() - started}
    except Exception as exc:
        return {'okay': False, 'seconds': time.monotonic() - started, 'error': str(exc)}


def host(root):
    memory = {k: int(v.split()[0]) * 1024 for k, v in
              (line.split(':', 1) for line in Path('/proc/meminfo').read_text().splitlines())}
    fs = os.statvfs(root)
    pressure = {}
    for resource in ('memory', 'io', 'cpu'):
        path = Path('/proc/pressure') / resource
        if path.exists():
            pressure[resource] = {line.split()[0]: dict(p.split('=') for p in line.split()[1:])
                                  for line in path.read_text().splitlines()}
    return {'time': time.time(), 'available': memory['MemAvailable'],
            'swap_used': memory['SwapTotal'] - memory['SwapFree'],
            'disk_free': fs.f_bavail * fs.f_frsize, 'load': os.getloadavg(), 'pressure': pressure}


def owned(root):
    result = command(['docker', 'ps', '-aq', '--filter', f'label={LABEL}={root.name}'])
    result.check_returncode()
    return result.stdout.split()


def stop_owned(root):
    ids = owned(root)
    if ids:
        result = command(['docker', 'stop', '-t', '8', *ids], timeout=60)
        result.check_returncode()
    running = command(['docker', 'ps', '-q', '--filter', f'label={LABEL}={root.name}'])
    running.check_returncode()
    if running.stdout.strip():
        raise RuntimeError('Some benchmark containers did not stop')


def lease(root, duration):
    """Independent heartbeat lease; systemd ExecStopPost also invokes stop."""
    deadline = time.monotonic() + duration
    try:
        while not (root / 'finished.json').exists():
            ensure_watch(root)
            if time.monotonic() > deadline:
                raise RuntimeError('Independent benchmark lease expired')
            time.sleep(2)
    except BaseException as exc:
        if not (root / 'abort.json').exists():
            save(root / 'abort.json', {'time': time.time(), 'reason': str(exc)})
        raise
    finally:
        stop_owned(root)


def cgroups(root):
    samples = {}
    for ident in owned(root):
        result = command(['docker', 'inspect', '--format', '{{.State.Pid}} {{.Name}}', ident])
        fields = result.stdout.split()
        if len(fields) != 2 or fields[0] == '0':
            continue
        try:
            pid = fields[0]
            relative = Path(f'/proc/{pid}/cgroup').read_text().strip().split('::', 1)[1]
            directory = Path('/sys/fs/cgroup') / relative.lstrip('/')
            files = {}
            for name in ('memory.current', 'memory.peak', 'memory.events', 'memory.stat',
                         'cpu.stat', 'io.stat', 'io.max', 'cpu.max'):
                path = directory / name
                if path.exists():
                    files[name] = path.read_text().strip()
            samples[fields[1].lstrip('/')] = files
        except (OSError, IndexError):
            pass
    return samples


def guard_reasons(sample, readiness, baseline, current=None):
    reasons = []
    if sample['available'] < 3 * GIB:
        reasons.append('available RAM below 3 GiB')
    if sample['disk_free'] < 20 * GIB:
        reasons.append('root filesystem free space below 20 GiB')
    if sample['swap_used'] > baseline['host']['swap_used'] + 256 * 1024 ** 2:
        reasons.append('host swap use increased by over 256 MiB')
    if not readiness['okay'] or readiness['seconds'] > baseline.get('readiness_limit', max(1.0, baseline['readiness_p95'] * 5)):
        reasons.append('application readiness failed or exceeded latency guard')
    if current is not None:
        for name, original in baseline['containers'].items():
            now = current.get(name)
            if not now or any(now[k] != original[k] for k in ('id', 'started', 'restarts')):
                reasons.append(f'existing container identity/restart changed: {name}')
            elif original['health'] == 'healthy' and now['health'] != 'healthy':
                reasons.append(f'existing container health regressed: {name}')
    return reasons


def ensure_watch(root):
    if (root / 'abort.json').exists():
        raise RuntimeError('Benchmark aborted; inspect abort.json before any continuation')
    status = json.loads((root / 'watch.json').read_text())
    if time.time() - status['time'] > 15:
        raise RuntimeError('Benchmark watchdog heartbeat is stale')


def prepare(root):
    root.mkdir(mode=0o700, parents=True, exist_ok=True)
    if (root / 'baseline.json').exists():
        raise RuntimeError('Refusing to overwrite an existing benchmark baseline')
    for folder in ('inputs', 'data', 'logs', 'results', 'specs'):
        (root / folder).mkdir(mode=0o700)
    readings = []
    for _ in range(12):
        reading = probe()
        if not reading['okay']:
            raise RuntimeError('Existing application is not ready at baseline')
        readings.append(reading['seconds'])
        time.sleep(1)
    sample = host(root)
    if sample['available'] < 8 * GIB or sample['disk_free'] < 30 * GIB:
        raise RuntimeError('Insufficient initial headroom')
    baseline = {'time': time.time(), 'host': sample, 'containers': containers(),
                'readiness_samples': readings, 'readiness_p95': sorted(readings)[-1]}
    save(root / 'baseline.json', baseline)
    print(json.dumps({'prepared': str(root), 'host': sample, 'readiness_ms': statistics.median(readings)*1000}))


def watch(root, duration):
    baseline = json.loads((root / 'baseline.json').read_text())
    deadline = time.monotonic() + duration
    strikes = 0
    count = 0
    pool = ThreadPoolExecutor(max_workers=1)
    pending = None
    deep = {'time': time.time(), 'containers': baseline['containers'], 'cgroups': {}, 'bytes': 0}

    def telemetry():
        observed = containers()
        charged = cgroups(root)
        size_result = command(['du', '-sb', str(root)])
        size_result.check_returncode()
        size = size_result.stdout.split()
        return {'time': time.time(), 'containers': observed, 'cgroups': charged,
                'bytes': int(size[0]) if size else 0}

    try:
        with (root / 'samples.jsonl').open('a') as out:
            while not (root / 'finished.json').exists():
                if time.monotonic() > deadline:
                    raise RuntimeError('Benchmark timebox expired')
                sample = host(root)
                readiness = probe()
                if pending is not None and pending.done():
                    deep = pending.result()
                    pending = None
                if pending is None and count % 5 == 0:
                    pending = pool.submit(telemetry)
                reasons = guard_reasons(sample, readiness, baseline, deep['containers'])
                sample['benchmark_bytes'] = deep['bytes']
                if deep['bytes'] > 12 * GIB:
                    reasons.append('benchmark directory exceeded 12 GiB')
                if time.time() - deep['time'] > 60:
                    reasons.append('Docker telemetry unavailable for over 60 seconds')
                strikes = strikes + 1 if reasons else 0
                sample.update(readiness=readiness, reasons=reasons,
                              telemetry_time=deep['time'], cgroups=deep['cgroups'])
                save(root / 'watch.json', sample)
                out.write(json.dumps(sample) + '\n')
                out.flush()
                immediate = any(reason.startswith('existing container') or
                                reason.startswith('benchmark directory') for reason in reasons)
                if baseline.get('fail_fast_readiness') and any(reason.startswith('application readiness') for reason in reasons):
                    immediate = True
                if strikes >= 3 or immediate:
                    raise RuntimeError('; '.join(reasons))
                count += 1
                time.sleep(2)
    except BaseException as exc:
        if not (root / 'abort.json').exists():
            save(root / 'abort.json', {'time': time.time(), 'reason': str(exc)})
        stop_owned(root)
        raise


def execute(root, name, args, seconds):
    ensure_watch(root)
    begin = time.time()
    log_path = root / 'logs' / (name + '.log')
    with log_path.open('w') as log:
        process = subprocess.Popen(['nice', '-n', '15', 'ionice', '-c', '3', *args],
                                   stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
        try:
            while process.poll() is None:
                ensure_watch(root)
                if time.time() - begin > seconds:
                    raise RuntimeError(f'{name} exceeded timebox')
                time.sleep(2)
        except BaseException:
            try:
                stop_owned(root)
            finally:
                if process.poll() is None:
                    os.killpg(process.pid, signal.SIGTERM)
                    try:
                        process.wait(timeout=10)
                    except subprocess.TimeoutExpired:
                        os.killpg(process.pid, signal.SIGKILL)
            raise
    receipt = {'name': name, 'started': begin, 'ended': time.time(),
               'elapsed_seconds': time.time() - begin, 'exit_code': process.returncode}
    save(root / 'results' / (name + '-receipt.json'), receipt)
    print(json.dumps(receipt))
    if process.returncode:
        raise RuntimeError(f'{name} failed; inspect {log_path}')


def stage(root, spec_path):
    ensure_watch(root)
    spec = json.loads(spec_path.read_text())
    phase = spec['name']
    name = root.name + '-' + phase
    memory = spec.get('memory', '5g')
    args = ['docker', 'run', '--name', name, '--label', f'{LABEL}={root.name}',
            '--cpus', str(spec.get('cpus', '1.5')), '--cpu-shares', '128', '--memory', memory,
            '--memory-swap', memory, '--pids-limit', '256', '--cap-drop', 'ALL',
            '--security-opt', 'no-new-privileges', '--blkio-weight', '100',
            '--device-read-bps', '/dev/sda:' + spec.get('read_bps', '5mb'),
            '--device-write-bps', '/dev/sda:' + spec.get('write_bps', '2mb'),
            '--network', spec.get('network', 'none')]
    extra = spec.get('args', [])
    if not any('/tmp:' in value for value in extra):
        args += ['--tmpfs', '/tmp:rw,nosuid,nodev,size=512m']
    if spec.get('detach'):
        args.append('--detach')
    args += spec.get('args', []) + [spec['image']] + spec.get('command', [])
    save(root / 'results' / (phase + '-command.json'), {'command': args})
    execute(root, phase, args, spec.get('timeout', 3600))
    if not spec.get('detach'):
        result = command(['docker', 'inspect', name])
        result.check_returncode()
        info = json.loads(result.stdout)[0]
        save(root / 'results' / (phase + '-state.json'), {'state': info['State'],
               'resources': {k: info['HostConfig'][k] for k in ('Memory', 'MemorySwap', 'NanoCpus', 'PidsLimit')}})
        if info['State']['OOMKilled']:
            raise RuntimeError('Benchmark container reached its memory limit')


def fetch(root, args):
    target = root / 'inputs' / args.name
    if target.exists():
        raise RuntimeError('Refusing to overwrite input')
    execute(root, 'fetch-' + args.name, ['curl', '--fail', '--location', '--retry', '2',
            '--connect-timeout', '20', '--max-time', str(args.timeout), '--limit-rate', '3M',
            '--max-filesize', str(args.max_bytes), '--output', str(target), args.url], args.timeout + 30)
    if target.stat().st_size > args.max_bytes:
        raise RuntimeError('Input exceeded byte limit')
    hashes = {key: hashlib.new(key) for key in ('sha256', 'md5')}
    with target.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 ** 2), b''):
            for h in hashes.values():
                h.update(block)
    values = {k: v.hexdigest() for k, v in hashes.items()}
    for algorithm in ('sha256', 'md5'):
        expected = getattr(args, algorithm)
        if expected and values[algorithm] != expected:
            raise RuntimeError(f'{algorithm} checksum mismatch')
    save(root / 'results' / ('input-' + args.name + '.json'),
         {'url': args.url, 'bytes': target.stat().st_size, **values})


def finish(root):
    stop_owned(root)
    baseline = json.loads((root / 'baseline.json').read_text())
    sample, readiness, current = host(root), probe(), containers()
    reasons = guard_reasons(sample, readiness, baseline, current)
    running = command(['docker', 'ps', '-q', '--filter', f'label={LABEL}={root.name}'])
    running.check_returncode()
    save(root / 'finished.json', {'time': time.time(), 'host': sample, 'readiness': readiness,
                                'existing_container_changes': reasons, 'benchmark_running':
                                running.stdout.split()})
    print((root / 'finished.json').read_text())
    if reasons:
        raise RuntimeError('Post-benchmark verification requires review')


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--root', type=Path, required=True)
    sub = parser.add_subparsers(dest='action', required=True)
    sub.add_parser('prepare')
    w = sub.add_parser('watch'); w.add_argument('--duration', type=int, default=10800)
    e = sub.add_parser('exec'); e.add_argument('--name', required=True); e.add_argument('--timeout', type=int, default=1800); e.add_argument('command', nargs=argparse.REMAINDER)
    s = sub.add_parser('stage'); s.add_argument('spec', type=Path)
    f = sub.add_parser('fetch'); f.add_argument('--name', required=True); f.add_argument('--url', required=True); f.add_argument('--max-bytes', type=int, required=True); f.add_argument('--sha256'); f.add_argument('--md5'); f.add_argument('--timeout', type=int, default=1800)
    sub.add_parser('finish')
    sub.add_parser('stop')
    l = sub.add_parser('lease'); l.add_argument('--duration', type=int, default=10800)
    args = parser.parse_args()
    root = args.root.resolve()
    if not root.name.startswith('bikers-osm-kz-'):
        parser.error('Dedicated bikers-osm-kz-* root required')
    if args.action == 'prepare': prepare(root)
    elif args.action == 'watch': watch(root, args.duration)
    elif args.action == 'exec': execute(root, args.name, args.command[1:] if args.command[:1] == ['--'] else args.command, args.timeout)
    elif args.action == 'stage': stage(root, args.spec)
    elif args.action == 'fetch': fetch(root, args)
    elif args.action == 'lease': lease(root, args.duration)
    elif args.action == 'stop': stop_owned(root)
    else: finish(root)


if __name__ == '__main__':
    main()

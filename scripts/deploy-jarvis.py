#!/usr/bin/env python3
"""Build a clean Git checkout on Jarvis and replace only its app container."""

import argparse
from datetime import datetime, timezone
import fcntl
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile


def run(args, *, cwd, stdin=None, visible=False):
    result = subprocess.run(
        args, cwd=cwd, input=stdin, text=True,
        stdout=None if visible else subprocess.PIPE,
        stderr=None if visible else subprocess.PIPE,
    )
    if result.returncode:
        raise RuntimeError(f"{args[0]} {args[1]} failed (exit {result.returncode})")
    return result.stdout.strip() if result.stdout is not None else ''


def atomic_write(path, content):
    descriptor, temporary = tempfile.mkstemp(prefix=f'.{path.name}.', dir=path.parent)
    try:
        with os.fdopen(descriptor, 'wb') as handle:
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--runtime-dir', type=Path, default=Path('/home/user/apps/bikerskz'))
    args = parser.parse_args()
    checkout = Path(__file__).resolve().parent.parent
    runtime = args.runtime_dir.resolve()
    env_file = runtime / '.env'
    if not env_file.is_file() or env_file.is_symlink() or env_file.stat().st_mode & 0o077:
        raise RuntimeError('Runtime .env must be a private regular file (mode 0600)')
    if (checkout / '.env').exists():
        raise RuntimeError('Keep deployment credentials outside the Git checkout')
    if run(['git', 'status', '--porcelain', '--untracked-files=all'], cwd=checkout):
        raise RuntimeError('Checkout is dirty; commit or remove local changes before deploying')
    revision = run(['git', 'rev-parse', 'HEAD'], cwd=checkout)
    if revision != run(['git', 'rev-parse', 'origin/main'], cwd=checkout):
        raise RuntimeError('Checkout must match origin/main; run git pull --ff-only first')
    platform = run(['docker', 'info', '--format', '{{.OSType}}/{{.Architecture}}'], cwd=checkout)
    if platform not in ('linux/amd64', 'linux/x86_64'):
        raise RuntimeError('Run this deployment on the Linux/amd64 Jarvis Docker host')
    os.umask(0o077)
    with (runtime / '.deploy.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        compose = ['docker', 'compose', '--project-directory', str(checkout),
                   '--env-file', str(env_file), '-p', 'bikerskz',
                   '-f', str(checkout / 'docker-compose.yml')]

        def inspect(container):
            return json.loads(run(['docker', 'inspect', container], cwd=checkout))[0]

        app_id = run(compose + ['ps', '-q', 'app'], cwd=checkout)
        db_id = run(compose + ['ps', '-q', 'db'], cwd=checkout)
        if not app_id or not db_id:
            raise RuntimeError('This command upgrades the existing running app and database')
        previous = inspect(app_id)
        database = inspect(db_id)
        if previous['State'].get('Health', {}).get('Status') != 'healthy':
            raise RuntimeError('Existing app must be healthy before deployment')
        original_env = env_file.read_bytes()
        original_compose = json.loads(run(compose + ['config', '--format', 'json'], cwd=checkout))
        if original_compose['services']['app']['image'] != previous['Config']['Image']:
            raise RuntimeError('Runtime APP_IMAGE does not match the running app')
        old_environment = dict(value.split('=', 1) for value in previous['Config']['Env'])
        if any(old_environment.get(key) != str(value) for key, value in
               original_compose['services']['app']['environment'].items()):
            raise RuntimeError('Runtime environment changed; review configuration separately')
        image = f'bikerskz:git-{revision}'
        print(f'Building {image} on Jarvis', flush=True)
        run(['docker', 'build', '--build-arg', f'VCS_REF={revision}', '-t', image, str(checkout)],
            cwd=checkout, visible=True)
        built = inspect(image)
        if built['Config'].get('Labels', {}).get('org.opencontainers.image.revision') != revision:
            raise RuntimeError('Built image does not identify the expected Git revision')
        updated_env, replacements = re.subn(
            rb'(?m)^APP_IMAGE=[^\r\n]*', b'APP_IMAGE=' + image.encode(), original_env)
        if replacements != 1:
            raise RuntimeError('Runtime .env must contain exactly one APP_IMAGE assignment')
        if run(['git', 'status', '--porcelain', '--untracked-files=all'], cwd=checkout):
            raise RuntimeError('Checkout changed during build')
        if run(['git', 'rev-parse', 'HEAD'], cwd=checkout) != revision:
            raise RuntimeError('Git revision changed during build')

        atomic_write(env_file, updated_env)
        try:
            run(compose + ['config', '--quiet'], cwd=checkout)
            run(compose + ['up', '-d', '--no-deps', '--no-build', '--wait',
                           '--wait-timeout', '120', 'app'], cwd=checkout)
            current = inspect(run(compose + ['ps', '-q', 'app'], cwd=checkout))
            current_db = inspect(run(compose + ['ps', '-q', 'db'], cwd=checkout))
            unchanged = (
                sorted(previous['Config']['Env']) == sorted(current['Config']['Env'])
                and previous['Mounts'] == current['Mounts']
                and previous['HostConfig']['PortBindings'] == current['HostConfig']['PortBindings']
                and database['Id'] == current_db['Id']
                and database['State']['StartedAt'] == current_db['State']['StartedAt']
            )
            if not unchanged:
                raise RuntimeError('Runtime environment, ports, mounts or database changed')
            if current['Image'] != built['Id'] or current['State']['Health']['Status'] != 'healthy':
                raise RuntimeError('New app did not become healthy with the expected image')
            if current['Config']['User'] != '10001:10001' or not current['HostConfig']['ReadonlyRootfs']:
                raise RuntimeError('App must remain unprivileged with a read-only root filesystem')
        except Exception:
            atomic_write(env_file, original_env)
            print('Activation failed; restoring the previous app image.', file=sys.stderr, flush=True)
            # Resolved configuration stays in memory; it includes private values.
            rollback = compose[:-2] + ['-f', '-']
            run(rollback + ['up', '-d', '--no-deps', '--no-build', '--wait',
                            '--wait-timeout', '120', 'app'], cwd=checkout,
                stdin=json.dumps(original_compose))
            raise

        receipt = {
            'deployed_at': datetime.now(timezone.utc).isoformat(),
            'revision': revision, 'image': image, 'image_id': built['Id'],
            'previous_image': previous['Config']['Image'],
            'checkout': str(checkout), 'database_container_unchanged': True,
            'runtime_configuration_unchanged': True, 'healthy': True,
        }
        receipts = runtime / '.deploy'
        receipts.mkdir(mode=0o700, exist_ok=True)
        atomic_write(receipts / 'current-git-release.json',
                     (json.dumps(receipt, indent=2) + '\n').encode())
        print(f'Deployed {revision}; app healthy, database and runtime settings retained.')


if __name__ == '__main__':
    try:
        main()
    except (OSError, RuntimeError, ValueError, KeyError) as error:
        print(f'Deployment stopped: {error}', file=sys.stderr)
        sys.exit(1)

#!/usr/bin/env python3
"""Build a clean Git checkout on Jarvis and replace only its app container."""

import argparse
import copy
from contextlib import contextmanager
from datetime import datetime, timezone
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import uuid


MIGRATION_FILES = (
    'schema.sql', 'schema_002_garage_clubs.sql', 'schema_003_bike_photos.sql',
    'schema_004_club_images.sql', 'schema_005_avatars_ride_media.sql',
    'schema_006_trust.sql', 'schema_007_notifications_preferences.sql',
    'schema_008_club_notifications.sql',
    'schema_009_notification_preferences_reminders.sql',
    'schema_010_ride_maps.sql',
)

GIS_ENVIRONMENT = {'PHOTON_BASE_URL': 'http://photon:2322',
                   'VALHALLA_BASE_URL': 'http://valhalla:8002'}
GIS_NETWORK = 'bikerskz_gis'

SCHEMA_RECEIPTS_SQL = '''-- bikerskz schema receipts
BEGIN TRANSACTION ISOLATION LEVEL READ COMMITTED READ ONLY;
SET LOCAL lock_timeout = '65s';
SELECT pg_advisory_xact_lock(775021613);
SELECT COALESCE(json_agg(json_build_object('version',version,'checksum',checksum)
 ORDER BY version),'[]'::json)::text FROM schema_migrations;
COMMIT;
'''

RETAINED_STATE_SQL = r"""-- bikerskz retained state
BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL timezone = 'UTC';
SET LOCAL bytea_output = 'hex';
SELECT format(
 'SELECT json_build_object(''table'',%L,''rows'',count(*),''sha256'',encode(sha256(convert_to(COALESCE(string_agg(row_hash, '''' ORDER BY row_hash COLLATE "C"), ''''),''UTF8'')),''hex''))::text FROM (SELECT encode(sha256(convert_to(%s,''UTF8'')),''hex'') AS row_hash FROM %I.%I t%s) hashed;',
 tablename,
 CASE WHEN tablename='rides' THEN '(to_jsonb(t)-''map_plan'')::text'
 ELSE 'to_jsonb(t)::text' END,
 schemaname,tablename,
 CASE WHEN tablename='schema_migrations' THEN ' WHERE version<=9' ELSE '' END)
FROM pg_tables WHERE schemaname='public'
 ORDER BY tablename
\gexec
SELECT json_build_object('sequences',COALESCE(json_agg(to_jsonb(s) ORDER BY sequencename),'[]'::json))::text
FROM (SELECT sequencename,last_value,start_value,increment_by,min_value,max_value,cycle,cache_size
 FROM pg_sequences WHERE schemaname='public') s;
COMMIT;
"""


MIGRATION_ADDITIONS_SQL = '''-- bikerskz schema-10 additions
BEGIN TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY;
SELECT json_build_object(
 'rides_with_map_plan',(SELECT count(*) FROM rides WHERE map_plan IS NOT NULL))::text;
COMMIT;
'''


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


def migration_checksums(checkout):
    directory = checkout / 'apps/backend/internal/database/postgres'
    source = (directory / 'migrations.go').read_text()
    # This opt-in is deliberately limited to the reviewed 9 -> 10 transition.
    # A future registered migration must not be applied by this mode silently.
    registered = [int(value) for value in re.findall(r'\{(\d+),\s*\w+\}', source)]
    if registered != list(range(1, 11)):
        raise RuntimeError('Migration-10 mode requires exactly source migrations 1 through 10')
    return {version: hashlib.sha256((directory / name).read_bytes()).hexdigest()
            for version, name in enumerate(MIGRATION_FILES, 1)}


def query_database(checkout, database_id, sql):
    return run(['docker', 'exec', '-i', database_id, 'psql', '-X', '-qAt',
                '-v', 'ON_ERROR_STOP=1', '-U', 'bikerskz', '-d', 'bikerskz'],
               cwd=checkout, stdin=sql)


def schema_receipts(checkout, database_id):
    rows = json.loads(query_database(checkout, database_id, SCHEMA_RECEIPTS_SQL))
    receipts = {row['version']: row['checksum'] for row in rows}
    if len(receipts) != len(rows):
        raise RuntimeError('Duplicate schema migration receipts')
    return receipts


def retained_state(checkout, database_id):
    rows = [json.loads(line) for line in
            query_database(checkout, database_id, RETAINED_STATE_SQL).splitlines() if line]
    return {'tables': [row for row in rows if 'table' in row],
            'sequences': next(row['sequences'] for row in rows if 'sequences' in row)}


def migration_additions(checkout, database_id):
    state = json.loads(query_database(checkout, database_id, MIGRATION_ADDITIONS_SQL))
    if state != {'rides_with_map_plan': 0}:
        raise RuntimeError('Migration-10 additions must leave all existing ride map plans NULL')
    return state


def configuration_plan(previous, resolved, configure_gis=False):
    """Allow only the reviewed GIS environment/network addition; retain everything else."""
    old = dict(value.split('=', 1) for value in previous['Config']['Env'])
    desired = {key: str(value) for key, value in resolved['services']['app']['environment'].items()}
    changed = {key for key, value in desired.items() if old.get(key) != value}
    allowed = {*GIS_ENVIRONMENT, 'MAPS_ARCHIVE_URL'}
    if changed and (not configure_gis or not changed <= allowed):
        raise RuntimeError('Runtime environment changed; review configuration separately')
    expected = dict(old)
    rollback = copy.deepcopy(resolved)
    prior_networks = set(previous.get('NetworkSettings', {}).get('Networks', {}))
    if configure_gis:
        if any(desired.get(key) != value for key, value in GIS_ENVIRONMENT.items()):
            raise RuntimeError('Reviewed GIS service URLs do not match the private network contract')
        archive = desired.get('MAPS_ARCHIVE_URL', '')
        if not re.fullmatch(r'/map-data/[a-f0-9]{12,64}/kazakhstan\.pmtiles', archive):
            raise RuntimeError('Reviewed GIS archive must use a content-versioned same-origin map-data path')
        for key in allowed:
            expected[key] = desired[key]
            environment = rollback['services']['app']['environment']
            if key in old:
                environment[key] = old[key]
            else:
                environment.pop(key, None)
        network_keys = [key for key, value in resolved.get('networks', {}).items()
                        if value.get('name') == GIS_NETWORK and value.get('external') is True]
        if len(network_keys) != 1 or network_keys[0] not in resolved['services']['app'].get('networks', {}):
            raise RuntimeError('Reviewed GIS network must be the explicit external bikerskz_gis network')
        if GIS_NETWORK not in prior_networks:
            rollback['services']['app']['networks'].pop(network_keys[0])
    expected_networks = prior_networks | ({GIS_NETWORK} if configure_gis else set())
    planned_networks = {resolved.get('networks', {}).get(key, {}).get('name', key)
                        for key in resolved['services']['app'].get('networks', {})}
    if planned_networks != expected_networks:
        raise RuntimeError('Runtime networks changed beyond the reviewed GIS addition')
    return expected, expected_networks, rollback, sorted(changed)


def settle_migrator(checkout, name):
    # A failed Compose client can leave the one-off process running. Prove this
    # deployment's container is absent or terminated before classifying receipts.
    containers = run(['docker', 'container', 'ls', '--all', '--filter',
                      f'name=^{name}$', '--format', '{{.ID}}'], cwd=checkout).splitlines()
    if not containers:
        return
    if len(containers) != 1:
        raise RuntimeError('Cannot identify the deployment migration container')
    container = containers[0]
    state = json.loads(run(['docker', 'inspect', container], cwd=checkout))[0]['State']
    if state['Running']:
        run(['docker', 'container', 'stop', '--time', '30', container], cwd=checkout)
        state = json.loads(run(['docker', 'inspect', container], cwd=checkout))[0]['State']
    if state['Running']:
        raise RuntimeError('Cannot confirm the deployment migrator has terminated')
    run(['docker', 'container', 'rm', container], cwd=checkout)


@contextmanager
def deployment_lock(runtime):
    """Share one host lock with a trusted parent process across make/exec."""
    os.umask(0o077)
    path = runtime / '.deploy.lock'
    inherited = os.environ.get('BIKERSKZ_DEPLOY_LOCK_FD')
    if inherited is None:
        handle = path.open('a')
    else:
        if not inherited.isascii() or not inherited.isdigit() or int(inherited) < 3:
            raise RuntimeError('Invalid inherited deployment lock descriptor')
        descriptor = int(inherited)
        expected, actual = path.stat(), os.fstat(descriptor)
        if path.is_symlink() or (actual.st_dev, actual.st_ino) != (expected.st_dev, expected.st_ino):
            raise RuntimeError('Inherited deployment lock does not match this runtime')
        handle = os.fdopen(os.dup(descriptor), 'a')
    try:
        # An inherited descriptor shares the parent's open-file description.
        # Taking flock again still acquires/verifies the real lock; no bypass exists.
        fcntl.flock(handle, fcntl.LOCK_EX | fcntl.LOCK_NB)
        yield handle
    finally:
        handle.close()


def deploy(checkout, runtime, migrate_to=None, configure_gis=False):
    with deployment_lock(runtime):
        _deploy(checkout, runtime, migrate_to, configure_gis)


def _deploy(checkout, runtime, migrate_to=None, configure_gis=False):
    if migrate_to not in (None, 10):
        raise RuntimeError('Only the explicitly reviewed migration target 10 is supported')
    if (migrate_to or configure_gis) and not (runtime / '.deploy/automatic-paused').is_file():
        raise RuntimeError('Pause automatic deployment on Jarvis before the reviewed schema/configuration rollout')
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
    if database['State'].get('Health', {}).get('Status') != 'healthy':
        raise RuntimeError('Existing database must be healthy before deployment')
    target_receipts = migration_checksums(checkout) if migrate_to else None
    previous_receipts = schema_receipts(checkout, db_id) if migrate_to else None
    if migrate_to and previous_receipts != {version: checksum for version, checksum in
                                           target_receipts.items() if version <= 9}:
        raise RuntimeError('Migration-10 mode requires exact source-matching schema-9 receipts')
    original_env = env_file.read_bytes()
    original_compose = json.loads(run(compose + ['config', '--format', 'json'], cwd=checkout))
    if original_compose['services']['app']['image'] != previous['Config']['Image']:
        raise RuntimeError('Runtime APP_IMAGE does not match the running app')
    expected_environment, expected_networks, rollback_compose, configuration_changes = configuration_plan(
        previous, original_compose, configure_gis)
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
    if env_file.read_bytes() != original_env:
        raise RuntimeError('Runtime .env changed during build')

    migration_started = False
    migration_applied = False
    migrator_settled = True
    quiesced_state = None
    new_migration_state = None
    try:
        if migrate_to:
            print('Stopping only the app before the schema-10 upgrade.', flush=True)
            run(compose + ['stop', '--timeout', '30', 'app'], cwd=checkout)
            migration_started = True
            # Fingerprint while all app writers/workers are stopped. Only the
            # intentional nullable ride map column and receipt are excluded.
            quiesced_state = retained_state(checkout, db_id)
        atomic_write(env_file, updated_env)
        run(compose + ['config', '--quiet'], cwd=checkout)
        if migrate_to:
            migration_error = None
            migrator_name = f'bikerskz-migrate-{uuid.uuid4().hex}'
            migrator_settled = False
            try:
                # compose run has no --no-build flag. Omitting --build and
                # forbidding pulls consumes the already verified local image.
                print('Applying the explicit schema-10 upgrade with the verified image.', flush=True)
                run(compose + ['run', '-T', '--name', migrator_name, '--no-deps', '--pull', 'never',
                               'app', 'migrate'], cwd=checkout)
            except (OSError, RuntimeError) as error:
                migration_error = error
            settle_migrator(checkout, migrator_name)
            migrator_settled = True
            observed = schema_receipts(checkout, db_id)
            if observed == target_receipts:
                migration_applied = True
                if migration_error:
                    print('Migration command failed, but exact schema-10 receipts confirm commit; continuing activation.',
                          file=sys.stderr, flush=True)
            elif observed == previous_receipts:
                raise RuntimeError('Migration did not commit; schema 9 remains intact') from migration_error
            else:
                raise RuntimeError('Unexpected migration receipts; app remains stopped for explicit recovery')
            if retained_state(checkout, db_id) != quiesced_state:
                raise RuntimeError('Migration changed retained database state; explicit recovery required')
            new_migration_state = migration_additions(checkout, db_id)
        run(compose + ['up', '-d', '--no-deps', '--no-build', '--wait',
                       '--wait-timeout', '120', 'app'], cwd=checkout)
        current = inspect(run(compose + ['ps', '-q', 'app'], cwd=checkout))
        current_db = inspect(run(compose + ['ps', '-q', 'db'], cwd=checkout))
        unchanged = (
            expected_environment == dict(value.split('=', 1) for value in current['Config']['Env'])
            and previous['Mounts'] == current['Mounts']
            and previous['HostConfig']['PortBindings'] == current['HostConfig']['PortBindings']
            and expected_networks == set(current.get('NetworkSettings', {}).get('Networks', {}))
            and database['Id'] == current_db['Id']
            and database['State']['StartedAt'] == current_db['State']['StartedAt']
        )
        if not unchanged:
            raise RuntimeError('Runtime environment, ports, mounts, networks or database changed')
        if current['Image'] != built['Id'] or current['State']['Health']['Status'] != 'healthy':
            raise RuntimeError('New app did not become healthy with the expected image')
        if current['Config']['User'] != '10001:10001' or not current['HostConfig']['ReadonlyRootfs']:
            raise RuntimeError('App must remain unprivileged with a read-only root filesystem')
    except Exception:
        safe_old_image = not migration_started
        if migration_started:
            # CLI exit status cannot determine whether COMMIT succeeded.
            # Never put the old executable back against schema 10 or an
            # unknown/unreadable receipt state, and never erase new data.
            try:
                safe_old_image = (migrator_settled and
                                  schema_receipts(checkout, db_id) == previous_receipts)
            except Exception:
                safe_old_image = False
        if safe_old_image:
            atomic_write(env_file, original_env)
            print('Activation failed; restoring the previous app image.', file=sys.stderr, flush=True)
            # Resolved configuration stays in memory; it includes private values.
            rollback = compose[:-2] + ['-f', '-']
            run(rollback + ['up', '-d', '--no-deps', '--no-build', '--wait',
                            '--wait-timeout', '120', 'app'], cwd=checkout,
                stdin=json.dumps(rollback_compose))
        else:
            try:
                run(compose + ['stop', '--timeout', '30', 'app'], cwd=checkout)
            except (OSError, RuntimeError):
                print('Could not confirm the target app stopped; inspect it before recovery.',
                      file=sys.stderr, flush=True)
            print('Schema changed or cannot be verified. Keeping the target image selected; '
                  'do not restore the schema-9 image. Explicit roll-forward recovery is required.',
                  file=sys.stderr, flush=True)
        raise

    receipt = {
        'deployed_at': datetime.now(timezone.utc).isoformat(),
        'revision': revision, 'image': image, 'image_id': built['Id'],
        'previous_image': previous['Config']['Image'],
        'checkout': str(checkout), 'database_container_unchanged': True,
        'runtime_configuration_unchanged': not configure_gis,
        'reviewed_gis_configuration': configure_gis, 'configuration_changes': configuration_changes,
        'healthy': True,
    }
    if migrate_to:
        receipt.update({
            'migration_from': 9, 'migration_to': migrate_to,
            'migration_checksum': target_receipts[migrate_to],
            'migration_applied': migration_applied,
            'retained_state_unchanged': True,
            'quiesced_retained_state': quiesced_state,
            'migration_new_state': new_migration_state,
        })
    receipts = runtime / '.deploy'
    receipts.mkdir(mode=0o700, exist_ok=True)
    atomic_write(receipts / 'current-git-release.json',
                 (json.dumps(receipt, indent=2) + '\n').encode())
    print(f'Deployed {revision}; app healthy, database and runtime settings retained.')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--runtime-dir', type=Path,
                        default=Path(os.environ.get('BIKERSKZ_DEPLOY_RUNTIME_DIR', '/home/user/apps/bikerskz')))
    parser.add_argument('--migrate-to', type=int, choices=(10,),
                        help='Explicitly coordinate the reviewed schema 9 to 10 upgrade')
    parser.add_argument('--configure-gis', action='store_true',
                        help='Allow only the reviewed GIS URLs and added private network')
    args = parser.parse_args()
    deploy(Path(__file__).resolve().parent.parent, args.runtime_dir.resolve(), args.migrate_to, args.configure_gis)


if __name__ == '__main__':
    try:
        main()
    except (OSError, RuntimeError, ValueError, KeyError) as error:
        print(f'Deployment stopped: {error}', file=sys.stderr)
        sys.exit(1)

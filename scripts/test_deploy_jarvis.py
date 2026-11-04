"""Exercise deployment ordering/recovery without Docker or production access."""

import copy
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


SPEC = importlib.util.spec_from_file_location('deploy_jarvis', Path(__file__).with_name('deploy-jarvis.py'))
DEPLOY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(DEPLOY)

REVISION = 'a' * 40
OLD_IMAGE = 'bikerskz:git-' + 'b' * 40
NEW_IMAGE = 'bikerskz:git-' + REVISION


def container(image, image_id, container_id):
    return {
        'Id': container_id, 'Image': image_id,
        'Config': {'Image': image, 'Env': ['PORT=8080', 'KEY=private-test-value'],
                   'User': '10001:10001', 'Labels': {'org.opencontainers.image.revision': REVISION}},
        'State': {'StartedAt': '2026-11-04T00:00:00Z', 'Health': {'Status': 'healthy'}},
        'Mounts': [{'Name': 'bikerskz_media', 'Destination': '/var/lib/bikerskz/media'}],
        'HostConfig': {'PortBindings': {'8080/tcp': [{'HostIp': '0.0.0.0', 'HostPort': '3050'}]},
                       'ReadonlyRootfs': True},
    }


class DockerFixture:
    def __init__(self, checkout, runtime):
        self.checkout, self.runtime = checkout, runtime
        self.target = DEPLOY.migration_checksums(checkout)
        self.schema = {version: checksum for version, checksum in self.target.items() if version <= 7}
        self.old = container(OLD_IMAGE, 'old-image-id', 'old-app-id')
        self.current = copy.deepcopy(self.old)
        self.database = container('postgres:18.6-alpine3.24', 'database-image-id', 'db-id')
        self.events = []
        self.commands = []
        self.build_fails = False
        self.activation_fails = False
        self.migration_fails = False
        self.migration_commits = True
        self.unknown_schema = False
        self.receipts_unreadable_after_migration = False
        self.retention_changes = False
        self.activated_environment_changes = False
        self.migration_ran = False
        self.migrator_present = False
        self.migrator_running = False
        self.migrator_lingers = False
        self.migrator_cleanup_fails = False
        self.migrator_lookup_fails = False
        self.migration_commits_when_stopped = False

    def run(self, args, *, cwd, stdin=None, visible=False):
        self.commands.append(list(args))
        if args[:2] == ['git', 'status']:
            return ''
        if args[:2] == ['git', 'rev-parse']:
            return REVISION
        if args[:2] == ['docker', 'info']:
            return 'linux/amd64'
        if args[:2] == ['docker', 'build']:
            self.events.append('build')
            if self.build_fails:
                raise RuntimeError('isolated build failure')
            return ''
        if args[:2] == ['docker', 'inspect']:
            if args[2] == NEW_IMAGE:
                return json.dumps([container(NEW_IMAGE, 'new-image-id', 'new-image-id')])
            if args[2] == 'migration-id':
                return json.dumps([{'State': {'Running': self.migrator_running}}])
            return json.dumps([self.database if args[2] == 'db-id' else self.current])
        if args[:2] == ['docker', 'container']:
            if args[2] == 'ls':
                self.events.append('migrator-lookup')
                assert args[args.index('--filter') + 1].startswith('name=^bikerskz-migrate-')
                if self.migrator_lookup_fails:
                    raise RuntimeError('isolated migrator lookup failure')
                return 'migration-id' if self.migrator_present else ''
            if args[2] == 'stop':
                self.events.append('migrator-stop')
                self.migrator_running = False
                if self.migration_commits_when_stopped:
                    self.schema = dict(self.target)
                return ''
            if args[2] == 'rm':
                self.events.append('migrator-remove')
                if self.migrator_cleanup_fails:
                    raise RuntimeError('isolated migrator cleanup failure')
                assert not self.migrator_running
                self.migrator_present = False
                return ''
        if args[:2] == ['docker', 'exec']:
            if stdin == DEPLOY.SCHEMA_RECEIPTS_SQL:
                self.events.append('receipts')
                if self.migration_ran and self.receipts_unreadable_after_migration:
                    raise RuntimeError('isolated receipt read failure')
                return json.dumps([{'version': version, 'checksum': checksum}
                                   for version, checksum in sorted(self.schema.items())])
            if stdin == DEPLOY.RETAINED_STATE_SQL:
                self.events.append('snapshot')
                changed = self.retention_changes and self.migration_ran
                return '\n'.join((json.dumps({'table': 'sessions', 'rows': 3,
                                              'sha256': ('2' if changed else '1') * 64}),
                                  json.dumps({'sequences': [{'sequencename': 'notifications_sequence_seq',
                                                            'last_value': 5}]})))
            raise AssertionError('Unexpected database command')
        if args[:2] == ['docker', 'compose']:
            if 'ps' in args:
                return 'db-id' if args[-1] == 'db' else 'app-id'
            if 'config' in args:
                if '--format' in args:
                    return json.dumps({'services': {'app': {'image': OLD_IMAGE,
                                                           'environment': {'PORT': '8080', 'KEY': 'private-test-value'}}}})
                return ''
            if 'stop' in args:
                self.events.append('stop')
                return ''
            if 'run' in args:
                self.events.append('migrate')
                self.migration_ran = True
                self.migrator_present = True
                self.migrator_running = self.migrator_lingers
                assert args[-2:] == ['app', 'migrate']
                assert '--no-deps' in args and '--pull' in args and 'never' in args
                assert '--build' not in args and '--service-ports' not in args
                assert '--name' in args and '--rm' not in args
                assert not visible
                assert ('APP_IMAGE=' + NEW_IMAGE).encode() in (self.runtime / '.env').read_bytes()
                if self.migration_commits:
                    self.schema = dict(self.target)
                if self.unknown_schema:
                    self.schema = {9: 'unexpected'}
                if self.migration_fails:
                    raise RuntimeError('isolated migration command failure')
                return ''
            if 'up' in args:
                if stdin is not None:
                    self.events.append('restore')
                    assert json.loads(stdin)['services']['app']['image'] == OLD_IMAGE
                    self.current = copy.deepcopy(self.old)
                    return ''
                self.events.append('activate')
                if self.activation_fails:
                    raise RuntimeError('isolated activation failure')
                self.current = container(NEW_IMAGE, 'new-image-id', 'new-app-id')
                if self.activated_environment_changes:
                    self.current['Config']['Env'] = ['PORT=8080']
                return ''
        raise AssertionError(f'Unexpected command: {args}')


class DeploymentTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        root = Path(self.temporary.name)
        self.checkout = root / 'repo'
        self.runtime = root / 'runtime'
        directory = self.checkout / 'apps/backend/internal/database/postgres'
        directory.mkdir(parents=True)
        self.runtime.mkdir()
        for version, filename in enumerate(DEPLOY.MIGRATION_FILES, 1):
            (directory / filename).write_text(f'-- migration {version}\n')
        manifest = ','.join(f'{{{version}, schema{version}}}' for version in range(1, 9))
        (directory / 'migrations.go').write_text('func migrations() []migration { return []migration{' + manifest + '} }\n')
        self.original = ('APP_IMAGE=' + OLD_IMAGE + '\nPRIVATE_SETTING=private-test-value\n').encode()
        (self.runtime / '.env').write_bytes(self.original)
        (self.runtime / '.env').chmod(0o600)
        self.docker = DockerFixture(self.checkout, self.runtime)
        self.addCleanup(patch.stopall)
        patch.object(DEPLOY, 'run', self.docker.run).start()

    def deploy(self, migrate_to=8):
        previous_umask = os.umask(0o077)
        try:
            DEPLOY.deploy(self.checkout, self.runtime, migrate_to)
        finally:
            os.umask(previous_umask)

    def test_success_builds_before_quiescing_and_preserves_data_before_activation(self):
        self.deploy()
        events = self.docker.events
        self.assertLess(events.index('build'), events.index('stop'))
        self.assertLess(events.index('stop'), events.index('snapshot'))
        self.assertLess(events.index('snapshot'), events.index('migrate'))
        self.assertLess(events.index('migrate'), max(i for i, value in enumerate(events) if value == 'snapshot'))
        self.assertLess(max(i for i, value in enumerate(events) if value == 'snapshot'), events.index('activate'))
        self.assertNotIn('restore', events)
        receipt = json.loads((self.runtime / '.deploy/current-git-release.json').read_text())
        self.assertEqual((receipt['migration_from'], receipt['migration_to']), (7, 8))
        self.assertEqual(receipt['migration_checksum'], self.docker.target[8])
        self.assertTrue(receipt['retained_state_unchanged'])
        self.assertEqual(receipt['quiesced_retained_state']['tables'][0]['rows'], 3)
        self.assertEqual(receipt['image'], NEW_IMAGE)

    def test_build_failure_does_not_stop_app_or_change_environment(self):
        self.docker.build_fails = True
        with self.assertRaisesRegex(RuntimeError, 'build failure'):
            self.deploy()
        self.assertNotIn('stop', self.docker.events)
        self.assertNotIn('restore', self.docker.events)
        self.assertEqual((self.runtime / '.env').read_bytes(), self.original)

    def test_schema_drift_or_future_source_is_rejected_before_build(self):
        self.docker.schema[3] = 'changed'
        with self.assertRaisesRegex(RuntimeError, 'schema-7 receipts'):
            self.deploy()
        self.assertNotIn('build', self.docker.events)
        source = self.checkout / 'apps/backend/internal/database/postgres/migrations.go'
        source.write_text(source.read_text().replace('{8, schema8}', '{8, schema8},{9, schema9}'))
        with self.assertRaisesRegex(RuntimeError, 'exactly source migrations'):
            self.deploy()
        self.assertNotIn('stop', self.docker.events)

    def test_database_must_be_healthy_before_any_build_or_stop(self):
        self.docker.database['State']['Health']['Status'] = 'unhealthy'
        with self.assertRaisesRegex(RuntimeError, 'database must be healthy'):
            self.deploy()
        self.assertEqual(self.docker.events, [])

    def test_uncommitted_migration_failure_restores_only_old_image(self):
        self.docker.migration_fails = True
        self.docker.migration_commits = False
        with self.assertRaisesRegex(RuntimeError, 'schema 7 remains intact'):
            self.deploy()
        self.assertIn('restore', self.docker.events)
        self.assertNotIn('activate', self.docker.events)
        self.assertEqual((self.runtime / '.env').read_bytes(), self.original)
        self.assertFalse((self.runtime / '.deploy/current-git-release.json').exists())

    def test_uncertain_command_exit_continues_only_after_exact_committed_receipts(self):
        self.docker.migration_fails = True
        self.deploy()
        self.assertIn('activate', self.docker.events)
        self.assertNotIn('restore', self.docker.events)

    def test_lingering_migrator_terminates_before_schema_seven_is_classified(self):
        self.docker.migration_fails = True
        self.docker.migration_commits = False
        self.docker.migrator_lingers = True
        with self.assertRaisesRegex(RuntimeError, 'schema 7 remains intact'):
            self.deploy()
        events = self.docker.events
        self.assertLess(events.index('migrator-stop'), events.index('migrator-remove'))
        receipt_reads = [i for i, event in enumerate(events) if event == 'receipts']
        self.assertLess(events.index('migrator-remove'), receipt_reads[1])
        self.assertLess(receipt_reads[1], events.index('restore'))
        self.assertFalse(self.docker.migrator_running)

    def test_commit_during_migrator_termination_keeps_target_image(self):
        self.docker.migration_fails = True
        self.docker.migration_commits = False
        self.docker.migrator_lingers = True
        self.docker.migration_commits_when_stopped = True
        self.deploy()
        self.assertNotIn('restore', self.docker.events)
        self.assertIn('activate', self.docker.events)

    def test_unverified_migrator_cleanup_fails_closed_even_with_schema_seven(self):
        for state in ('migrator_cleanup_fails', 'migrator_lookup_fails'):
            with self.subTest(state=state):
                self.docker = DockerFixture(self.checkout, self.runtime)
                DEPLOY.run = self.docker.run
                self.docker.migration_fails = True
                self.docker.migration_commits = False
                setattr(self.docker, state, True)
                with self.assertRaises(RuntimeError):
                    self.deploy()
                self.assertNotIn('restore', self.docker.events)
                self.assertNotIn('activate', self.docker.events)
                self.assertEqual(self.docker.events[-1], 'stop')
                self.assertIn(('APP_IMAGE=' + NEW_IMAGE).encode(), (self.runtime / '.env').read_bytes())
                (self.runtime / '.env').write_bytes(self.original)

    def test_activation_failure_after_commit_never_restores_old_schema_image(self):
        self.docker.activation_fails = True
        with self.assertRaisesRegex(RuntimeError, 'activation failure'):
            self.deploy()
        self.assertNotIn('restore', self.docker.events)
        self.assertEqual(self.docker.schema, self.docker.target)
        self.assertIn(('APP_IMAGE=' + NEW_IMAGE).encode(), (self.runtime / '.env').read_bytes())

    def test_post_commit_verification_failure_stops_target_before_rollforward(self):
        self.docker.activated_environment_changes = True
        with self.assertRaisesRegex(RuntimeError, 'Runtime environment'):
            self.deploy()
        self.assertNotIn('restore', self.docker.events)
        self.assertEqual(self.docker.events[-1], 'stop')
        self.assertLess(self.docker.events.index('activate'),
                        max(i for i, event in enumerate(self.docker.events) if event == 'stop'))

    def test_unknown_or_unreadable_migration_outcome_never_restores_old_image(self):
        for state in ('unknown_schema', 'receipts_unreadable_after_migration'):
            with self.subTest(state=state):
                self.docker = DockerFixture(self.checkout, self.runtime)
                DEPLOY.run = self.docker.run
                setattr(self.docker, state, True)
                with self.assertRaises(RuntimeError):
                    self.deploy()
                self.assertNotIn('restore', self.docker.events)
                self.assertNotIn('activate', self.docker.events)
                self.assertIn(('APP_IMAGE=' + NEW_IMAGE).encode(), (self.runtime / '.env').read_bytes())
                (self.runtime / '.env').write_bytes(self.original)

    def test_retention_difference_stops_before_activation_without_data_downgrade(self):
        self.docker.retention_changes = True
        with self.assertRaisesRegex(RuntimeError, 'retained database state'):
            self.deploy()
        self.assertNotIn('activate', self.docker.events)
        self.assertNotIn('restore', self.docker.events)

    def test_default_app_only_failure_keeps_existing_rollback_behavior(self):
        self.docker.activation_fails = True
        with self.assertRaisesRegex(RuntimeError, 'activation failure'):
            self.deploy(migrate_to=None)
        self.assertIn('restore', self.docker.events)
        self.assertNotIn('migrate', self.docker.events)
        self.assertNotIn('stop', self.docker.events)
        self.assertEqual((self.runtime / '.env').read_bytes(), self.original)


if __name__ == '__main__':
    unittest.main()

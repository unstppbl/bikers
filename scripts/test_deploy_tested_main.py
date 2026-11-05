"""Exercise the production CI gate and process-lock contract without production access."""

import copy
import fcntl
import importlib.util
import json
import os
import shlex
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch


SCRIPT = Path(__file__).with_name('deploy-tested-main.py')
SPEC = importlib.util.spec_from_file_location('tested_main', SCRIPT)
GATE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(GATE)
REVISION = 'a' * 40
OLD = 'b' * 40
OTHER = 'c' * 40


def ci_run():
    return {'id': 123, 'head_sha': REVISION, 'head_branch': 'main', 'event': 'push',
            'status': 'completed', 'conclusion': 'success', 'path': GATE.WORKFLOW_PATH,
            'name': 'CI', 'repository': {'id': GATE.REPOSITORY_ID},
            'head_repository': {'id': GATE.REPOSITORY_ID}}


def ci_jobs():
    return [{'name': name, 'status': 'completed', 'conclusion': 'success', 'head_sha': REVISION}
            for name in sorted(GATE.REQUIRED_JOBS)]


class GateFixture:
    def __init__(self, root):
        self.checkout, self.runtime = root / 'repo', root / 'runtime'
        directory = self.checkout / 'apps/backend/internal/database/postgres'
        directory.mkdir(parents=True)
        (self.runtime / '.deploy').mkdir(parents=True)
        for index, filename in enumerate(GATE.DEPLOY.MIGRATION_FILES, 1):
            (directory / filename).write_text(f'-- migration {index}\n')
        (directory / 'migrations.go').write_text(','.join(f'{{{i}, schema{i}}}' for i in range(1, 11)))
        self.schema = GATE.DEPLOY.migration_checksums(self.checkout)
        self.main, self.live, self.head = REVISION, OLD, OLD
        self.run_value, self.jobs = ci_run(), ci_jobs()
        self.events = []
        self.requests = []
        self.unhealthy = False
        self.dirty = False
        self.configuration_changed = False
        self.build_fails = False
        self.divergent = False
        self.changed_after_fetch = False
        self.runtime_changed = True
        self.comparisons = []
        self.receipt()

    def receipt(self):
        (self.runtime / '.deploy/current-git-release.json').write_text(json.dumps({
            'revision': self.live, 'image': 'bikerskz:git-' + self.live, 'healthy': True}))

    def require_lock(self):
        with (self.runtime / '.deploy.lock').open('a') as contender:
            try:
                fcntl.flock(contender, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError:
                return
            raise AssertionError('Operation did not hold the deployment lock')

    def github(self, path):
        self.requests.append(path)
        if path.endswith('/git/ref/heads/main'):
            return {'ref': 'refs/heads/main', 'object': {'sha': self.main, 'type': 'commit'}}
        if '/actions/workflows/ci.yml/runs?' in path:
            return {'workflow_runs': [copy.deepcopy(self.run_value)]}
        if path.endswith('/actions/runs/123/jobs?filter=latest&per_page=100'):
            return {'total_count': len(self.jobs), 'jobs': copy.deepcopy(self.jobs)}
        raise AssertionError(f'Unexpected GitHub request: {path}')

    def command(self, args, *, cwd, **_):
        if args[:2] == ['docker', 'inspect']:
            self.require_lock()
            return json.dumps([{'Config': {'Labels': {'org.opencontainers.image.revision': self.live},
                                          'Image': 'bikerskz:git-' + self.live},
                                'State': {'Health': {'Status': 'unhealthy' if self.unhealthy else 'healthy'}}}])
        if args[:4] == ['git', 'remote', 'get-url', 'origin']:
            return 'https://github.com/unstppbl/bikers.git'
        if args[:2] == ['git', 'symbolic-ref']:
            return 'main'
        if args[:2] == ['git', 'status']:
            return ' M local-change' if self.dirty else ''
        if args[:2] == ['git', 'fetch']:
            self.require_lock()
            self.events.append('fetch')
            return ''
        if args[:2] == ['git', 'rev-parse']:
            return self.main if args[-1] == 'origin/main' else self.head
        if args[:2] == ['git', 'merge-base']:
            self.require_lock()
            if self.divergent:
                raise RuntimeError('git merge-base failed')
            return ''
        if args[:2] == ['git', 'diff']:
            return 'docker-compose.yml' if self.configuration_changed else ''
        if args[:2] == ['git', 'merge']:
            self.require_lock()
            self.events.append('fast-forward')
            self.head = REVISION
            if self.changed_after_fetch:
                self.main = OTHER
            return ''
        raise AssertionError(f'Unexpected command: {args}')

    def make(self, checkout, runtime, lock):
        self.require_lock()
        self.events.append('make')
        if self.build_fails:
            raise RuntimeError('isolated build failed')
        self.live = REVISION
        self.receipt()

    def changes(self, checkout, base, revision):
        self.require_lock()
        self.comparisons.append((checkout, base, revision))
        return self.runtime_changed


class GateTests(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        self.fixture = GateFixture(Path(directory.name))
        previous_umask = os.umask(0o077)
        self.addCleanup(os.umask, previous_umask)
        self.addCleanup(patch.stopall)
        patch.dict(os.environ).start()
        os.environ.pop('BIKERSKZ_DEPLOY_LOCK_FD', None)
        patch.object(GATE, 'github', self.fixture.github).start()
        patch.object(GATE.DEPLOY, 'run', self.fixture.command).start()
        patch.object(GATE.DEPLOY, 'schema_receipts', lambda *_: self.fixture.schema).start()
        patch.object(GATE, 'invoke_make', self.fixture.make).start()
        patch.object(GATE, 'has_deployment_changes', self.fixture.changes).start()

    def deploy(self, revision=REVISION, retry=False):
        return GATE.deploy_tested_main(self.fixture.checkout, self.fixture.runtime, revision, retry)

    def test_verified_main_holds_lock_through_fast_forward_and_make_and_checks_live_result(self):
        result = self.deploy()
        self.assertEqual(self.fixture.events, ['fetch', 'fast-forward', 'make'])
        self.assertEqual(result['status'], 'deployed')
        self.assertEqual(result['ci_run_id'], 123)
        receipt = json.loads((self.fixture.runtime / '.deploy/current-automatic-deployment.json').read_text())
        self.assertEqual(receipt['revision'], REVISION)

    def test_running_revision_controls_noop_even_when_checkout_is_behind(self):
        self.fixture.live = REVISION
        self.fixture.receipt()
        result = self.deploy()
        self.assertEqual(result['status'], 'already-deployed')
        self.assertEqual(self.fixture.head, OLD)
        self.assertEqual(self.fixture.events, [])

    def test_documentation_only_check_preserves_live_release_and_records_the_skip(self):
        self.fixture.runtime_changed = False
        release = self.fixture.runtime / '.deploy/current-git-release.json'
        automatic = self.fixture.runtime / '.deploy/current-automatic-deployment.json'
        automatic.write_text(json.dumps({'revision': OLD, 'status': 'deployed', 'ci_run_id': 100}))
        original_release, original_automatic = release.read_bytes(), automatic.read_bytes()
        result = self.deploy()
        self.assertEqual(result['status'], 'no-runtime-changes')
        self.assertEqual(result['revision'], REVISION)
        self.assertEqual(result['running_revision'], OLD)
        self.assertEqual(self.fixture.events, ['fetch', 'fast-forward'])
        self.assertEqual(self.fixture.live, OLD)
        self.assertEqual(self.fixture.head, REVISION)
        self.assertEqual(release.read_bytes(), original_release)
        self.assertEqual(automatic.read_bytes(), original_automatic)
        self.assertEqual(self.fixture.comparisons, [(self.fixture.checkout, OLD, REVISION)])
        self.assertEqual(json.loads((self.fixture.runtime / '.deploy/last-automatic-check.json').read_text()), result)

    def test_pending_code_is_compared_with_live_revision_even_when_checkout_is_current(self):
        self.fixture.head = REVISION
        result = self.deploy()
        self.assertEqual(result['status'], 'deployed')
        self.assertEqual(self.fixture.comparisons, [(self.fixture.checkout, OLD, REVISION)])
        self.assertIn('make', self.fixture.events)

    def test_successful_documentation_retry_clears_failure_for_later_checks(self):
        self.fixture.runtime_changed = False
        failure = self.fixture.runtime / '.deploy/automatic-failure.json'
        failure.write_text(json.dumps({'revision': REVISION, 'reason': 'previous checkout failure'}))
        with self.assertRaisesRegex(RuntimeError, 'previously failed'):
            self.deploy()
        self.assertEqual(self.deploy(retry=True)['status'], 'no-runtime-changes')
        self.assertFalse(failure.exists())
        self.assertEqual(self.deploy()['status'], 'no-runtime-changes')
        self.assertNotIn('make', self.fixture.events)

    def test_stale_or_invalid_revision_never_fetches_or_builds(self):
        for revision in (OTHER, REVISION + ';touch /tmp/unwanted', REVISION.upper(), '', '0' * 39):
            with self.subTest(revision=revision), self.assertRaises(RuntimeError):
                self.deploy(revision)
        self.assertEqual(self.fixture.events, [])

    def test_ci_must_be_exact_public_push_workflow_and_not_fork_or_manual_success(self):
        cases = (('event', 'workflow_dispatch'), ('head_sha', OTHER), ('head_branch', 'topic'),
                 ('conclusion', 'failure'), ('status', 'in_progress'), ('path', '.github/workflows/other.yml'),
                 ('name', 'Other CI'), ('repository', {'id': 1}), ('head_repository', {'id': 1}))
        for key, value in cases:
            with self.subTest(key=key):
                self.fixture.run_value = ci_run()
                self.fixture.run_value[key] = value
                with self.assertRaisesRegex(RuntimeError, 'no successful push CI'):
                    self.deploy()
        self.assertEqual(self.fixture.events, [])

    def test_skipped_missing_duplicate_or_wrong_sha_required_jobs_do_not_qualify(self):
        invalid = []
        skipped = ci_jobs()
        skipped[0]['conclusion'] = 'skipped'
        invalid.append(skipped)
        invalid.append(ci_jobs()[:-1])
        invalid.append(ci_jobs() + [ci_jobs()[0]])
        wrong = ci_jobs()
        wrong[0]['head_sha'] = OTHER
        invalid.append(wrong)
        for jobs in invalid:
            with self.subTest(jobs=jobs):
                self.fixture.jobs = jobs
                with self.assertRaisesRegex(RuntimeError, 'no successful push CI'):
                    self.deploy()
        self.assertEqual(self.fixture.events, [])

    def test_schema_drift_stops_before_make_and_does_not_touch_running_revision(self):
        self.fixture.schema[9] = 'not-source-checksum'
        with self.assertRaisesRegex(RuntimeError, 'explicit reviewed migration'):
            self.deploy()
        self.assertEqual(self.fixture.events, ['fetch', 'fast-forward'])
        self.assertEqual(self.fixture.live, OLD)

    def test_compose_change_requires_manual_rollout_before_checkout_update(self):
        self.fixture.configuration_changed = True
        with self.assertRaisesRegex(RuntimeError, 'Compose configuration changed'):
            self.deploy()
        self.assertEqual(self.fixture.events, ['fetch'])
        self.assertEqual(self.fixture.head, OLD)

    def test_dirty_or_divergent_checkout_is_preserved(self):
        self.fixture.dirty = True
        with self.assertRaisesRegex(RuntimeError, 'dirty'):
            self.deploy()
        self.assertEqual(self.fixture.events, [])
        self.fixture.dirty = False
        self.fixture.divergent = True
        with self.assertRaisesRegex(RuntimeError, 'merge-base'):
            self.deploy(retry=True)
        self.assertEqual(self.fixture.head, OLD)
        self.assertNotIn('make', self.fixture.events)

    def test_failed_revision_is_suppressed_until_explicit_retry(self):
        self.fixture.build_fails = True
        with self.assertRaisesRegex(RuntimeError, 'isolated build failed'):
            self.deploy()
        self.fixture.build_fails = False
        self.fixture.events.clear()
        with self.assertRaisesRegex(RuntimeError, 'previously failed'):
            self.deploy()
        self.assertEqual(self.fixture.events, [])
        self.assertEqual(self.deploy(retry=True)['status'], 'deployed')
        self.assertFalse((self.fixture.runtime / '.deploy/automatic-failure.json').exists())

    def test_pause_marker_stops_without_fetch_or_make(self):
        (self.fixture.runtime / '.deploy/automatic-paused').touch()
        with self.assertRaisesRegex(RuntimeError, 'paused'):
            self.deploy()
        self.assertEqual(self.fixture.events, [])

    def test_main_advance_after_checkout_update_cannot_deploy_stale_commit(self):
        self.fixture.changed_after_fetch = True
        with self.assertRaisesRegex(RuntimeError, 'not current main'):
            self.deploy()
        self.assertEqual(self.fixture.events, ['fetch', 'fast-forward'])
        self.assertEqual(self.fixture.live, OLD)

    def test_unhealthy_app_or_manual_rollback_receipt_mismatch_stops_before_fetch(self):
        self.fixture.unhealthy = True
        with self.assertRaisesRegex(RuntimeError, 'must be healthy'):
            self.deploy()
        self.fixture.unhealthy = False
        self.fixture.live = OTHER
        with self.assertRaisesRegex(RuntimeError, 'release receipt differ'):
            self.deploy(retry=True)
        self.assertEqual(self.fixture.events, [])

    def test_forced_ssh_command_has_no_shell_or_flag_injection_surface(self):
        self.assertEqual(GATE.ssh_command('deploy ' + REVISION), (REVISION, False))
        self.assertEqual(GATE.ssh_command('deploy ' + REVISION + ' --retry'), (REVISION, True))
        for command in ('', 'deploy ' + REVISION + '; id', 'deploy ' + REVISION + '\n',
                        'deploy ' + REVISION + ' --runtime-dir /tmp', 'deploy ' + REVISION + ' retry',
                        'env X=1 deploy ' + REVISION, 'deploy ' + REVISION.upper()):
            with self.subTest(command=command), self.assertRaises(RuntimeError):
                GATE.ssh_command(command)


class LockTests(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        self.runtime = Path(directory.name)
        previous_umask = os.umask(0o077)
        self.addCleanup(os.umask, previous_umask)
        environment = patch.dict(os.environ)
        environment.start()
        self.addCleanup(environment.stop)
        os.environ.pop('BIKERSKZ_DEPLOY_LOCK_FD', None)

    def test_manual_child_cannot_enter_parent_lock_but_inherited_make_child_can(self):
        helper = str(SCRIPT.with_name('deploy-jarvis.py').resolve())
        program = ('import importlib.util,sys;from pathlib import Path;'
                   f's=importlib.util.spec_from_file_location("deploy",{helper!r});'
                   'm=importlib.util.module_from_spec(s);s.loader.exec_module(m);'
                   f'\nwith m.deployment_lock(Path({str(self.runtime)!r})): print("acquired")')
        with GATE.DEPLOY.deployment_lock(self.runtime) as lock:
            blocked = subprocess.run([sys.executable, '-c', program], capture_output=True, text=True)
            self.assertNotEqual(blocked.returncode, 0)
            self.assertIn('BlockingIOError', blocked.stderr)
            environment = dict(os.environ, BIKERSKZ_DEPLOY_LOCK_FD=str(lock.fileno()))
            child = subprocess.run([sys.executable, '-c', program], env=environment,
                                   pass_fds=(lock.fileno(),), capture_output=True, text=True)
            self.assertEqual(child.returncode, 0, child.stderr)
            self.assertEqual(child.stdout.strip(), 'acquired')
            # Closing the child's duplicate must not release the parent's lock.
            blocked = subprocess.run([sys.executable, '-c', program], capture_output=True, text=True)
            self.assertNotEqual(blocked.returncode, 0)

    def test_make_entrypoint_inherits_lock_and_cannot_receive_migration_make_options(self):
        checkout = self.runtime / 'checkout'
        checkout.mkdir()
        report = self.runtime / 'child-report.json'
        helper = str(SCRIPT.with_name('deploy-jarvis.py').resolve())
        program = ("import importlib.util,json,os,sys;from pathlib import Path;"
                   f"s=importlib.util.spec_from_file_location('deploy',{helper!r});"
                   "m=importlib.util.module_from_spec(s);s.loader.exec_module(m);"
                   "\nwith m.deployment_lock(Path(os.environ['BIKERSKZ_DEPLOY_RUNTIME_DIR'])): "
                   f"Path({str(report)!r}).write_text(json.dumps({{'args':sys.argv[1:],"
                   "'makeflags':os.environ.get('MAKEFLAGS'),'deploy_args':os.environ.get('DEPLOY_ARGS'),"
                   "'runtime':os.environ.get('BIKERSKZ_DEPLOY_RUNTIME_DIR')}))")
        (checkout / 'child.py').write_text(program)
        (checkout / 'Makefile').write_text('deploy-jarvis:\n\t' + shlex.quote(sys.executable)
                                         + ' child.py $(DEPLOY_ARGS)\n')
        os.environ['MAKEFLAGS'] = '--just-print'
        os.environ['DEPLOY_ARGS'] = '--migrate-to 9'
        with GATE.DEPLOY.deployment_lock(self.runtime) as lock:
            GATE.invoke_make(checkout, self.runtime, lock)
            with (self.runtime / '.deploy.lock').open('a') as contender:
                with self.assertRaises(BlockingIOError):
                    fcntl.flock(contender, fcntl.LOCK_EX | fcntl.LOCK_NB)
        value = json.loads(report.read_text())
        self.assertEqual(value['args'], [])
        self.assertEqual(value['runtime'], str(self.runtime))
        self.assertNotIn('--just-print', value['makeflags'] or '')
        self.assertNotIn('--migrate-to', value['deploy_args'] or '')

    def test_descriptor_for_another_file_cannot_claim_the_deployment_lock(self):
        with GATE.DEPLOY.deployment_lock(self.runtime), tempfile.TemporaryFile() as wrong:
            os.environ['BIKERSKZ_DEPLOY_LOCK_FD'] = str(wrong.fileno())
            with self.assertRaisesRegex(RuntimeError, 'does not match'):
                with GATE.DEPLOY.deployment_lock(self.runtime):
                    self.fail('Wrong descriptor entered the lock')

    def test_independent_descriptor_of_correct_file_still_must_acquire_real_flock(self):
        with GATE.DEPLOY.deployment_lock(self.runtime), (self.runtime / '.deploy.lock').open('a') as independent:
            os.environ['BIKERSKZ_DEPLOY_LOCK_FD'] = str(independent.fileno())
            with self.assertRaises(BlockingIOError):
                with GATE.DEPLOY.deployment_lock(self.runtime):
                    self.fail('An independent descriptor bypassed the real lock')


if __name__ == '__main__':
    unittest.main()

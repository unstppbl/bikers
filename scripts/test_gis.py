"""Test staging, archive boundaries and cleanup without Docker or host access."""
from contextlib import contextmanager
import importlib.util
import io
import json
import os
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import Mock, patch

SPEC = importlib.util.spec_from_file_location('gis_operations', Path(__file__).with_name('gis.py'))
GIS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(GIS)


class AssetsTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)

    def archive(self, name, entries):
        path = self.root / name
        with tarfile.open(path, 'w:gz') as output:
            for filename, content, kind in entries:
                member = tarfile.TarInfo(filename)
                if kind == 'file':
                    member.size = len(content)
                    output.addfile(member, io.BytesIO(content))
                else:
                    member.type = tarfile.LNKTYPE if kind == 'hardlink' else tarfile.SYMTYPE
                    member.linkname = content
                    output.addfile(member)
        return path

    def valid_archives(self, extra=()):
        fonts = self.archive('fonts.tar.gz', [
            ('./noto_sans_regular/0-255.pbf', b'glyph data', 'file'),
            ('./noto_sans_regular/1024-1279.pbf', './noto_sans_regular/0-255.pbf', 'hardlink'),
            *extra])
        sprites = self.archive('sprites.tar.gz', [
            ('sprites/' + name, b'{}' if name.endswith('.json') else b'\x89PNG\r\n\x1a\ndata', 'file')
            for name in ('base.json', 'base.png', 'base@2x.json', 'base@2x.png')])
        return fonts, sprites

    def test_extracts_expected_faces_and_materializes_validated_hardlinks(self):
        fonts, sprites = self.valid_archives()
        target = self.root / 'public'
        manifest = GIS.extract_assets(fonts, sprites, target)
        first = target / 'fonts/noto_sans_regular/0-255.pbf'
        second = target / 'fonts/noto_sans_regular/1024-1279.pbf'
        self.assertEqual(first.read_bytes(), second.read_bytes())
        self.assertFalse(second.is_symlink())
        self.assertNotEqual(first.stat().st_ino, second.stat().st_ino)
        self.assertEqual(len(manifest), 6)
        self.assertTrue(all(len(item['sha256']) == 64 for item in manifest))

    def test_rejects_traversal_absolute_paths_and_symlinks_even_when_not_selected(self):
        for name, content, kind in [('../escape', b'a', 'file'), ('/escape', b'a', 'file'),
                                     ('unrelated-link', '/etc/passwd', 'symlink')]:
            with self.subTest(name=name):
                fonts, sprites = self.valid_archives([(name, content, kind)])
                with self.assertRaises(ValueError):
                    GIS.extract_assets(fonts, sprites, self.root / ('out-' + str(len(name))))
                self.assertFalse((self.root / 'escape').exists())

    def test_rejects_hardlink_escape_in_selected_glyph(self):
        fonts = self.archive('bad-fonts.tar.gz', [
            ('noto_sans_regular/0-255.pbf', '../outside', 'hardlink')])
        _, sprites = self.valid_archives()
        with self.assertRaisesRegex(ValueError, 'unsafe path'):
            GIS.extract_assets(fonts, sprites, self.root / 'out')

    def test_missing_high_dpi_sprites_or_cyrillic_block_fails_staging(self):
        fonts, _ = self.valid_archives()
        sprites = self.archive('incomplete.tar.gz', [('base.json', b'{}', 'file')])
        with self.assertRaisesRegex(ValueError, 'missing'):
            GIS.extract_assets(fonts, sprites, self.root / 'out')

    def test_asset_downloads_are_pinned_and_bounded(self):
        self.assertEqual(GIS.ASSETS['fonts.tar.gz']['bytes'], 31304823)
        self.assertTrue(all(len(value['sha256']) == 64 and value['url'].startswith('https://')
                            for value in GIS.ASSETS.values()))


class GuardTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.runtime = Path(self.tmp.name) / 'runtime'; self.runtime.mkdir()
        self.control = Path(self.tmp.name) / 'control'; self.control.mkdir()
        self.stopped = Mock()
        self.addCleanup(patch.stopall)
        patch.object(GIS.guard, 'stop_owned', self.stopped).start()
        patch.object(GIS.guard, 'probe', return_value={'okay': True, 'seconds': .003}).start()
        patch.object(GIS.guard, 'host', return_value={'available': 9 * GIS.guard.GIB,
            'disk_free': 30 * GIS.guard.GIB, 'swap_used': 0}).start()
        patch.object(GIS.guard, 'containers', return_value={}).start()
        patch.object(GIS, 'parent_identity', return_value='parent-start').start()
        patch.object(GIS.time, 'sleep').start()
        patch.object(GIS.tempfile, 'mkdtemp', return_value=str(self.control)).start()
        self.stop_gis = patch.object(GIS, 'stop_gis').start()
        self.processes = []

        def launch(args, **_kwargs):
            process = Mock()
            process.poll.return_value = None
            process.wait.return_value = 0
            self.processes.append(process)
            if args[2] == '_watch':
                GIS.guard.save(Path(args[3]) / 'watch.json', {'time': GIS.time.time()})
            return process
        patch.object(GIS.subprocess, 'Popen', side_effect=launch).start()

        def finish(job):
            GIS.guard.save(job / 'finished.json', {'existing_container_changes': [], 'complete_snapshot': True})
        patch.object(GIS.guard, 'finish', side_effect=finish).start()

    def test_success_preserves_full_finish_evidence_and_archives_tmpfs_controls(self):
        with GIS.guarded_job(self.runtime) as job:
            self.assertTrue((job / 'watch.json').is_symlink())
            self.assertEqual(json.loads((job / 'baseline.json').read_text())['readiness_limit'], .25)
        self.assertFalse((job / 'watch.json').is_symlink())
        self.assertTrue(json.loads((job / 'finished.json').read_text())['complete_snapshot'])
        self.assertFalse(self.control.exists())
        self.assertIs(GIS.guard.stop_owned, self.stopped)
        self.stop_gis.assert_not_called()
        self.assertTrue(all(process.wait.called for process in self.processes))

    def test_failed_body_stops_only_explicit_startup_services_and_preserves_abort(self):
        with self.assertRaisesRegex(RuntimeError, 'fixture failure'):
            with GIS.guarded_job(self.runtime, abort_services=('photon',)) as job:
                raise RuntimeError('fixture failure')
        self.assertEqual(json.loads((job / 'abort.json').read_text())['reason'], 'fixture failure')
        self.stop_gis.assert_called_with(self.runtime, ['photon'])
        self.assertIs(GIS.guard.stop_owned, self.stopped)
        self.assertFalse(self.control.exists())

    def test_failed_docker_cleanup_still_joins_children_and_restores_cleanup_function(self):
        self.stopped.side_effect = RuntimeError('Docker cleanup failed')
        with self.assertRaisesRegex(RuntimeError, 'Docker cleanup failed'):
            with GIS.guarded_job(self.runtime) as job:
                pass
        self.assertTrue(all(process.wait.called for process in self.processes))
        self.assertIs(GIS.guard.stop_owned, self.stopped)
        self.assertIn('Docker cleanup failed', (job / 'abort.json').read_text())

    def test_abort_arriving_during_finish_cannot_be_reported_as_success(self):
        def finish(job):
            GIS.guard.save(job / 'abort.json', {'reason': 'simultaneous readiness failure'})
            GIS.guard.save(job / 'finished.json', {'complete_snapshot': True})
        GIS.guard.finish.side_effect = finish
        with self.assertRaisesRegex(RuntimeError, 'aborted before completion'):
            with GIS.guarded_job(self.runtime) as job:
                pass
        self.assertEqual(json.loads((job / 'abort.json').read_text())['reason'], 'simultaneous readiness failure')

    def test_preflight_rejects_slow_ready_before_starting_any_child(self):
        GIS.guard.probe.return_value = {'okay': True, 'seconds': .251}
        with self.assertRaisesRegex(RuntimeError, '250 ms'):
            with GIS.guarded_job(self.runtime):
                self.fail('Unsafe job entered')
        self.assertEqual(self.processes, [])

    def test_lease_parent_death_stops_jobs_and_retains_first_abort_reason(self):
        job = self.runtime / 'job'; job.mkdir()
        GIS.guard.save(job / 'guard-plan.json', {'runtime': str(self.runtime), 'abort_services': ['photon']})
        GIS.guard.save(job / 'abort.json', {'reason': 'first guard reason'})
        with patch.object(GIS.guard, 'ensure_watch'), patch.object(GIS, 'parent_identity', side_effect=FileNotFoundError):
            with self.assertRaises(FileNotFoundError):
                GIS.child_lease(job, 60, 12345, 'parent-start')
        self.assertEqual(json.loads((job / 'abort.json').read_text())['reason'], 'first guard reason')
        self.stop_gis.assert_called_with(self.runtime, ['photon'])
        self.assertTrue(json.loads((job / 'finished.json').read_text())['failed'])


class SelectionTests(unittest.TestCase):
    def test_copied_input_is_hashed_and_growth_is_bounded(self):
        with tempfile.TemporaryDirectory() as tmp:
            source = Path(tmp) / 'source'; source.write_bytes(b'verified input')
            target = Path(tmp) / 'target'
            GIS.copy_verified(source, target, GIS.digest(source))
            self.assertEqual(target.read_bytes(), source.read_bytes())
            with self.assertRaisesRegex(ValueError, 'checksum mismatch'):
                GIS.copy_verified(source, Path(tmp) / 'wrong-hash', '0' * 64)
            with self.assertRaisesRegex(ValueError, 'beyond'):
                GIS.copy_verified(source, Path(tmp) / 'oversized', GIS.digest(source), max_bytes=3)

    def test_environment_write_is_private_atomic_and_rejects_embedded_lines(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / '.env'
            GIS.atomic_env(path, {'PHOTON_DATA_DIR': '/a/dataset'})
            self.assertEqual(GIS.read_env(path), {'PHOTON_DATA_DIR': '/a/dataset'})
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            with self.assertRaises(ValueError):
                GIS.atomic_env(path, {'PHOTON_DATA_DIR': '/a\nEVIL=value'})
            self.assertEqual(GIS.read_env(path), {'PHOTON_DATA_DIR': '/a/dataset'})

    def test_failed_component_switch_restores_previous_selection(self):
        with tempfile.TemporaryDirectory() as tmp:
            runtime = Path(tmp)
            candidate = runtime / 'releases/new'; candidate.mkdir(parents=True)
            (candidate / 'candidate.json').write_text(json.dumps({'component': 'photon'}))
            old = {'PHOTON_DATA_DIR': '/old/index'}
            GIS.atomic_env(runtime / '.env', old)

            @contextmanager
            def job(*_args, **_kwargs):
                yield runtime

            with patch.object(GIS, 'guarded_job', job), patch.object(GIS.guard, 'execute', side_effect=RuntimeError('failed canary')):
                with self.assertRaisesRegex(RuntimeError, 'failed canary'):
                    GIS.switch(runtime, 'photon', candidate)
            self.assertEqual(GIS.read_env(runtime / '.env'), old)

    def test_initial_activation_refuses_occupied_port(self):
        with tempfile.TemporaryDirectory() as tmp:
            runtime = Path(tmp)
            GIS.atomic_env(runtime / '.env', {'GIS_PUBLIC_DIR': '/data'})
            with patch.object(GIS, 'execute', side_effect=['', 'LISTEN unrelated']), patch.object(GIS, 'guarded_job') as guard:
                with self.assertRaisesRegex(RuntimeError, 'already in use'):
                    GIS.activate(runtime)
            guard.assert_not_called()

if __name__ == '__main__':
    unittest.main()

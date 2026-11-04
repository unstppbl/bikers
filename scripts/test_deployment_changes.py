"""Verify deployment relevance against real Git tree differences."""

import importlib.util
from pathlib import Path
import subprocess
import tempfile
import unittest


SPEC = importlib.util.spec_from_file_location(
    'deployment_changes_gate', Path(__file__).with_name('deploy-tested-main.py'))
GATE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(GATE)


class DeploymentChangesTests(unittest.TestCase):
    def setUp(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        self.checkout = Path(directory.name)
        self.git('init', '--quiet', '--initial-branch=main')
        self.git('config', 'user.name', 'Repository Tests')
        self.git('config', 'user.email', 'tests@example.invalid')
        self.write('apps/web/src/main.tsx', 'export const value = 1\n')
        self.write('README.md', 'Repository documentation.\n')
        self.write('docs/guide.txt', 'Operator documentation.\n')
        self.base = self.commit('Initial test tree')

    def git(self, *arguments):
        result = subprocess.run(['git', *arguments], cwd=self.checkout, check=True,
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        return result.stdout.strip()

    def write(self, name, content):
        path = self.checkout / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)

    def commit(self, message):
        self.git('add', '--all')
        self.git('commit', '--quiet', '-m', message)
        return self.git('rev-parse', 'HEAD')

    def changed(self, revision, base=None):
        return GATE.has_deployment_changes(self.checkout, base or self.base, revision)

    def test_identical_tree_needs_no_deployment(self):
        self.assertFalse(self.changed(self.base))

    def test_documentation_addition_deletion_and_rename_need_no_deployment(self):
        self.write('docs/new-manual.txt', 'New manual.\n')
        (self.checkout / 'docs/guide.txt').unlink()
        (self.checkout / 'notes').mkdir()
        self.git('mv', 'README.md', 'notes/README.md')
        self.assertFalse(self.changed(self.commit('Update documentation tree')))

    def test_markdown_suffix_and_agent_directories_are_ignored(self):
        names = ('notes/guide.MD', 'apps/web/guide.mD', '.agents/config.toml',
                 'tools/.codex/hooks.py', 'nested/.claude/commands.sh')
        previous = self.base
        for name in names:
            with self.subTest(name=name):
                self.write(name, 'Instruction content.\n')
                current = self.commit('Add instruction file')
                self.assertFalse(self.changed(current, previous))
                previous = current

    def test_unknown_paths_and_runtime_assets_require_deployment(self):
        names = ('.github/workflows/ci.yml', 'apps/web/src/help.mdx',
                 'apps/web/public/notice.txt', 'new-input.bin')
        previous = self.base
        for name in names:
            with self.subTest(name=name):
                self.write(name, 'New deployment input.\n')
                current = self.commit('Add deployment input')
                self.assertTrue(self.changed(current, previous))
                previous = current

    def test_mixed_code_and_documentation_require_deployment(self):
        self.write('README.md', 'Updated documentation.\n')
        self.write('apps/web/src/main.tsx', 'export const value = 2\n')
        self.assertTrue(self.changed(self.commit('Update code and documentation')))

    def test_moving_code_into_documentation_still_requires_deployment(self):
        self.git('mv', 'apps/web/src/main.tsx', 'docs/main.md')
        current = self.commit('Move code into documentation')
        self.assertIn('R100', self.git('diff', '--name-status', '--find-renames=100%', self.base, current))
        self.assertTrue(self.changed(current))

    def test_docs_followup_cannot_hide_undeployed_code_from_an_earlier_commit(self):
        self.write('apps/web/src/main.tsx', 'export const value = 2\n')
        code_revision = self.commit('Change application code')
        self.write('docs/followup.md', 'Document the change.\n')
        current = self.commit('Document application change')
        self.assertFalse(self.changed(current, code_revision))
        self.assertTrue(self.changed(current))

    def test_reverted_code_and_remaining_documentation_need_no_deployment(self):
        self.write('apps/web/src/main.tsx', 'export const value = 2\n')
        self.commit('Change application code')
        self.write('apps/web/src/main.tsx', 'export const value = 1\n')
        self.write('README.md', 'Document the reverted change.\n')
        self.assertFalse(self.changed(self.commit('Revert code but retain documentation')))

    def test_git_diff_failure_fails_closed(self):
        with self.assertRaises(RuntimeError):
            self.changed('0' * 40)


if __name__ == '__main__':
    unittest.main()

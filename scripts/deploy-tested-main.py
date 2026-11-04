#!/usr/bin/env python3
"""Deploy current main only after independently verifying its successful public CI."""

import argparse
from datetime import datetime, timezone
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import sys
from urllib.parse import urlencode
from urllib.request import Request, urlopen


REPOSITORY = 'unstppbl/bikers'
REPOSITORY_ID = 1441953426
WORKFLOW_PATH = '.github/workflows/ci.yml'
REQUIRED_JOBS = frozenset(('Web', 'Backend', 'Integration', 'Deployment tooling'))
REVISION_PATTERN = re.compile(r'[0-9a-f]{40}', re.ASCII)
SSH_COMMAND_PATTERN = re.compile(r'deploy ([0-9a-f]{40})( --retry)?', re.ASCII)
CHECKOUT = Path('/home/user/apps/bikerskz/repo')
RUNTIME = Path('/home/user/apps/bikerskz')
PROTECTED_CONFIGURATION = ('docker-compose.yml',)
MAX_RESPONSE_BYTES = 2 * 1024 * 1024

SPEC = importlib.util.spec_from_file_location('jarvis_deployment', Path(__file__).with_name('deploy-jarvis.py'))
DEPLOY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(DEPLOY)


def github(path):
    request = Request('https://api.github.com' + path, headers={
        'Accept': 'application/vnd.github+json',
        'X-GitHub-Api-Version': '2022-11-28',
        'User-Agent': 'bikerskz-tested-deployment',
    })
    with urlopen(request, timeout=20) as response:
        payload = response.read(MAX_RESPONSE_BYTES + 1)
    if len(payload) > MAX_RESPONSE_BYTES:
        raise RuntimeError('GitHub response exceeds the deployment gate limit')
    return json.loads(payload)


def current_main():
    value = github(f'/repos/{REPOSITORY}/git/ref/heads/main')
    revision = value['object']['sha']
    if value.get('ref') != 'refs/heads/main' or value['object'].get('type') != 'commit' or not valid_revision(revision):
        raise RuntimeError('GitHub did not return a valid main commit')
    return revision


def valid_revision(value):
    return isinstance(value, str) and REVISION_PATTERN.fullmatch(value) is not None


def successful_ci(revision):
    parameters = urlencode({'branch': 'main', 'event': 'push', 'head_sha': revision,
                            'status': 'success', 'per_page': 10})
    payload = github(f'/repos/{REPOSITORY}/actions/workflows/ci.yml/runs?{parameters}')
    for run in payload['workflow_runs']:
        if not (run.get('head_sha') == revision and run.get('head_branch') == 'main'
                and run.get('event') == 'push' and run.get('status') == 'completed'
                and run.get('conclusion') == 'success' and run.get('path') == WORKFLOW_PATH
                and run.get('name') == 'CI' and run.get('repository', {}).get('id') == REPOSITORY_ID
                and run.get('head_repository', {}).get('id') == REPOSITORY_ID
                and type(run.get('id')) is int):
            continue
        jobs = github(f'/repos/{REPOSITORY}/actions/runs/{run["id"]}/jobs?filter=latest&per_page=100')
        if jobs['total_count'] > 100:
            raise RuntimeError('CI job list exceeds the deployment gate limit')
        required = [job for job in jobs['jobs'] if job.get('name') in REQUIRED_JOBS]
        if (len(required) == len(REQUIRED_JOBS)
                and {job['name'] for job in required} == REQUIRED_JOBS
                and all(job.get('status') == 'completed' and job.get('conclusion') == 'success'
                        and job.get('head_sha') == revision for job in required)):
            return run['id']
    raise RuntimeError('Current main has no successful push CI with all required test jobs')


def require_current_main(revision):
    if current_main() != revision:
        raise RuntimeError('Requested revision is not current main; wait for the newest main CI')


def read_json(path):
    return json.loads(path.read_text())


def live_release(checkout, runtime):
    values = json.loads(DEPLOY.run(['docker', 'inspect', 'bikerskz-app-1'], cwd=checkout))
    if len(values) != 1:
        raise RuntimeError('Cannot identify the running BIKERS.KZ application')
    app = values[0]
    revision = app['Config'].get('Labels', {}).get('org.opencontainers.image.revision')
    if not valid_revision(revision) or app['State'].get('Health', {}).get('Status') != 'healthy':
        raise RuntimeError('Running application must be healthy and identify its Git revision')
    receipt = read_json(runtime / '.deploy/current-git-release.json')
    if receipt.get('revision') != revision or receipt.get('healthy') is not True:
        raise RuntimeError('Live application and release receipt differ; inspect manual rollback state')
    if receipt.get('image') != app['Config']['Image']:
        raise RuntimeError('Live application and release receipt select different images')
    return revision


def prepare_checkout(checkout, revision, live_revision):
    if DEPLOY.run(['git', 'remote', 'get-url', 'origin'], cwd=checkout) != f'https://github.com/{REPOSITORY}.git':
        raise RuntimeError('Deployment checkout must use the expected public GitHub origin')
    if DEPLOY.run(['git', 'symbolic-ref', '--quiet', '--short', 'HEAD'], cwd=checkout) != 'main':
        raise RuntimeError('Deployment checkout must be on main')
    if DEPLOY.run(['git', 'status', '--porcelain', '--untracked-files=all'], cwd=checkout):
        raise RuntimeError('Deployment checkout is dirty; preserve and review local changes')
    DEPLOY.run(['git', 'fetch', '--no-tags', 'origin',
                'refs/heads/main:refs/remotes/origin/main'], cwd=checkout)
    if DEPLOY.run(['git', 'rev-parse', 'origin/main'], cwd=checkout) != revision:
        raise RuntimeError('Main changed while fetching; wait for the newest main CI')
    DEPLOY.run(['git', 'merge-base', '--is-ancestor', 'HEAD', revision], cwd=checkout)
    DEPLOY.run(['git', 'merge-base', '--is-ancestor', live_revision, revision], cwd=checkout)
    changed = DEPLOY.run(['git', 'diff', '--name-only', live_revision, revision,
                         '--', *PROTECTED_CONFIGURATION], cwd=checkout)
    if changed:
        raise RuntimeError('Compose configuration changed; an explicit reviewed rollout is required')
    DEPLOY.run(['git', 'merge', '--ff-only', '--no-edit', revision], cwd=checkout)
    if DEPLOY.run(['git', 'rev-parse', 'HEAD'], cwd=checkout) != revision:
        raise RuntimeError('Deployment checkout did not advance to the validated main revision')


def require_current_schema(checkout):
    expected = DEPLOY.migration_checksums(checkout)
    # Container identity is fixed by the repository Compose project contract.
    if DEPLOY.schema_receipts(checkout, 'bikerskz-db-1') != expected:
        raise RuntimeError('Schema changes require an explicit reviewed migration before automatic deployment')


def invoke_make(checkout, runtime, lock):
    environment = dict(os.environ)
    environment['BIKERSKZ_DEPLOY_LOCK_FD'] = str(lock.fileno())
    environment['BIKERSKZ_DEPLOY_RUNTIME_DIR'] = str(runtime)
    # Do not inherit shell-driven Make options into the fixed deployment command.
    for key in ('MAKEFLAGS', 'MFLAGS', 'MAKEFILES', 'DEPLOY_ARGS'):
        environment.pop(key, None)
    result = subprocess.run(['make', 'deploy-jarvis', 'DEPLOY_ARGS='],
                            cwd=checkout, env=environment, pass_fds=(lock.fileno(),), check=False)
    if result.returncode:
        raise RuntimeError(f'Jarvis deployment command failed (exit {result.returncode})')


def record(path, value):
    DEPLOY.atomic_write(path, (json.dumps(value, indent=2) + '\n').encode())


def deploy_tested_main(checkout, runtime, revision, retry=False):
    if not valid_revision(revision):
        raise RuntimeError('Revision must be exactly 40 lowercase hexadecimal characters')
    require_current_main(revision)
    ci_run = successful_ci(revision)
    with DEPLOY.deployment_lock(runtime) as lock:
        receipts = runtime / '.deploy'
        receipts.mkdir(mode=0o700, exist_ok=True)
        if (receipts / 'automatic-paused').exists():
            raise RuntimeError('Automatic deployment is paused on Jarvis')
        failure = receipts / 'automatic-failure.json'
        if failure.exists() and read_json(failure).get('revision') == revision and not retry:
            raise RuntimeError('This commit previously failed deployment; inspect it and explicitly retry')
        require_current_main(revision)
        try:
            live_revision = live_release(checkout, runtime)
            if live_revision == revision:
                return {'status': 'already-deployed', 'revision': revision, 'ci_run_id': ci_run}
            prepare_checkout(checkout, revision, live_revision)
            require_current_schema(checkout)
            require_current_main(revision)
            invoke_make(checkout, runtime, lock)
            if live_release(checkout, runtime) != revision:
                raise RuntimeError('Deployment completed without the expected healthy live revision')
        except (OSError, RuntimeError, ValueError, KeyError) as error:
            record(failure, {'revision': revision, 'ci_run_id': ci_run,
                             'failed_at': datetime.now(timezone.utc).isoformat(), 'reason': str(error)})
            raise
        result = {'status': 'deployed', 'revision': revision, 'ci_run_id': ci_run,
                  'deployed_at': datetime.now(timezone.utc).isoformat()}
        record(receipts / 'current-automatic-deployment.json', result)
        if failure.exists():
            failure.unlink()
        return result


def ssh_command(value):
    match = SSH_COMMAND_PATTERN.fullmatch(value)
    if match is None:
        raise RuntimeError('Allowed SSH command is deploy <40-character main revision> [--retry]')
    return match[1], bool(match[2])


def main():
    if 'SSH_ORIGINAL_COMMAND' in os.environ:
        revision, retry = ssh_command(os.environ['SSH_ORIGINAL_COMMAND'])
        checkout, runtime = CHECKOUT, RUNTIME
    else:
        parser = argparse.ArgumentParser(description=__doc__)
        parser.add_argument('--revision', required=True)
        parser.add_argument('--retry', action='store_true')
        parser.add_argument('--checkout', type=Path, default=CHECKOUT)
        parser.add_argument('--runtime-dir', type=Path, default=RUNTIME)
        args = parser.parse_args()
        revision, retry = args.revision, args.retry
        checkout, runtime = args.checkout.resolve(), args.runtime_dir.resolve()
    print(json.dumps(deploy_tested_main(checkout, runtime, revision, retry), sort_keys=True))


if __name__ == '__main__':
    try:
        main()
    except (OSError, RuntimeError, ValueError, KeyError) as error:
        print(f'Automatic deployment stopped: {error}', file=sys.stderr)
        sys.exit(1)

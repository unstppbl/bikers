# GitHub Actions deployments

Public application tests run in `unstppbl/bikers`. Production deployment is
controlled by the private `unstppbl/bikers-deploy` repository and its dedicated
homelab runner. The runner is registered only with the private repository.

## Release flow

1. Pull requests and pushes to `main` run the `CI` workflow on GitHub-hosted
   Ubuntu runners. Its required checks are `Web`, `Backend`, `Integration` and
   `Deployment tooling`. PostgreSQL integration uses a disposable service.
2. After successful push CI on this repository's `main`, `Request production
   deployment` creates a short-lived GitHub App installation token. It is
   limited to Actions write permission on `bikers-deploy` and dispatches that
   repository's `deploy.yml` workflow with the tested full commit SHA.
3. The private workflow runs its request validation tests and contacts Jarvis
   through the fixed `jarvis-deploy` SSH alias. Production workflows serialize
   without cancelling an active deployment.
4. The SSH key permits only `deploy <40-character-lowercase-SHA> [--retry]`.
   Its forced command starts a host-managed systemd service, so an SSH or runner
   disconnect does not terminate an activating release.
5. Jarvis independently resolves public `main`, verifies a completed successful
   push run of `.github/workflows/ci.yml` and all four required test jobs, and
   checks that the running image agrees with its healthy release receipt.
6. Under the same host lock used by manual deployments, Jarvis fetches and
   fast-forwards its clean checkout, checks configuration and migration
   receipts, then runs `make deploy-jarvis`. The existing native build, health
   checks, retained runtime/data checks and app-only rollback remain in force.

The public privileged trigger never checks out pull-request code or consumes
its artifacts. A dispatch payload cannot authorize a release: the host gate
checks the source and test evidence itself. An older main run cannot deploy
after a newer main revision has become current.

## GitHub configuration

The deployment App is private to the owner, has no webhook or user OAuth, and
is installed only on `unstppbl/bikers-deploy`. Its permissions are Actions
read/write plus GitHub's mandatory metadata read access. It cannot edit source,
repository secrets or repository administration settings.

The public application repository uses:

| Setting | Purpose |
| --- | --- |
| Variable `DEPLOY_APP_CLIENT_ID` | Public identifier of the deployment App |
| Secret `DEPLOY_APP_PRIVATE_KEY` | Mint a short-lived private deployment token |

No personal GitHub token, production environment file, or homelab SSH key is
stored in the public repository. Protect `main` with the four required CI
checks above and a pull request requirement. Review changes to both workflow
files as production control changes.

The private repository uses the `DEPLOY_PAUSED` variable to stop requests. Its
runner carries the `bikers-deploy` label. The checked-in Dockerfile pins the
official Linux/amd64 runner image; the runner application can update within its
persistent home volume.

## Homelab boundary

The private repository owns the runner Dockerfile, Compose file, request
client, forced SSH wrapper and their tests. Jarvis keeps its runner source and
runtime under `/home/user/apps/bikers-deploy`, separate from the application
checkout. The container runs as UID/GID 1001 with a read-only root filesystem,
a writable runner-home volume, dropped capabilities and no privilege escalation.
It has CPU, memory and process limits and publishes no ports. Its read-only SSH
directory contains a dedicated forced-command key and pinned host key.

The container has no host Docker socket, application checkout, production
`.env`, database files, or personal SSH directory mounted. Its SSH key uses the
existing operator account but cannot request a shell, forwarding, arbitrary
commands or arbitrary deployment flags. The host-managed gate runs as that
operator because the existing deployment procedure requires its Docker access.

The public deployment gate and helper are installed together in
`/home/user/apps/bikerskz/bin`. Private runtime settings and persistent data stay
outside both source repositories.

## Pause, retry and recovery

Pause before a manual rollback, schema upgrade or runtime configuration change:

```sh
gh variable set DEPLOY_PAUSED --repo unstppbl/bikers-deploy --body true
ssh user@100.64.0.3 'touch /home/user/apps/bikerskz/.deploy/automatic-paused'
```

The private variable stops workflow requests; the host marker also stops a
request already queued elsewhere. Neither interrupts an active deployment.
Resume after checking the healthy image, release receipt and intended main:

```sh
ssh user@100.64.0.3 'rm /home/user/apps/bikerskz/.deploy/automatic-paused'
gh variable set DEPLOY_PAUSED --repo unstppbl/bikers-deploy --body false
```

A failed commit is recorded in `.deploy/automatic-failure.json` and is not
repeated automatically. After investigating, retry the current successfully
tested main revision explicitly:

```sh
gh workflow run deploy.yml --repo unstppbl/bikers-deploy \
  -f source_sha=FULL_CURRENT_MAIN_SHA -f retry=true
```

Retry does not bypass CI, main, configuration, schema or health checks.
Compose changes and migrations require an explicit reviewed rollout through
[Operations](operations.md#repository-deployments-on-jarvis). The automatic path
never supplies `--migrate-to`. Existing receipts and migration checksums must
match the source before building or restarting the app.

Review logs in the private Actions run and on Jarvis:

```sh
docker logs --tail 100 bikers-deploy-runner
sudo journalctl --unit 'bikers-auto-deploy-*' --no-pager
cat /home/user/apps/bikerskz/.deploy/current-automatic-deployment.json
```

`current-git-release.json` identifies the healthy running app and its prior
image. `current-automatic-deployment.json` adds the source CI run ID. A manual
rollback that leaves the live image and receipt inconsistent stops automatic
releases until an operator resolves it.

## Validation

Local deployment tests cover successful releases, failed activation, migration
recovery, strict command parsing, source/CI proof, schema and configuration
refusal, pause/retry handling and actual process-lock inheritance through make.
Private tests cover the SSH request boundary and managed service invocation.
The complete GitHub pipeline and its first live release must be verified after
registration and activation; local test success alone does not establish that
GitHub or Jarvis accepted a deployment.

# dockflux Build Plan

## Phase 1 — Project Skeleton
- [x] `go.mod` + `main.go`
- [x] `cmd/root.go` — cobra root, persistent flags, viper binding

## Phase 2 — Data Layer
- [x] `internal/config/config.go` — Config struct, Load, path expansion
- [x] `internal/inventory/inventory.go` — Inventory/Host structs, Load, ResolveHosts
- [x] `internal/lockfile/lockfile.go` — LockFile struct, Load, Save (atomic write), SetEntry, GetEntry

## Phase 3 — Runner Layer
- [x] `internal/runner/runner.go` — Runner interface + New factory
- [x] `internal/runner/local.go` — LocalRunner (os/exec)
- [x] `internal/runner/remote.go` — RemoteRunner (SSH dial, SFTP dir copy, exec, log stream)

## Phase 4 — Git Layer
- [x] `internal/gitops/gitops.go` — CloneOrPull, HeadCommit

## Phase 5 — UI Layer
- [x] `internal/ui/ui.go` — spinner, table renderer, diff renderer, error/success formatting

## Phase 6 — Deploy Orchestration
- [x] `internal/deploy/deploy.go` — parallel dispatch via errgroup, lockfile updates

## Phase 7 — Commands
- [x] `cmd/init.go` — interactive scaffold of dockflux.yml + inventory.yml
- [x] `cmd/sync.go` — clone/pull repo, print HEAD
- [x] `cmd/deploy.go` — deploy a stack to target hosts
- [x] `cmd/undeploy.go` — docker compose down on target hosts
- [x] `cmd/restart.go` — docker compose restart on target hosts
- [x] `cmd/rollback.go` — redeploy prior commit from lockfile
- [x] `cmd/status.go` — table view of lockfile state
- [x] `cmd/diff.go` — compare lockfile vs repo HEAD
- [x] `cmd/logs.go` — stream compose logs from a host
- [x] `cmd/exec.go` — docker compose exec on a host
- [x] `cmd/hosts/hosts.go` + `list.go` + `ping.go`

## Phase 8 — Test Data
- [x] `testdata/dockflux.yml`
- [x] `testdata/inventory.yml`
- [x] `testdata/stacks/nginx/docker-compose.yml`

## Phase 9 — Watch Daemon
- [x] `cmd/watch.go` — poll repo on interval, auto-deploy stale/undeployed stacks
- [x] `internal/reconcile/reconcile.go` — diff + deploy logic shared by watch and diff commands

## Phase 10 — Installation
- [x] `cmd/service/service.go` + `install.go` + `uninstall.go` + `status.go` — systemd unit management
- [x] `Makefile` — `make build`, `make install` (copies binary to /usr/local/bin)

---

## Backlog — Bugs & Missing Features

### P0 — Critical bugs (break core workflows)

- [ ] **Secrets not injected in watch daemon** — `reconcile.go` passes `nil` for `stackSecrets`; any stack requiring `.env` values deploys with empty secrets in daemon mode
- [ ] **HTTPS auth missing in reconciler** — `reconcile.go` calls `gitops.SSHAuth` unconditionally; users who chose HTTPS during `init` get broken auto-deploy
- [ ] **`.env.example` required for secrets injection** — `env.go` silently no-ops if `.env.example` is absent; should write `.env` directly from the secrets store when no example file exists

### P1 — Significant correctness issues

- [ ] **Partial deploy failure discards lockfile updates** — if any host fails, `g.Wait()` errors and the lockfile is not saved, even though some hosts already have the new version running; those hosts are redundantly redeployed next time
- [ ] **Rollback leaves detached HEAD on crash** — `CheckoutCommit` → deploy → `CheckoutBranch`; a crash between the first and last call leaves the repo clone in detached HEAD permanently, breaking subsequent sync/watch ticks
- [ ] **SSH host key verification disabled for git transport** — `gitops.go` sets `InsecureIgnoreHostKey` silently; should check `~/.ssh/known_hosts` or warn loudly the way the SSH runner does

### P1 — High-value missing features

- [ ] **Notifications** — no Slack/webhook/email on deploy success or failure; essential for daemon operation and team awareness
- [ ] **Per-stack watch opt-out** — no way to mark a stack `watch: manual` to prevent auto-deployment by the daemon; databases and stateful services need this

### P2 — Operational gaps

- [ ] **Live container status** — `dockflux status` reads the lockfile only; add a `--live` flag that SSHes to each host and runs `docker compose ps` to show actual container state
- [ ] **Post-deploy health verification** — lockfile records "running" immediately after `docker compose up -d`; should poll `docker compose ps` for a grace period before writing the entry
- [ ] **Shared lockfile for multi-user teams** — lockfile is local to the machine running the CLI; two operators see different state and can overwrite each other; options: commit it to the git repo, or fetch/push it via SFTP from a designated host
- [ ] **SSH passphrase not decrypted in daemon** — watch loop never reads the key passphrase from the secrets store; keys with passphrases cause daemon failure on every tick
- [ ] **No advisory lock between daemon and CLI** — simultaneous `dockflux deploy` and the watch daemon race on the git clone directory and lockfile; add a PID lockfile or `flock`

### P3 — Nice-to-have

- [ ] **Deploy history / audit log** — lockfile only stores current state; add an append-only log of deployments (who, what, when, result) for debugging and compliance
- [ ] **Staged / canary rollout** — all hosts get the new version simultaneously; add `--canary N` to deploy to N hosts first, verify, then continue
- [ ] **Watch backoff + circuit breaker** — a broken compose file causes a retry every 60 s indefinitely; add exponential backoff and a max-attempts cap per stack
- [ ] **Compose file validation before deploy** — run `docker compose config` locally before SFTP copy to catch syntax errors before they reach the host
- [ ] **Multi-environment secrets namespacing** — secrets are keyed by stack name only; staging and production stacks with the same name share a namespace; add an `--env` scoping layer
- [ ] **macOS / launchd service support** — `service install` is systemd-only; add a launchd plist path for macOS users
- [ ] **Image cleanup** — no `docker image prune` offered after deploy; old images accumulate on hosts over time

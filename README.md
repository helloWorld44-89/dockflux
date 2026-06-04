# dockflux

[![CI](https://github.com/darkmode_dev/dockflux/actions/workflows/ci.yml/badge.svg)](https://github.com/darkmode_dev/dockflux/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/darkmode_dev/dockflux)](https://goreportcard.com/report/github.com/darkmode_dev/dockflux)
[![Latest Release](https://img.shields.io/github/v/release/darkmode_dev/dockflux)](https://github.com/darkmode_dev/dockflux/releases/latest)
[![Go Version](https://img.shields.io/badge/go-1.26-00ADD8?logo=go)](go.mod)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

A lightweight, Flux-inspired GitOps tool for managing Docker Compose stacks across a fleet of hosts. Store your stacks in a git repo, point dockflux at it, and it deploys to local or remote hosts over SSH — no agents on the target hosts, no Kubernetes, no complexity.

## Overview

dockflux bridges the GitOps model (git as the source of truth) with the simplicity of Docker Compose. On each reconcile pass it pulls the stacks repo, compares the deployed commit recorded in its lockfile against the current HEAD on each host, and deploys anything that's out of date. Run it as a one-shot CLI command or as a background daemon (`dockflux watch`) that auto-deploys on every git push.

**Why not Flux or ArgoCD?** Those tools are excellent but require a Kubernetes cluster. dockflux targets bare-metal and VM fleets where the stack is `docker compose up`.

## Design decisions

| Concern | Choice | Rationale |
|---|---|---|
| Git | [go-git](https://github.com/go-git/go-git) | Pure Go — no `git` binary required on the operator machine |
| Remote execution | `golang.org/x/crypto/ssh` + [sftp](https://github.com/pkg/sftp) | No `ssh`/`scp` binaries; full context propagation through all SSH operations |
| Parallelism | `golang.org/x/sync/errgroup` | Structured concurrency — all target hosts deploy in parallel, first error cancels the group |
| Secrets | AES-256-GCM + Argon2id | Authenticated encryption; Argon2id (time=3, mem=64 MB, threads=4) makes offline brute force impractical |
| State | Local YAML lockfile, atomic write | Simple and inspectable; atomic `os.Rename` prevents partial writes |
| CLI | Cobra + Viper | Standard Go CLI idiom; persistent flags, shell auto-complete |
| Terminal UI | [pterm](https://github.com/pterm/pterm) | Spinners and color tables without a full TUI framework |

## Architecture

```
cmd/                    Cobra commands — thin wrappers over internal packages
├── hosts/              hosts list, hosts ping
├── secrets/            secrets set / list / delete / env
└── service/            systemd service install / uninstall / status

internal/
├── config/             Load dockflux.yml, home-dir expansion, config file discovery
├── inventory/          Host list, group targeting, per-stack assignment
├── lockfile/           Deployed-state tracking; atomic YAML write via tmp + rename
├── runner/             Runner interface with two implementations:
│   ├── local.go        LocalRunner — os/exec docker compose
│   └── remote.go       RemoteRunner — SSH dial + SFTP directory copy
├── gitops/             CloneOrPull, HeadCommit, Checkout — wraps go-git
├── deploy/             Parallel dispatch via errgroup; updates lockfile on success
├── reconcile/          Watch-loop logic: diff lockfile vs HEAD, deploy stale stacks
├── secrets/            AES-256-GCM encrypted store; .env injection at deploy time
├── importer/           SSH/SFTP import of existing compose stacks from remote hosts
├── ui/                 pterm wrappers — spinner, status table, diff renderer
└── updater/            Background GitHub release check; self-update command
```

### Deploy flow

```
dockflux deploy <stack> --all
  → config.Load            reads dockflux.yml
  → inventory.Load         reads inventory.yml, resolves target hosts
  → secrets.Load           decrypts secrets.enc with Argon2id-derived key
  → gitops.HeadCommit      reads repo HEAD SHA
  → deploy.Run             parallel across hosts (errgroup)
      → secrets.InjectEnv  writes .env from store, registers cleanup
      → runner.CopyStack   SFTP walk → remote host (or no-op for local)
      → runner.ComposeRun  SSH: docker compose up -d
      → lockfile.SetEntry  records commit + timestamp (mutex-protected)
      → cleanup()          removes temporary .env from local cache
  → lockfile.Save          atomic write via os.Rename
```

## Install

```bash
curl -sSL https://raw.githubusercontent.com/darkmode_dev/dockflux/master/install.sh | bash
```

Supports Linux and macOS on amd64 and arm64. The installer downloads the correct binary, verifies its SHA256 checksum, and places it in `/usr/local/bin`.

## Quick start

```bash
# Interactive setup — creates dockflux.yml, inventory.yml, and secrets store
dockflux init

# Pull (or clone) the stacks repo
dockflux sync

# Deploy a stack to all hosts assigned to it
dockflux deploy nginx --all

# Show what is deployed where
dockflux status

# Run the reconciliation daemon (auto-deploys on git push)
dockflux watch
```

## Configuration

### `dockflux.yml`

```yaml
repo:
  url: git@github.com:org/stacks.git
  branch: main
  local_path: ~/.dockflux/repo
  key: ~/.ssh/id_ed25519     # omit to use ssh-agent or a public repo

stacks_dir: stacks/          # subdirectory inside the repo that contains stack dirs
state_file: ./dockflux.lock  # lockfile path (relative to this config file)
inventory: ./inventory.yml   # host list path
secrets_file: ~/.dockflux/secrets.enc
```

### `inventory.yml`

```yaml
hosts:
  local:
    type: local              # runs on this machine — no SSH

  web01:
    host: 192.168.1.10
    port: 22                 # default
    user: deploy
    key: ~/.ssh/id_ed25519
    groups:
      - production
      - web
    compose_dir: /opt/stacks
    stacks:                  # omit to run every stack in the repo
      - nginx
      - certbot
```

Hosts with no `stacks:` list run every stack — the safe default for single-host setups. Hosts with an explicit list only receive stacks named in it.

## Commands

| Command | Description |
|---|---|
| `init` | Interactive setup wizard |
| `sync` | Clone or pull the stacks repo |
| `deploy <stack>` | Deploy a stack (`--pull` to refresh images first) |
| `undeploy <stack>` | Tear down a stack (`docker compose down`) |
| `restart <stack>` | Restart without redeploying |
| `rollback <stack>` | Re-deploy the previous commit recorded in the lockfile |
| `diff` | Show stacks that are stale or not yet deployed |
| `status` | Table of every deployed stack, host, commit, and age |
| `logs <stack>` | Stream `docker compose logs` from a host (`-f` to follow) |
| `exec <stack>` | Run a command inside a running service container |
| `watch` | Poll the repo and auto-deploy stale stacks |
| `import` | Import existing compose stacks from remote hosts |
| `hosts list` | List configured hosts |
| `hosts ping` | Check SSH connectivity to all hosts |
| `secrets set/list/delete` | Manage the encrypted secrets store |
| `service install/uninstall/status` | Manage the dockflux systemd daemon |
| `update` | Self-update to the latest release |

### Targeting flags

`deploy`, `undeploy`, `restart`, `rollback`, and `watch` all accept:

```
--all              every host in inventory
--group <name>     all hosts in a named group
--host <name>      a single named host
--local            the local host entry
--dry-run          print what would happen; deploy nothing
```

## Secrets

Secrets are stored encrypted at rest using AES-256-GCM. The encryption key is derived from a master password via Argon2id (time=3, memory=64 MB, threads=4). The file format is `salt | nonce | ciphertext`; a fresh random salt and nonce are generated on every save, so identical plaintext produces distinct ciphertext each time.

```bash
# Store a secret
dockflux secrets set myapp DATABASE_URL "postgres://prod/db"

# Secrets are injected at deploy time via a temporary .env
dockflux deploy myapp --all
```

At deploy time, secrets are written to a `.env` file alongside the compose file in the local repo cache, the stack directory is copied to the host over SFTP, and the `.env` is removed from the local cache immediately after. If a `.env.example` exists in the stack directory it acts as a template — only keys listed in it are written, preserving placeholder values for unset secrets. If no `.env.example` exists, all secrets for the stack are written directly.

The master password can be supplied via the `DOCKFLUX_SECRETS_PASSWORD` environment variable for non-interactive use (CI, daemon mode).

## Development

```bash
make build      # compile → ./dockflux
make install    # compile and copy to /usr/local/bin
make test       # go test ./...
make clean      # remove compiled binary
```

## Updating

```bash
dockflux update
```

dockflux checks for newer releases on every command invocation (asynchronously, in the background) and prints a notice when one is available.

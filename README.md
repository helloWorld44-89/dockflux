# dockflux

A lightweight, Flux-inspired deployment tool for managing Docker Compose stacks across a fleet of hosts. Define your stacks in a git repo, point dockflux at it, and deploy to local or remote hosts over SSH — no agents, no Kubernetes, no complexity.

## How it works

1. Store your Docker Compose stacks in a git repo (one directory per stack)
2. Run `dockflux sync` to pull the repo to your control machine
3. Run `dockflux deploy <stack>` to push and start a stack on one or more hosts
4. Optionally run `dockflux watch` to auto-deploy whenever the repo changes

State is tracked in a local lockfile so you always know what commit is running where.

## Install

```bash
curl -sSL https://raw.githubusercontent.com/helloWorld44-89/dockflux/master/install.sh | bash
```

Supports Linux and macOS on amd64 and arm64. The installer downloads the correct binary, verifies its SHA256 checksum, and places it in `/usr/local/bin`.

## Quick start

```bash
dockflux init        # interactive setup — creates dockflux.yml, inventory.yml, secrets store
dockflux sync        # clone or pull the stacks repo
dockflux deploy <stack>   # deploy a stack to one or more hosts
dockflux status      # show what is deployed where
```

## Commands

| Command | Description |
|---|---|
| `init` | Interactive setup wizard |
| `sync` | Clone or pull the stacks git repo |
| `deploy <stack>` | Deploy a stack to one or more hosts |
| `undeploy <stack>` | Tear down a stack |
| `restart <stack>` | Restart a stack |
| `rollback <stack>` | Redeploy the previously deployed commit |
| `diff` | Show what would change if you deployed now |
| `status` | Show what is deployed where |
| `logs <stack>` | Stream docker compose logs from a host |
| `exec <stack>` | Run a command inside a running service container |
| `watch` | Poll the repo and auto-deploy stale stacks |
| `import` | Import existing compose stacks from remote hosts |
| `hosts list` | List configured hosts |
| `hosts ping` | Check SSH connectivity to hosts |
| `secrets set/list/delete` | Manage encrypted secrets for stacks |
| `service install/uninstall/status` | Manage the dockflux systemd service |
| `update` | Update dockflux to the latest release |

## Updating

```bash
dockflux update
```

dockflux will also notify you automatically when a newer version is available.

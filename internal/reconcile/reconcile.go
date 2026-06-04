package reconcile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/helloWorld44-89/dockflux/internal/config"
	"github.com/helloWorld44-89/dockflux/internal/deploy"
	"github.com/helloWorld44-89/dockflux/internal/gitops"
	"github.com/helloWorld44-89/dockflux/internal/inventory"
	"github.com/helloWorld44-89/dockflux/internal/lockfile"
	"github.com/helloWorld44-89/dockflux/internal/runner"
	"github.com/helloWorld44-89/dockflux/internal/secrets"
	"github.com/helloWorld44-89/dockflux/internal/ui"
)

// Result holds the outcome of a single reconcile pass.
type Result struct {
	Head      string
	Deployed  []string // stacks that were deployed this pass
	Skipped   []string // stacks already at HEAD
	Failed    []string // stacks that errored
}

// Run performs one reconcile pass: sync repo, reload inventory, find stale/undeployed
// stacks, and deploy them to their assigned hosts. If dryRun is true, nothing is deployed.
// Targeting flags mirror the CLI flags; when all are zero-values --all is assumed.
// secretsStore, if non-nil, is used to inject per-stack secrets at deploy time.
func Run(ctx context.Context, cfg *config.Config, hostFlag, groupFlag string, allFlag, localFlag bool, dryRun bool, secretsStore *secrets.Store) (*Result, error) {
	// Build git auth: SSH key if configured, nil otherwise (public repo or ssh-agent).
	auth, err := gitops.SSHAuth(cfg.Repo.Key)
	if err != nil {
		return nil, err
	}
	if err := gitops.CloneOrPull(cfg.Repo.URL, cfg.Repo.Branch, cfg.Repo.LocalPath, auth); err != nil {
		return nil, err
	}

	// Reload inventory from the repo after pull so new stack assignments take effect.
	inv, err := inventory.Load(cfg.Inventory)
	if err != nil {
		return nil, fmt.Errorf("loading inventory: %w", err)
	}
	if hostFlag == "" && groupFlag == "" && !localFlag {
		allFlag = true
	}
	hosts, err := inventory.ResolveHosts(inv, hostFlag, groupFlag, allFlag, localFlag)
	if err != nil {
		return nil, err
	}

	head, err := gitops.HeadCommit(cfg.Repo.LocalPath)
	if err != nil {
		return nil, err
	}

	lf, err := lockfile.Load(cfg.StateFile)
	if err != nil {
		return nil, err
	}

	result := &Result{Head: head}

	// Walk every stack directory in the repo
	stacksRoot := filepath.Join(cfg.Repo.LocalPath, cfg.StacksDir)
	entries, err := os.ReadDir(stacksRoot)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		stackName := entry.Name()

		stackHosts := inventory.FilterForStack(hosts, stackName)
		if len(stackHosts) == 0 {
			result.Skipped = append(result.Skipped, stackName)
			continue
		}

		if isUpToDate(lf, stackName, stackHosts, head) {
			result.Skipped = append(result.Skipped, stackName)
			continue
		}

		ui.Info("Deploying %s @ %s", stackName, head)

		opts := runner.RunOptions{
			Stack:     stackName,
			StackPath: filepath.Join(stacksRoot, stackName),
			Action:    runner.ActionUp,
			DryRun:    dryRun,
			Commit:    head,
		}

		var stackSecrets map[string]string
		if secretsStore != nil {
			stackSecrets = secretsStore.GetStackSecrets(stackName)
		}
		if err := deploy.Run(ctx, stackHosts, opts, lf, cfg.StateFile, stackSecrets); err != nil {
			ui.Error("Failed to deploy %s: %v", stackName, err)
			result.Failed = append(result.Failed, stackName)
			continue
		}

		result.Deployed = append(result.Deployed, stackName)
	}

	return result, nil
}

// isUpToDate returns true only if every target host has the stack at HEAD.
func isUpToDate(lf *lockfile.LockFile, stack string, hosts []*inventory.Host, head string) bool {
	for _, h := range hosts {
		entry := lf.GetEntry(stack, h.Name)
		if entry == nil || entry.Commit != head {
			return false
		}
	}
	return true
}

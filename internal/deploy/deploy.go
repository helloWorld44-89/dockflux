package deploy

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/helloWorld44-89/dockflux/internal/inventory"
	"github.com/helloWorld44-89/dockflux/internal/lockfile"
	"github.com/helloWorld44-89/dockflux/internal/runner"
	"github.com/helloWorld44-89/dockflux/internal/secrets"
	"github.com/helloWorld44-89/dockflux/internal/ui"
	"golang.org/x/sync/errgroup"
)

// Run deploys (or undeploys/restarts) a stack across the given hosts in parallel.
// On success it updates the lockfile and saves it to lfPath.
// stackSecrets are injected into .env before the stack files are copied.
func Run(
	ctx context.Context,
	hosts []*inventory.Host,
	opts runner.RunOptions,
	lf *lockfile.LockFile,
	lfPath string,
	stackSecrets map[string]string,
) error {
	// Inject .env from secrets + .env.example before any host copies
	if opts.Action != runner.ActionDown && len(stackSecrets) > 0 {
		cleanup, err := secrets.InjectEnv(opts.StackPath, stackSecrets)
		if err != nil {
			return fmt.Errorf("injecting .env: %w", err)
		}
		defer cleanup()
	}
	g, gctx := errgroup.WithContext(ctx)
	var mu sync.Mutex

	for _, h := range hosts {
		h := h
		g.Go(func() error {
			stop := ui.Spinner(fmt.Sprintf("host %s: %s %s", h.Name, opts.Action, opts.Stack))

			r, err := runner.New(h)
			if err != nil {
				stop(false, fmt.Sprintf("host %s: runner init failed", h.Name))
				return fmt.Errorf("host %s: %w", h.Name, err)
			}

			hostOpts := opts
			hostOpts.ComposeDir = h.ComposeDir
			if h.Type == inventory.HostTypeLocal {
				hostOpts.ComposeDir = opts.StackPath
			}

			if err := r.CopyStack(gctx, hostOpts); err != nil {
				stop(false, fmt.Sprintf("host %s: copy failed", h.Name))
				return fmt.Errorf("host %s copy: %w", h.Name, err)
			}

			out, err := r.ComposeRun(gctx, hostOpts)
			if err != nil {
				stop(false, fmt.Sprintf("host %s: compose failed\n%s", h.Name, out))
				return fmt.Errorf("host %s compose: %w", h.Name, err)
			}

			status := statusFromAction(opts.Action)
			stop(true, fmt.Sprintf("host %s: %s", h.Name, status))

			if !opts.DryRun {
				entry := &lockfile.LockEntry{
					Commit:     opts.Commit,
					DeployedAt: time.Now().UTC(),
					Status:     status,
				}
				mu.Lock()
				if opts.Action == runner.ActionDown {
					lf.RemoveEntry(opts.Stack, h.Name)
				} else {
					lf.SetEntry(opts.Stack, h.Name, entry)
				}
				mu.Unlock()
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	// Fix #5: save lockfile for all actions including restart
	if !opts.DryRun {
		return lockfile.Save(lfPath, lf)
	}
	return nil
}

func statusFromAction(action runner.ComposeAction) string {
	switch action {
	case runner.ActionDown:
		return "stopped"
	case runner.ActionRestart:
		return "running"
	default:
		return "running"
	}
}

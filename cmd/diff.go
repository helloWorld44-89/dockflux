package cmd

import (
	"os"
	"path/filepath"

	"github.com/darkmode_dev/dockflux/internal/gitops"
	"github.com/darkmode_dev/dockflux/internal/lockfile"
	"github.com/darkmode_dev/dockflux/internal/ui"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show what would change if you deployed now",
	RunE:  runDiff,
}

func runDiff(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	lf, err := lockfile.Load(cfg.StateFile)
	if err != nil {
		return err
	}

	head, err := gitops.HeadCommit(cfg.Repo.LocalPath)
	if err != nil {
		return err
	}

	var rows []ui.DiffRow

	// Check deployed stacks for staleness
	for stackName, state := range lf.Stacks {
		for hostName, entry := range state {
			if entry.Commit != head {
				rows = append(rows, ui.DiffRow{
					Stack:          stackName,
					Host:           hostName,
					DeployedCommit: entry.Commit,
					HeadCommit:     head,
					State:          "stale",
				})
			}
		}
	}

	// Check repo for stacks not in the lockfile at all
	stacksRoot := filepath.Join(cfg.Repo.LocalPath, cfg.StacksDir)
	entries, err := os.ReadDir(stacksRoot)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if _, deployed := lf.Stacks[e.Name()]; !deployed {
				rows = append(rows, ui.DiffRow{
					Stack:      e.Name(),
					Host:       "-",
					HeadCommit: head,
					State:      "undeployed",
				})
			}
		}
	}

	ui.DiffTable(rows)
	return nil
}

package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/darkmode_dev/dockflux/internal/deploy"
	"github.com/darkmode_dev/dockflux/internal/gitops"
	"github.com/darkmode_dev/dockflux/internal/inventory"
	"github.com/darkmode_dev/dockflux/internal/lockfile"
	"github.com/darkmode_dev/dockflux/internal/runner"
	"github.com/darkmode_dev/dockflux/internal/ui"
	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback <stack>",
	Short: "Redeploy the previously deployed commit of a stack",
	Args:  cobra.ExactArgs(1),
	RunE:  runRollback,
}

func init() {
	addTargetFlags(rollbackCmd)
	rollbackCmd.Flags().String("commit", "", "Specific commit SHA to roll back to")
}

func runRollback(cmd *cobra.Command, args []string) error {
	stackName := args[0]

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	inv, err := inventory.Load(cfg.Inventory)
	if err != nil {
		return err
	}

	hosts, err := resolveTargets(cmd, inv)
	if err != nil {
		return err
	}

	lf, err := lockfile.Load(cfg.StateFile)
	if err != nil {
		return err
	}

	commitFlag, _ := cmd.Flags().GetString("commit")
	targetCommit := commitFlag

	if targetCommit == "" {
		targetCommit, err = resolveRollbackCommit(lf, stackName, hosts)
		if err != nil {
			return err
		}
	}

	ui.Info("Rolling back %s to commit %s", stackName, targetCommit)

	if err := gitops.CheckoutCommit(cfg.Repo.LocalPath, targetCommit); err != nil {
		return fmt.Errorf("checking out %s: %w", targetCommit, err)
	}

	stackPath := filepath.Join(cfg.Repo.LocalPath, cfg.StacksDir, stackName)
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	stackSecrets, err := loadStackSecrets(cfg.SecretsFile, stackName)
	if err != nil {
		return err
	}

	opts := runner.RunOptions{
		Stack:     stackName,
		StackPath: stackPath,
		Action:    runner.ActionUp,
		DryRun:    dryRun,
		Commit:    targetCommit,
	}

	deployErr := deploy.Run(cmd.Context(), hosts, opts, lf, cfg.StateFile, stackSecrets)

	// Always restore HEAD, even if deploy failed
	if restoreErr := gitops.CheckoutBranch(cfg.Repo.LocalPath, cfg.Repo.Branch); restoreErr != nil {
		ui.Warn("Could not restore branch %s: %v", cfg.Repo.Branch, restoreErr)
	}

	if deployErr != nil {
		return fmt.Errorf("rollback deploy failed: %w", deployErr)
	}

	ui.Success("Rolled back %s to %s", stackName, targetCommit)
	return nil
}

func resolveRollbackCommit(lf *lockfile.LockFile, stack string, hosts []*inventory.Host) (string, error) {
	var commit string
	for _, h := range hosts {
		entry := lf.GetEntry(stack, h.Name)
		if entry == nil {
			return "", fmt.Errorf("no deployment record for stack %q on host %q", stack, h.Name)
		}
		if commit == "" {
			commit = entry.Commit
		} else if commit != entry.Commit {
			return "", fmt.Errorf(
				"hosts have different deployed commits for stack %q; use --commit to specify one",
				stack,
			)
		}
	}
	if commit == "" {
		return "", fmt.Errorf("no deployed commit found for stack %q", stack)
	}
	return commit, nil
}

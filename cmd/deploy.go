package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/helloWorld44-89/dockflux/internal/deploy"
	"github.com/helloWorld44-89/dockflux/internal/gitops"
	"github.com/helloWorld44-89/dockflux/internal/inventory"
	"github.com/helloWorld44-89/dockflux/internal/lockfile"
	"github.com/helloWorld44-89/dockflux/internal/runner"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy <stack>",
	Short: "Deploy a stack to one or more hosts",
	Args:  cobra.ExactArgs(1),
	RunE:  runDeploy,
}

func init() {
	addTargetFlags(deployCmd)
	deployCmd.Flags().Bool("pull", false, "Pull images before deploying")
}

func runDeploy(cmd *cobra.Command, args []string) error {
	return runComposeAction(cmd, args, runner.ActionUp)
}

// runComposeAction is shared by deploy, undeploy, and restart.
func runComposeAction(cmd *cobra.Command, args []string, action runner.ComposeAction) error {
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

	commit, err := gitops.HeadCommit(cfg.Repo.LocalPath)
	if err != nil {
		return fmt.Errorf("could not read HEAD commit (run 'dockflux sync' first): %w", err)
	}

	stackPath := filepath.Join(cfg.Repo.LocalPath, cfg.StacksDir, stackName)

	pull, _ := cmd.Flags().GetBool("pull")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	stackSecrets, err := loadStackSecrets(cfg.SecretsFile, stackName)
	if err != nil {
		return err
	}

	opts := runner.RunOptions{
		Stack:     stackName,
		StackPath: stackPath,
		Action:    action,
		Pull:      pull,
		DryRun:    dryRun,
		Commit:    commit,
	}

	return deploy.Run(cmd.Context(), hosts, opts, lf, cfg.StateFile, stackSecrets)
}

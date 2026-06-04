package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/darkmode_dev/dockflux/internal/config"
	"github.com/darkmode_dev/dockflux/internal/reconcile"
	"github.com/darkmode_dev/dockflux/internal/secrets"
	"github.com/darkmode_dev/dockflux/internal/ui"
	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Poll the repo and auto-deploy stale stacks",
	Long: `watch runs a continuous reconciliation loop. On each tick it:
  1. Pulls the latest commits from the stacks repo
  2. Compares each stack's deployed commit against HEAD
  3. Deploys any stack that is stale or not yet deployed

Targeting flags (--all, --group, --host, --local) control which hosts
are reconciled. If no targeting flag is provided, --all is assumed.`,
	RunE: runWatch,
}

func init() {
	addTargetFlags(watchCmd)
	watchCmd.Flags().Duration("interval", 60*time.Second, "How often to poll the repo")
	rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, args []string) error {
	interval, _ := cmd.Flags().GetDuration("interval")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Load secrets once at startup so the daemon doesn't prompt on every tick.
	var secretsStore *secrets.Store
	if _, statErr := os.Stat(cfg.SecretsFile); statErr == nil {
		password, err := secrets.PromptPassword("Secrets master password")
		if err != nil {
			return err
		}
		secretsStore, err = secrets.Load(cfg.SecretsFile, password)
		if err != nil {
			return err
		}
	}

	ui.Info("dockflux watch started — interval: %s, dry-run: %v", interval, dryRun)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on start, then on each tick.
	tick(cmd, cfg, dryRun, secretsStore)

	for {
		select {
		case <-ticker.C:
			tick(cmd, cfg, dryRun, secretsStore)
		case <-cmd.Context().Done():
			ui.Info("watch stopped")
			return nil
		}
	}
}

func tick(cmd *cobra.Command, cfg *config.Config, dryRun bool, secretsStore *secrets.Store) {
	hostFlag, _ := cmd.Flags().GetString("host")
	groupFlag, _ := cmd.Flags().GetString("group")
	allFlag, _ := cmd.Flags().GetBool("all")
	localFlag, _ := cmd.Flags().GetBool("local")

	ts := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("\n[%s] reconciling...\n", ts)

	result, err := reconcile.Run(cmd.Context(), cfg, hostFlag, groupFlag, allFlag, localFlag, dryRun, secretsStore)
	if err != nil {
		ui.Error("reconcile error: %v", err)
		return
	}

	if len(result.Deployed) > 0 {
		ui.Success("deployed: %v @ %s", result.Deployed, result.Head)
	}
	if len(result.Failed) > 0 {
		ui.Error("failed: %v", result.Failed)
	}
	if len(result.Skipped) > 0 {
		ui.Info("up to date: %v", result.Skipped)
	}
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jconder44/dockflux/internal/importer"
	"github.com/jconder44/dockflux/internal/inventory"
	"github.com/jconder44/dockflux/internal/ui"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import existing Docker Compose stacks from remote hosts into the local repo",
	Long: `Connects to each target host via SSH/SFTP, discovers stack directories under
compose_dir, and downloads compose files into the local stacks directory.

.env files are intentionally skipped. Store secrets with 'dockflux secrets set'.`,
	RunE: runImport,
}

func init() {
	addTargetFlags(importCmd)
	importCmd.Flags().Bool("force", false, "Overwrite stacks that already exist locally")
}

func runImport(cmd *cobra.Command, args []string) error {
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

	force, _ := cmd.Flags().GetBool("force")
	stacksDir := filepath.Join(cfg.Repo.LocalPath, cfg.StacksDir)

	if err := os.MkdirAll(stacksDir, 0755); err != nil {
		return fmt.Errorf("creating stacks dir: %w", err)
	}

	totalImported, totalSkipped, totalEmpty := 0, 0, 0

	for _, host := range hosts {
		if host.Type == inventory.HostTypeLocal {
			ui.Warn("Skipping local host — nothing to import via SSH")
			continue
		}

		stop := ui.Spinner(fmt.Sprintf("Connecting to %s (%s)", host.Name, host.Host))
		results, err := importer.ImportFromHost(cmd.Context(), host, stacksDir, force)
		if err != nil {
			stop(false, fmt.Sprintf("%s: %v", host.Name, err))
			continue
		}
		stop(true, fmt.Sprintf("%s: %d stack(s) discovered", host.Name, len(results)))

		for _, r := range results {
			switch {
			case r.Skipped:
				totalSkipped++
				pterm.Warning.Printf("  %-24s already exists locally (use --force to overwrite)\n", r.Stack)
			case len(r.Files) == 0:
				totalEmpty++
				pterm.Info.Printf("  %-24s no compose files found — skipped\n", r.Stack)
			default:
				totalImported++
				pterm.Success.Printf("  %-24s %v\n", r.Stack, r.Files)
			}
		}
	}

	pterm.Println()
	if totalImported > 0 {
		pterm.Info.Printf("Imported %d stack(s) into %s\n", totalImported, stacksDir)
		pterm.Info.Println("Review the files, then commit them to your git repo.")
		pterm.Info.Println("To store .env secrets:  dockflux secrets set <stack> KEY value")
	}
	if totalSkipped > 0 {
		pterm.Warning.Printf("%d stack(s) skipped (already exist locally).\n", totalSkipped)
	}
	if totalImported == 0 && totalSkipped == 0 && totalEmpty == 0 {
		pterm.Info.Println("No stacks found on the target host(s).")
	}

	return nil
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/helloWorld44-89/dockflux/internal/importer"
	"github.com/helloWorld44-89/dockflux/internal/inventory"
	"github.com/helloWorld44-89/dockflux/internal/secrets"
	"github.com/helloWorld44-89/dockflux/internal/ui"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import existing Docker Compose stacks from remote hosts into the local repo",
	Long: `Connects to each target host via SSH/SFTP, discovers stack directories under
compose_dir, and downloads compose files into the local stacks directory.

If a .env file is found it is imported as .env.example (values replaced with
"changeme") and the real values are saved to the encrypted secrets store.`,
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

	// stackSecrets accumulates env values across all hosts to save in one pass.
	stackSecrets := make(map[string]map[string]string)

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
				if len(r.Secrets) > 0 {
					stackSecrets[r.Stack] = r.Secrets
				}
			}
		}
	}

	// Save imported .env values into the secrets store.
	if len(stackSecrets) > 0 {
		pterm.Println()
		pterm.Info.Println("Found .env files — saving values to the secrets store.")
		password, err := secrets.PromptPassword("Master password")
		if err != nil {
			return fmt.Errorf("reading password: %w", err)
		}

		store, err := secrets.Load(cfg.SecretsFile, password)
		if err != nil {
			return fmt.Errorf("loading secrets: %w", err)
		}

		totalKeys := 0
		for stack, kvs := range stackSecrets {
			for k, v := range kvs {
				store.SetStackSecret(stack, k, v)
				totalKeys++
			}
		}

		if err := secrets.Save(cfg.SecretsFile, password, store); err != nil {
			return fmt.Errorf("saving secrets: %w", err)
		}
		pterm.Success.Printf("Saved %d secret(s) across %d stack(s).\n", totalKeys, len(stackSecrets))
	}

	pterm.Println()
	if totalImported > 0 {
		pterm.Info.Printf("Imported %d stack(s) into %s\n", totalImported, stacksDir)
		pterm.Info.Println("Review the files, then commit them to your git repo.")
	}
	if totalSkipped > 0 {
		pterm.Warning.Printf("%d stack(s) skipped (already exist locally).\n", totalSkipped)
	}
	if totalImported == 0 && totalSkipped == 0 && totalEmpty == 0 {
		pterm.Info.Println("No stacks found on the target host(s).")
	}

	return nil
}

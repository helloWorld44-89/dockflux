package secretscmd

import (
	"github.com/helloWorld44-89/dockflux/internal/secrets"
	"github.com/helloWorld44-89/dockflux/internal/ui"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list [stack]",
	Short: "List secret keys (values are never shown)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runList,
}

func runList(cmd *cobra.Command, args []string) error {
	secretsPath := secretsFilePath()

	password, err := secrets.PromptPassword("Master password")
	if err != nil {
		return err
	}

	store, err := secrets.Load(secretsPath, password)
	if err != nil {
		return err
	}

	if len(args) == 1 {
		// Single stack
		stackName := args[0]
		stackSecrets := store.GetStackSecrets(stackName)
		if len(stackSecrets) == 0 {
			ui.Info("No secrets for stack %q", stackName)
			return nil
		}
		rows := [][]string{{"KEY", "VALUE"}}
		for k := range stackSecrets {
			rows = append(rows, []string{k, "••••••••"})
		}
		_ = pterm.DefaultTable.WithHasHeader().WithData(rows).Render()
		return nil
	}

	// All stacks
	if len(store.Stacks) == 0 && store.Git.HTTPSToken == "" && store.Git.KeyPassphrase == "" {
		ui.Info("No secrets stored yet.")
		return nil
	}

	rows := [][]string{{"STACK", "KEY", "VALUE"}}

	if store.Git.HTTPSToken != "" {
		rows = append(rows, []string{"git", "https_token", "••••••••"})
	}
	if store.Git.KeyPassphrase != "" {
		rows = append(rows, []string{"git", "key_passphrase", "••••••••"})
	}

	for stackName, kvs := range store.Stacks {
		for k := range kvs {
			rows = append(rows, []string{stackName, k, "••••••••"})
		}
	}

	_ = pterm.DefaultTable.WithHasHeader().WithData(rows).Render()
	return nil
}

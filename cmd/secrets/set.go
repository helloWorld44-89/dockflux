package secretscmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/darkmode_dev/dockflux/internal/secrets"
	"github.com/darkmode_dev/dockflux/internal/ui"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set <stack> <key>",
	Short: "Set a secret value for a stack",
	Args:  cobra.ExactArgs(2),
	RunE:  runSet,
}

func runSet(cmd *cobra.Command, args []string) error {
	stack, key := args[0], args[1]

	// Prompt for the value (masked)
	var value string
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("Value for %s/%s", stack, key)).
				EchoMode(huh.EchoModePassword).
				Value(&value).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("value cannot be empty")
					}
					return nil
				}),
		),
	).Run(); err != nil {
		return err
	}

	secretsPath := secretsFilePath()

	password, err := secrets.PromptPassword("Master password")
	if err != nil {
		return err
	}

	store, err := secrets.Load(secretsPath, password)
	if err != nil {
		return err
	}

	store.SetStackSecret(stack, key, value)

	if err := secrets.Save(secretsPath, password, store); err != nil {
		return err
	}

	ui.Success("Set %s/%s", stack, key)
	return nil
}

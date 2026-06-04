package secretscmd

import (
	"github.com/helloWorld44-89/dockflux/internal/secrets"
	"github.com/helloWorld44-89/dockflux/internal/ui"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <stack> <key>",
	Short: "Delete a secret from a stack",
	Args:  cobra.ExactArgs(2),
	RunE:  runDelete,
}

func runDelete(cmd *cobra.Command, args []string) error {
	stack, key := args[0], args[1]

	secretsPath := secretsFilePath()

	password, err := secrets.PromptPassword("Master password")
	if err != nil {
		return err
	}

	store, err := secrets.Load(secretsPath, password)
	if err != nil {
		return err
	}

	store.DeleteStackSecret(stack, key)

	if err := secrets.Save(secretsPath, password, store); err != nil {
		return err
	}

	ui.Success("Deleted %s/%s", stack, key)
	return nil
}

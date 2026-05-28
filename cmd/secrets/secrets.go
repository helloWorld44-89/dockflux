package secretscmd

import "github.com/spf13/cobra"

var SecretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage encrypted secrets for stacks",
	Long: `secrets manages the encrypted store (~/.dockflux/secrets.enc) that holds:
  - Git credentials (HTTPS token, SSH key passphrase)
  - Per-stack .env values injected at deploy time

The store is protected by a master password. Set DOCKFLUX_SECRETS_PASSWORD
in the environment to avoid interactive prompts (required for the watch daemon).`,
}

func init() {
	SecretsCmd.AddCommand(setCmd)
	SecretsCmd.AddCommand(listCmd)
	SecretsCmd.AddCommand(deleteCmd)
	SecretsCmd.AddCommand(envCmd)
}

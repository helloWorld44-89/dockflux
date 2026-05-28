package secretscmd

import (
	"fmt"
	"path/filepath"

	"github.com/jconder44/dockflux/internal/config"
	"github.com/jconder44/dockflux/internal/secrets"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var envCmd = &cobra.Command{
	Use:   "env <stack>",
	Short: "Preview the generated .env for a stack (shows values, use with care)",
	Args:  cobra.ExactArgs(1),
	RunE:  runEnv,
}

func runEnv(cmd *cobra.Command, args []string) error {
	stackName := args[0]

	cfgPath := viper.ConfigFileUsed()
	if cfgPath == "" {
		cfgPath = "dockflux.yml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	secretsPath := cfg.SecretsFile

	password, err := secrets.PromptPassword("Master password")
	if err != nil {
		return err
	}

	store, err := secrets.Load(secretsPath, password)
	if err != nil {
		return err
	}

	stackPath := filepath.Join(cfg.Repo.LocalPath, cfg.StacksDir, stackName)
	content, err := secrets.GenerateEnv(stackPath, store.GetStackSecrets(stackName))
	if err != nil {
		return err
	}
	if content == nil {
		fmt.Printf("No .env.example found at %s/.env.example\n", stackPath)
		return nil
	}

	fmt.Print(string(content))
	return nil
}

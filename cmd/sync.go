package cmd

import (
	"fmt"

	"github.com/helloWorld44-89/dockflux/internal/config"
	"github.com/helloWorld44-89/dockflux/internal/gitops"
	"github.com/helloWorld44-89/dockflux/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Clone or pull the stacks git repository",
	RunE:  runSync,
}

func runSync(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	stop := ui.Spinner(fmt.Sprintf("Syncing %s (%s)", cfg.Repo.URL, cfg.Repo.Branch))

	auth, err := gitops.SSHAuth(cfg.Repo.Key)
	if err != nil {
		stop(false, "Invalid SSH key")
		return err
	}
	err = gitops.CloneOrPull(cfg.Repo.URL, cfg.Repo.Branch, cfg.Repo.LocalPath, auth)
	if err != nil {
		stop(false, "Sync failed")
		return err
	}

	commit, err := gitops.HeadCommit(cfg.Repo.LocalPath)
	if err != nil {
		stop(false, "Could not read HEAD commit")
		return err
	}

	stop(true, fmt.Sprintf("Synced to %s", commit))
	return nil
}

func loadConfig() (*config.Config, error) {
	cfgPath := viper.ConfigFileUsed()
	if cfgPath == "" {
		cfgPath = "dockflux.yml"
	}
	return config.Load(cfgPath)
}

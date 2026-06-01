package cmd

import (
	"github.com/helloWorld44-89/dockflux/internal/lockfile"
	"github.com/helloWorld44-89/dockflux/internal/ui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show what is deployed where",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	lf, err := lockfile.Load(cfg.StateFile)
	if err != nil {
		return err
	}

	ui.StatusTable(lf)
	return nil
}

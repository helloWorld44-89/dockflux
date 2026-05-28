package service

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Stop, disable, and remove the dockflux-watch systemd service",
	RunE:  runUninstall,
}

func runUninstall(cmd *cobra.Command, args []string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("service uninstall requires root (run with sudo)")
	}

	// Best-effort stop and disable — ignore errors if service doesn't exist
	_ = systemctl("stop", "dockflux-watch.service")
	_ = systemctl("disable", "dockflux-watch.service")

	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing unit file: %w", err)
	}

	if err := systemctl("daemon-reload"); err != nil {
		return err
	}

	fmt.Println("dockflux-watch service removed.")
	return nil
}

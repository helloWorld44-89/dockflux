package service

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of the dockflux-watch systemd service",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	c := exec.Command("systemctl", "status", "dockflux-watch.service")
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	// systemctl status exits 3 when inactive — don't treat that as an error
	_ = c.Run()
	return nil
}

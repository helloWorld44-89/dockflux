package service

import "github.com/spf13/cobra"

var ServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage the dockflux systemd service",
}

func init() {
	ServiceCmd.AddCommand(installCmd)
	ServiceCmd.AddCommand(uninstallCmd)
	ServiceCmd.AddCommand(statusCmd)
}

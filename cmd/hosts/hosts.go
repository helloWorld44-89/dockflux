package hosts

import "github.com/spf13/cobra"

var HostsCmd = &cobra.Command{
	Use:   "hosts",
	Short: "Manage and inspect configured hosts",
}

func init() {
	HostsCmd.AddCommand(listCmd)
	HostsCmd.AddCommand(pingCmd)
}

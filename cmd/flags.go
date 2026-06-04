package cmd

import (
	"github.com/helloWorld44-89/dockflux/internal/inventory"
	"github.com/spf13/cobra"
)

// addTargetFlags attaches the standard host-targeting flags to a command.
func addTargetFlags(cmd *cobra.Command) {
	cmd.Flags().String("host", "", "Target a single host by name")
	cmd.Flags().String("group", "", "Target all hosts in a group")
	cmd.Flags().Bool("all", false, "Target all hosts in the inventory")
	cmd.Flags().Bool("local", false, "Target the local host only")
	cmd.Flags().Bool("dry-run", false, "Print actions without executing them")
}

// resolveTargets reads the targeting flags and returns the matching hosts.
func resolveTargets(cmd *cobra.Command, inv *inventory.Inventory) ([]*inventory.Host, error) {
	hostFlag, _ := cmd.Flags().GetString("host")
	groupFlag, _ := cmd.Flags().GetString("group")
	allFlag, _ := cmd.Flags().GetBool("all")
	localFlag, _ := cmd.Flags().GetBool("local")
	return inventory.ResolveHosts(inv, hostFlag, groupFlag, allFlag, localFlag)
}

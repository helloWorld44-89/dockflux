package cmd

import (
	"fmt"

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

// resolveTargets reads the targeting flags and returns the matching hosts
// for stack. Explicit --host/--group/--local flags are taken as-is. With
// --all, or with no targeting flag at all, hosts are filtered down to those
// whose inventory `stacks:` list includes stack. Pass an empty stack for
// commands that aren't scoped to a single stack (e.g. import) — that skips
// the implicit-default and stack filtering, keeping the original behavior
// where a target flag is required.
func resolveTargets(cmd *cobra.Command, inv *inventory.Inventory, stack string) ([]*inventory.Host, error) {
	hostFlag, _ := cmd.Flags().GetString("host")
	groupFlag, _ := cmd.Flags().GetString("group")
	allFlag, _ := cmd.Flags().GetBool("all")
	localFlag, _ := cmd.Flags().GetBool("local")

	noFlagSet := hostFlag == "" && groupFlag == "" && !allFlag && !localFlag
	if stack != "" && (allFlag || noFlagSet) {
		hosts, err := inventory.ResolveHosts(inv, "", "", true, false)
		if err != nil {
			return nil, err
		}
		hosts = inventory.FilterForStack(hosts, stack)
		if len(hosts) == 0 {
			return nil, fmt.Errorf("no hosts assigned to stack %q in inventory — add it to a host's stacks: list, or target a host explicitly with --host", stack)
		}
		return hosts, nil
	}

	return inventory.ResolveHosts(inv, hostFlag, groupFlag, allFlag, localFlag)
}

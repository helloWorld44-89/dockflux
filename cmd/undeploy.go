package cmd

import (
	"github.com/helloWorld44-89/dockflux/internal/runner"
	"github.com/spf13/cobra"
)

var undeployCmd = &cobra.Command{
	Use:   "undeploy <stack>",
	Short: "Tear down a stack on one or more hosts",
	Args:  cobra.ExactArgs(1),
	RunE:  runUndeploy,
}

func init() {
	addTargetFlags(undeployCmd)
}

func runUndeploy(cmd *cobra.Command, args []string) error {
	return runComposeAction(cmd, args, runner.ActionDown)
}

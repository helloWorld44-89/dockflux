package cmd

import (
	"github.com/helloWorld44-89/dockflux/internal/runner"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart <stack>",
	Short: "Restart a stack on one or more hosts",
	Args:  cobra.ExactArgs(1),
	RunE:  runRestart,
}

func init() {
	addTargetFlags(restartCmd)
}

func runRestart(cmd *cobra.Command, args []string) error {
	return runComposeAction(cmd, args, runner.ActionRestart)
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jconder44/dockflux/internal/inventory"
	"github.com/jconder44/dockflux/internal/runner"
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec <stack> <service> -- <command>",
	Short: "Run a command inside a running service container",
	Args:  cobra.MinimumNArgs(2),
	RunE:  runExec,
}

func init() {
	execCmd.Flags().String("host", "local", "Target host name")
}

func runExec(cmd *cobra.Command, args []string) error {
	stackName := args[0]
	serviceName := args[1]
	execArgs := args[2:]

	if len(execArgs) == 0 {
		execArgs = []string{"sh"}
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	inv, err := inventory.Load(cfg.Inventory)
	if err != nil {
		return err
	}

	hostFlag, _ := cmd.Flags().GetString("host")
	h, ok := inv.Hosts[hostFlag]
	if !ok {
		return fmt.Errorf("host %q not found in inventory", hostFlag)
	}

	r, err := runner.New(h)
	if err != nil {
		return err
	}

	composeDir := h.ComposeDir
	if h.Type == inventory.HostTypeLocal {
		// Fix #3: use the stack-specific path, not the stacks root
		composeDir = filepath.Join(cfg.Repo.LocalPath, cfg.StacksDir, stackName)
	}

	return r.Exec(cmd.Context(), runner.ExecOptions{
		Stack:      stackName,
		ComposeDir: composeDir,
		Service:    serviceName,
		Cmd:        execArgs,
		Stdin:      os.Stdin,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	})
}

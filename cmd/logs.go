package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jconder44/dockflux/internal/inventory"
	"github.com/jconder44/dockflux/internal/runner"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs <stack>",
	Short: "Stream docker compose logs from a host",
	Args:  cobra.ExactArgs(1),
	RunE:  runLogs,
}

func init() {
	logsCmd.Flags().String("host", "local", "Target host name")
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
}

func runLogs(cmd *cobra.Command, args []string) error {
	stackName := args[0]

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

	follow, _ := cmd.Flags().GetBool("follow")

	r, err := runner.New(h)
	if err != nil {
		return err
	}

	composeDir := h.ComposeDir
	if h.Type == inventory.HostTypeLocal {
		// Fix #3: use the stack-specific path, not the stacks root
		composeDir = filepath.Join(cfg.Repo.LocalPath, cfg.StacksDir, stackName)
	}

	return r.Logs(cmd.Context(), runner.LogOptions{
		Stack:      stackName,
		ComposeDir: composeDir,
		Follow:     follow,
		Writer:     os.Stdout,
	})
}

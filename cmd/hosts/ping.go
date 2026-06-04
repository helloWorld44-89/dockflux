package hosts

import (
	"fmt"

	"github.com/darkmode_dev/dockflux/internal/runner"
	"github.com/darkmode_dev/dockflux/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var pingCmd = &cobra.Command{
	Use:   "ping",
	Short: "Test connectivity to all configured hosts",
	RunE:  runPing,
}

func runPing(cmd *cobra.Command, args []string) error {
	inv, err := loadInventory()
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(cmd.Context())

	for _, h := range inv.Hosts {
		h := h
		g.Go(func() error {
			r, err := runner.New(h)
			if err != nil {
				ui.Error("%-12s  FAILED  (%v)", h.Name, err)
				return nil // don't fail the group; report all results
			}
			if err := r.Ping(ctx); err != nil {
				ui.Error("%-12s  UNREACHABLE  (%v)", h.Name, err)
				return nil
			}
			ui.Success("%-12s  OK", h.Name)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	return nil
}

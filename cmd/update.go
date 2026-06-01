package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/jconder44/dockflux/internal/ui"
	"github.com/jconder44/dockflux/internal/updater"
	"github.com/spf13/cobra"
)

var updateYes bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update dockflux to the latest release",
	RunE:  runUpdate,
}

func init() {
	updateCmd.Flags().BoolVarP(&updateYes, "yes", "y", false, "skip confirmation prompt")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	stop := ui.Spinner("Checking for latest release...")

	latest, err := updater.FetchLatest()
	if err != nil {
		stop(false, "Could not reach GitHub")
		return err
	}
	stop(true, fmt.Sprintf("Latest release: %s", latest))

	if !updater.IsNewer(Version, latest) {
		ui.Success("Already up to date (%s)", Version)
		return nil
	}

	ui.Info("Installed: %s  →  Available: %s", Version, latest)

	if !updateYes {
		fmt.Print("Update now? [y/N] ")
		r := bufio.NewReader(os.Stdin)
		answer, _ := r.ReadString('\n')
		if !strings.EqualFold(strings.TrimSpace(answer), "y") {
			ui.Info("Update cancelled.")
			return nil
		}
	}

	stop = ui.Spinner(fmt.Sprintf("Downloading %s...", latest))
	if err := updater.Apply(latest); err != nil {
		stop(false, "Update failed")
		return err
	}
	stop(true, fmt.Sprintf("Updated to %s", latest))
	return nil
}

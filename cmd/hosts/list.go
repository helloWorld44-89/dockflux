package hosts

import (
	"strings"

	"github.com/jconder44/dockflux/internal/inventory"
	"github.com/jconder44/dockflux/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured hosts",
	RunE:  runList,
}

func runList(cmd *cobra.Command, args []string) error {
	inv, err := loadInventory()
	if err != nil {
		return err
	}

	rows := make([][]string, 0, len(inv.Hosts))
	for _, h := range inv.Hosts {
		rows = append(rows, []string{
			h.Name,
			string(h.Type),
			h.Host,
			h.User,
			strings.Join(h.Groups, ", "),
		})
	}

	ui.HostsTable(rows)
	return nil
}

func loadInventory() (*inventory.Inventory, error) {
	invPath := viper.GetString("inventory")
	if invPath == "" {
		invPath = "inventory.yml"
	}
	return inventory.Load(invPath)
}

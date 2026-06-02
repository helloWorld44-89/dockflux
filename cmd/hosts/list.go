package hosts

import (
	"strings"

	"github.com/helloWorld44-89/dockflux/internal/config"
	"github.com/helloWorld44-89/dockflux/internal/inventory"
	"github.com/helloWorld44-89/dockflux/internal/ui"
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
	cfgPath := viper.ConfigFileUsed()
	if cfgPath == "" {
		cfgPath = config.FindConfigFile()
	}
	if cfgPath == "" {
		cfgPath = ".dockflux/dockflux.yml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}
	return inventory.Load(cfg.Inventory)
}

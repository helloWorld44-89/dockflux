package cmd

import (
	"fmt"
	"os"

	"github.com/helloWorld44-89/dockflux/cmd/hosts"
	secretscmd "github.com/helloWorld44-89/dockflux/cmd/secrets"
	"github.com/helloWorld44-89/dockflux/cmd/service"
	"github.com/helloWorld44-89/dockflux/internal/ui"
	"github.com/helloWorld44-89/dockflux/internal/updater"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Version is set at build time via -ldflags "-X github.com/helloWorld44-89/dockflux/cmd.Version=..."
var Version = "dev"

var cfgFile string
var inventoryFile string

var rootCmd = &cobra.Command{
	Use:   "dockflux",
	Short: "Deploy Docker Compose stacks from a git repo to a fleet of hosts",
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if cmd.Use == "update" {
			return
		}
		if newer := updater.CheckForUpdate(Version); newer != "" {
			ui.Warn("dockflux %s is available (you have %s) — run 'dockflux update' to upgrade", newer, Version)
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig, updater.RefreshCacheAsync)

	rootCmd.Version = Version
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./dockflux.yml)")
	rootCmd.PersistentFlags().StringVar(&inventoryFile, "inventory", "", "inventory file (default: from config)")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(undeployCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(hosts.HostsCmd)
	rootCmd.AddCommand(service.ServiceCmd)
	rootCmd.AddCommand(secretscmd.SecretsCmd)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName("dockflux")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintln(os.Stderr, "Error reading config:", err)
		}
	}
}

package cmd

import (
	"fmt"
	"os"

	"github.com/jconder44/dockflux/cmd/hosts"
	secretscmd "github.com/jconder44/dockflux/cmd/secrets"
	"github.com/jconder44/dockflux/cmd/service"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var inventoryFile string

var rootCmd = &cobra.Command{
	Use:   "dockflux",
	Short: "Deploy Docker Compose stacks from a git repo to a fleet of hosts",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

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

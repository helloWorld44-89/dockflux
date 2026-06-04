package secretscmd

import (
	"github.com/darkmode_dev/dockflux/internal/config"
	"github.com/darkmode_dev/dockflux/internal/secrets"
	"github.com/spf13/viper"
)

// secretsFilePath returns the expanded secrets file path from config,
// falling back to the default if config cannot be loaded.
func secretsFilePath() string {
	cfgPath := viper.ConfigFileUsed()
	if cfgPath == "" {
		cfgPath = config.FindConfigFile()
	}
	if cfgPath == "" {
		cfgPath = ".dockflux/dockflux.yml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return secrets.DefaultPath()
	}
	return cfg.SecretsFile
}

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type RepoConfig struct {
	URL       string `yaml:"url"`
	Branch    string `yaml:"branch"`
	LocalPath string `yaml:"local_path"`
	Key       string `yaml:"key"` // SSH key for git auth (optional, falls back to ssh-agent)
}

type Config struct {
	Repo        RepoConfig `yaml:"repo"`
	StateFile   string     `yaml:"state_file"`
	Inventory   string     `yaml:"inventory"`
	StacksDir   string     `yaml:"stacks_dir"`
	SecretsFile string     `yaml:"secrets_file"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	cfg.Repo.LocalPath = expandPath(cfg.Repo.LocalPath)
	cfg.Repo.Key = expandPath(cfg.Repo.Key)
	cfg.StateFile = expandPath(cfg.StateFile)
	cfg.Inventory = expandPath(cfg.Inventory)
	cfg.SecretsFile = expandPath(cfg.SecretsFile)

	if cfg.Repo.Branch == "" {
		cfg.Repo.Branch = "main"
	}
	if cfg.StacksDir == "" {
		cfg.StacksDir = "stacks/"
	}
	if cfg.StateFile == "" {
		cfg.StateFile = "dockflux.lock"
	}
	if cfg.Inventory == "" {
		cfg.Inventory = "inventory.yml"
	}
	if cfg.SecretsFile == "" {
		home, _ := os.UserHomeDir()
		cfg.SecretsFile = filepath.Join(home, ".dockflux", "secrets.enc")
	}

	// Make relative paths absolute relative to the config file's dir
	base := filepath.Dir(path)
	if !filepath.IsAbs(cfg.StateFile) {
		cfg.StateFile = filepath.Join(base, cfg.StateFile)
	}
	if !filepath.IsAbs(cfg.Inventory) {
		cfg.Inventory = filepath.Join(base, cfg.Inventory)
	}

	return &cfg, nil
}

func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			p = filepath.Join(home, p[2:])
		}
	}
	return p
}

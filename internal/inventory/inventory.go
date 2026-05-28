package inventory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type HostType string

const (
	HostTypeLocal  HostType = "local"
	HostTypeRemote HostType = "remote"
)

type Host struct {
	Name       string   `yaml:"-"`
	Type       HostType `yaml:"type"`
	Host       string   `yaml:"host"`
	Port       int      `yaml:"port"`
	User       string   `yaml:"user"`
	Key        string   `yaml:"key"`
	Groups     []string `yaml:"groups"`
	ComposeDir string   `yaml:"compose_dir"`
}

type Inventory struct {
	Hosts map[string]*Host `yaml:"hosts"`
}

func Load(path string) (*Inventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading inventory %s: %w", path, err)
	}

	var inv Inventory
	if err := yaml.Unmarshal(data, &inv); err != nil {
		return nil, fmt.Errorf("parsing inventory: %w", err)
	}

	for name, h := range inv.Hosts {
		h.Name = name
		if h.Type == "" {
			h.Type = HostTypeRemote
		}
		if h.Port == 0 {
			h.Port = 22
		}
		if h.Key != "" {
			h.Key = expandPath(h.Key)
		}
		if h.ComposeDir == "" && h.Type == HostTypeRemote {
			h.ComposeDir = "/opt/stacks"
		}
	}

	return &inv, nil
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

// ResolveHosts returns the set of hosts to target based on flags.
func ResolveHosts(inv *Inventory, hostFlag, groupFlag string, allFlag, localFlag bool) ([]*Host, error) {
	if allFlag {
		hosts := make([]*Host, 0, len(inv.Hosts))
		for _, h := range inv.Hosts {
			hosts = append(hosts, h)
		}
		return hosts, nil
	}

	if localFlag {
		h, ok := inv.Hosts["local"]
		if !ok {
			return nil, fmt.Errorf("no 'local' host defined in inventory")
		}
		return []*Host{h}, nil
	}

	if hostFlag != "" {
		h, ok := inv.Hosts[hostFlag]
		if !ok {
			return nil, fmt.Errorf("host %q not found in inventory", hostFlag)
		}
		return []*Host{h}, nil
	}

	if groupFlag != "" {
		var matched []*Host
		for _, h := range inv.Hosts {
			for _, g := range h.Groups {
				if g == groupFlag {
					matched = append(matched, h)
					break
				}
			}
		}
		if len(matched) == 0 {
			return nil, fmt.Errorf("no hosts found in group %q", groupFlag)
		}
		return matched, nil
	}

	return nil, fmt.Errorf("no target specified: use --host, --group, --all, or --local")
}

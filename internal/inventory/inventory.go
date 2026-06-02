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
	Type       HostType `yaml:"type,omitempty"`
	Host       string   `yaml:"host,omitempty"`
	Port       int      `yaml:"port,omitempty"`
	User       string   `yaml:"user,omitempty"`
	Key        string   `yaml:"key,omitempty"`
	Groups     []string `yaml:"groups,omitempty"`
	ComposeDir string   `yaml:"compose_dir,omitempty"`
	Stacks     []string `yaml:"stacks,omitempty"`
}

// RunsStack returns true if this host should run the given stack.
// An empty Stacks list means the host runs all stacks (backward compat).
func (h *Host) RunsStack(stack string) bool {
	if len(h.Stacks) == 0 {
		return true
	}
	for _, s := range h.Stacks {
		if s == stack {
			return true
		}
	}
	return false
}

// FilterForStack returns only the hosts whose Stacks list includes stack
// (or whose list is empty, meaning they run everything).
func FilterForStack(hosts []*Host, stack string) []*Host {
	var out []*Host
	for _, h := range hosts {
		if h.RunsStack(stack) {
			out = append(out, h)
		}
	}
	return out
}

type Inventory struct {
	Hosts map[string]*Host `yaml:"hosts"`
}

func Load(path string) (*Inventory, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Inventory{Hosts: make(map[string]*Host)}, nil
	}
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

// Save writes the inventory back to path, unexpanding home-dir paths so the
// file stays portable across machines.
func Save(path string, inv *Inventory) error {
	// Shallow-copy each host with key path unexpanded before marshaling.
	saveable := &Inventory{Hosts: make(map[string]*Host, len(inv.Hosts))}
	for name, h := range inv.Hosts {
		cp := *h
		cp.Key = unexpandPath(cp.Key)
		saveable.Hosts[name] = &cp
	}
	data, err := yaml.Marshal(saveable)
	if err != nil {
		return fmt.Errorf("marshalling inventory: %w", err)
	}
	return os.WriteFile(path, data, 0644)
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

func unexpandPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if strings.HasPrefix(p, home+"/") {
		return "~/" + p[len(home)+1:]
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

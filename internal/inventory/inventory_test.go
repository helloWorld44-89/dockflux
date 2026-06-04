package inventory_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/helloWorld44-89/dockflux/internal/inventory"
)

func writeInventory(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "inventory.yml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing inventory file: %v", err)
	}
	return path
}

func TestLoad_Basic(t *testing.T) {
	path := writeInventory(t, `
hosts:
  local:
    type: local
  web01:
    host: 10.0.0.1
    user: deploy
    key: /tmp/test_key
    groups:
      - production
    compose_dir: /opt/stacks
`)
	inv, err := inventory.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(inv.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(inv.Hosts))
	}

	web01 := inv.Hosts["web01"]
	if web01 == nil {
		t.Fatal("expected web01 in inventory")
	}
	if web01.Name != "web01" {
		t.Errorf("Name: got %q, want %q", web01.Name, "web01")
	}
	if web01.Port != 22 {
		t.Errorf("default Port: got %d, want 22", web01.Port)
	}
	if web01.Type != inventory.HostTypeRemote {
		t.Errorf("default Type: got %q, want remote", web01.Type)
	}
}

func TestLoad_DefaultComposeDir(t *testing.T) {
	path := writeInventory(t, `
hosts:
  web01:
    host: 10.0.0.1
    user: deploy
`)
	inv, err := inventory.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if inv.Hosts["web01"].ComposeDir != "/opt/stacks" {
		t.Errorf("ComposeDir: got %q, want /opt/stacks", inv.Hosts["web01"].ComposeDir)
	}
}

func TestLoad_NotExist_ReturnsEmpty(t *testing.T) {
	inv, err := inventory.Load("/nonexistent/path/inventory.yml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if len(inv.Hosts) != 0 {
		t.Errorf("expected empty hosts, got %d entries", len(inv.Hosts))
	}
}

func TestResolveHosts_All(t *testing.T) {
	inv := &inventory.Inventory{
		Hosts: map[string]*inventory.Host{
			"web01": {Name: "web01"},
			"web02": {Name: "web02"},
		},
	}
	hosts, err := inventory.ResolveHosts(inv, "", "", true, false)
	if err != nil {
		t.Fatalf("ResolveHosts: %v", err)
	}
	if len(hosts) != 2 {
		t.Errorf("expected 2 hosts, got %d", len(hosts))
	}
}

func TestResolveHosts_ByName(t *testing.T) {
	inv := &inventory.Inventory{
		Hosts: map[string]*inventory.Host{
			"web01": {Name: "web01"},
			"web02": {Name: "web02"},
		},
	}
	hosts, err := inventory.ResolveHosts(inv, "web01", "", false, false)
	if err != nil {
		t.Fatalf("ResolveHosts: %v", err)
	}
	if len(hosts) != 1 || hosts[0].Name != "web01" {
		t.Errorf("expected [web01], got %v", hosts)
	}
}

func TestResolveHosts_ByGroup(t *testing.T) {
	inv := &inventory.Inventory{
		Hosts: map[string]*inventory.Host{
			"web01": {Name: "web01", Groups: []string{"production"}},
			"web02": {Name: "web02", Groups: []string{"staging"}},
			"db01":  {Name: "db01", Groups: []string{"production"}},
		},
	}
	hosts, err := inventory.ResolveHosts(inv, "", "production", false, false)
	if err != nil {
		t.Fatalf("ResolveHosts: %v", err)
	}
	if len(hosts) != 2 {
		t.Errorf("expected 2 production hosts, got %d", len(hosts))
	}
}

func TestResolveHosts_ByGroup_NotFound(t *testing.T) {
	inv := &inventory.Inventory{
		Hosts: map[string]*inventory.Host{
			"web01": {Name: "web01", Groups: []string{"production"}},
		},
	}
	_, err := inventory.ResolveHosts(inv, "", "staging", false, false)
	if err == nil {
		t.Error("expected error for missing group")
	}
}

func TestResolveHosts_ByName_NotFound(t *testing.T) {
	inv := &inventory.Inventory{
		Hosts: map[string]*inventory.Host{
			"web01": {Name: "web01"},
		},
	}
	_, err := inventory.ResolveHosts(inv, "missing", "", false, false)
	if err == nil {
		t.Error("expected error for missing host name")
	}
}

func TestResolveHosts_NoTarget_ReturnsError(t *testing.T) {
	inv := &inventory.Inventory{Hosts: map[string]*inventory.Host{}}
	_, err := inventory.ResolveHosts(inv, "", "", false, false)
	if err == nil {
		t.Error("expected error when no targeting flag set")
	}
}

func TestRunsStack_Explicit(t *testing.T) {
	h := &inventory.Host{Stacks: []string{"nginx", "certbot"}}
	if !h.RunsStack("nginx") {
		t.Error("expected nginx to match")
	}
	if !h.RunsStack("certbot") {
		t.Error("expected certbot to match")
	}
	if h.RunsStack("postgres") {
		t.Error("expected postgres not to match when not in list")
	}
}

func TestRunsStack_EmptyMeansAll(t *testing.T) {
	h := &inventory.Host{Stacks: nil}
	if !h.RunsStack("anything") {
		t.Error("empty Stacks list should match any stack name")
	}
}

func TestFilterForStack(t *testing.T) {
	hosts := []*inventory.Host{
		{Name: "web01", Stacks: []string{"nginx"}},
		{Name: "db01", Stacks: []string{"postgres"}},
		{Name: "all01", Stacks: nil},
	}
	got := inventory.FilterForStack(hosts, "nginx")
	// web01 (explicit match) + all01 (empty = all) should be included
	if len(got) != 2 {
		t.Errorf("expected 2 hosts for nginx, got %d", len(got))
	}
}

func TestFilterForStack_NoMatch(t *testing.T) {
	hosts := []*inventory.Host{
		{Name: "db01", Stacks: []string{"postgres"}},
	}
	got := inventory.FilterForStack(hosts, "nginx")
	if len(got) != 0 {
		t.Errorf("expected no hosts, got %d", len(got))
	}
}

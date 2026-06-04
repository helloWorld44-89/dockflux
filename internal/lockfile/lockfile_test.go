package lockfile_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/helloWorld44-89/dockflux/internal/lockfile"
)

func TestSetGetEntry(t *testing.T) {
	lf := lockfile.New()
	entry := &lockfile.LockEntry{
		Commit:     "abc1234",
		DeployedAt: time.Now().UTC().Truncate(time.Second),
		Status:     "running",
	}
	lf.SetEntry("nginx", "web01", entry)

	got := lf.GetEntry("nginx", "web01")
	if got == nil {
		t.Fatal("expected entry, got nil")
	}
	if got.Commit != "abc1234" {
		t.Errorf("commit: got %q, want %q", got.Commit, "abc1234")
	}
	if got.Status != "running" {
		t.Errorf("status: got %q, want %q", got.Status, "running")
	}
}

func TestGetEntry_Missing(t *testing.T) {
	lf := lockfile.New()
	if got := lf.GetEntry("nginx", "web01"); got != nil {
		t.Errorf("expected nil for missing entry, got %v", got)
	}
}

func TestGetEntry_MissingStack(t *testing.T) {
	lf := lockfile.New()
	lf.SetEntry("nginx", "web01", &lockfile.LockEntry{Commit: "abc"})
	if got := lf.GetEntry("postgres", "web01"); got != nil {
		t.Errorf("expected nil for missing stack, got %v", got)
	}
}

func TestRemoveEntry(t *testing.T) {
	lf := lockfile.New()
	entry := &lockfile.LockEntry{Commit: "abc1234", Status: "running"}
	lf.SetEntry("nginx", "web01", entry)
	lf.SetEntry("nginx", "web02", entry)

	lf.RemoveEntry("nginx", "web01")

	if lf.GetEntry("nginx", "web01") != nil {
		t.Error("expected web01 entry to be removed")
	}
	if lf.GetEntry("nginx", "web02") == nil {
		t.Error("expected web02 entry to remain")
	}
}

func TestRemoveEntry_LastHost_CleansStack(t *testing.T) {
	lf := lockfile.New()
	lf.SetEntry("nginx", "web01", &lockfile.LockEntry{Commit: "abc1234", Status: "running"})

	lf.RemoveEntry("nginx", "web01")

	if _, ok := lf.Stacks["nginx"]; ok {
		t.Error("expected stack map key to be removed after last host deleted")
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dockflux.lock")

	lf := lockfile.New()
	lf.SetEntry("nginx", "web01", &lockfile.LockEntry{
		Commit:     "abc1234",
		DeployedAt: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		Status:     "running",
	})
	lf.SetEntry("postgres", "db01", &lockfile.LockEntry{
		Commit:     "def5678",
		DeployedAt: time.Date(2025, 1, 2, 8, 0, 0, 0, time.UTC),
		Status:     "stopped",
	})

	if err := lockfile.Save(path, lf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := lockfile.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	e1 := loaded.GetEntry("nginx", "web01")
	if e1 == nil || e1.Commit != "abc1234" || e1.Status != "running" {
		t.Errorf("nginx/web01: got %+v", e1)
	}

	e2 := loaded.GetEntry("postgres", "db01")
	if e2 == nil || e2.Commit != "def5678" || e2.Status != "stopped" {
		t.Errorf("postgres/db01: got %+v", e2)
	}
}

func TestLoad_NotExist_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	lf, err := lockfile.Load(filepath.Join(dir, "nonexistent.lock"))
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if lf == nil || len(lf.Stacks) != 0 {
		t.Errorf("expected empty lockfile, got %+v", lf)
	}
}

func TestSave_Atomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dockflux.lock")

	lf := lockfile.New()
	lf.SetEntry("nginx", "web01", &lockfile.LockEntry{Commit: "aaa"})
	if err := lockfile.Save(path, lf); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	lf.SetEntry("nginx", "web01", &lockfile.LockEntry{Commit: "bbb"})
	if err := lockfile.Save(path, lf); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	loaded, err := lockfile.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if e := loaded.GetEntry("nginx", "web01"); e == nil || e.Commit != "bbb" {
		t.Errorf("expected bbb after overwrite, got %+v", e)
	}
}

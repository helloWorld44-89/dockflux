package lockfile

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type LockEntry struct {
	Commit     string    `yaml:"commit"`
	DeployedAt time.Time `yaml:"deployed_at"`
	Status     string    `yaml:"status"`
}

// StackState maps host name → LockEntry
type StackState map[string]*LockEntry

// LockFile maps stack name → StackState
type LockFile struct {
	Stacks map[string]StackState `yaml:"stacks"`
}

func New() *LockFile {
	return &LockFile{Stacks: make(map[string]StackState)}
}

func Load(path string) (*LockFile, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return New(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading lockfile: %w", err)
	}

	lf := New()
	if err := yaml.Unmarshal(data, lf); err != nil {
		return nil, fmt.Errorf("parsing lockfile: %w", err)
	}
	return lf, nil
}

// Save writes the lockfile atomically (write to tmp, rename into place).
func Save(path string, lf *LockFile) error {
	data, err := yaml.Marshal(lf)
	if err != nil {
		return fmt.Errorf("marshalling lockfile: %w", err)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".dockflux-lock-*")
	if err != nil {
		return fmt.Errorf("creating temp lockfile: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp lockfile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, path)
}

func (lf *LockFile) SetEntry(stack, host string, entry *LockEntry) {
	if lf.Stacks == nil {
		lf.Stacks = make(map[string]StackState)
	}
	if lf.Stacks[stack] == nil {
		lf.Stacks[stack] = make(StackState)
	}
	lf.Stacks[stack][host] = entry
}

func (lf *LockFile) GetEntry(stack, host string) *LockEntry {
	if lf.Stacks == nil {
		return nil
	}
	if lf.Stacks[stack] == nil {
		return nil
	}
	return lf.Stacks[stack][host]
}

func (lf *LockFile) RemoveEntry(stack, host string) {
	if lf.Stacks == nil {
		return
	}
	if lf.Stacks[stack] == nil {
		return
	}
	delete(lf.Stacks[stack], host)
	if len(lf.Stacks[stack]) == 0 {
		delete(lf.Stacks, stack)
	}
}

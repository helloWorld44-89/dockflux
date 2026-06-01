package runner

import (
	"context"
	"fmt"
	"io"

	"github.com/helloWorld44-89/dockflux/internal/inventory"
)

type ComposeAction string

const (
	ActionUp      ComposeAction = "up -d"
	ActionDown    ComposeAction = "down"
	ActionRestart ComposeAction = "restart"
)

type RunOptions struct {
	Stack      string
	StackPath  string        // absolute local path to the stack directory
	ComposeDir string        // remote base directory (ignored for local runner)
	Action     ComposeAction
	Pull       bool
	DryRun     bool
	Commit     string
}

type LogOptions struct {
	Stack      string
	ComposeDir string
	Follow     bool
	Writer     io.Writer
}

type ExecOptions struct {
	Stack      string
	ComposeDir string
	Service    string
	Cmd        []string
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
}

// Runner abstracts local vs remote compose operations.
type Runner interface {
	// CopyStack transfers the stack directory to the target.
	CopyStack(ctx context.Context, opts RunOptions) error

	// ComposeRun executes a docker compose action and returns combined output.
	ComposeRun(ctx context.Context, opts RunOptions) (string, error)

	// Logs streams docker compose logs to opts.Writer.
	Logs(ctx context.Context, opts LogOptions) error

	// Exec runs a command inside a running service container.
	Exec(ctx context.Context, opts ExecOptions) error

	// Ping verifies the runner target is reachable.
	Ping(ctx context.Context) error
}

// New returns the appropriate Runner for the given host.
func New(host *inventory.Host) (Runner, error) {
	if host.Type == inventory.HostTypeLocal {
		return &LocalRunner{}, nil
	}
	r, err := newRemoteRunner(host)
	if err != nil {
		return nil, fmt.Errorf("creating SSH runner for %s: %w", host.Name, err)
	}
	return r, nil
}

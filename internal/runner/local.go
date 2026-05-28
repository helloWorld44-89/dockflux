package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type LocalRunner struct{}

func (r *LocalRunner) CopyStack(_ context.Context, opts RunOptions) error {
	// For local runner the repo cache is used directly — no copy needed.
	return nil
}

func (r *LocalRunner) ComposeRun(ctx context.Context, opts RunOptions) (string, error) {
	args := []string{"compose"}
	if opts.Pull {
		if err := r.runCompose(ctx, opts.StackPath, append(args, "pull"), opts.DryRun, io.Discard); err != nil {
			return "", fmt.Errorf("docker compose pull: %w", err)
		}
	}

	actionParts := strings.Fields(string(opts.Action))
	args = append(args, actionParts...)

	var buf bytes.Buffer
	err := r.runCompose(ctx, opts.StackPath, args, opts.DryRun, &buf)
	return buf.String(), err
}

func (r *LocalRunner) runCompose(ctx context.Context, dir string, args []string, dryRun bool, out io.Writer) error {
	if dryRun {
		fmt.Fprintf(out, "[dry-run] docker %s (in %s)\n", strings.Join(args, " "), dir)
		return nil
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}

func (r *LocalRunner) Logs(ctx context.Context, opts LogOptions) error {
	args := []string{"compose", "logs"}
	if opts.Follow {
		args = append(args, "--follow")
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = opts.ComposeDir
	cmd.Stdout = opts.Writer
	cmd.Stderr = opts.Writer

	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-ctx.Done():
		_ = cmd.Process.Signal(os.Interrupt)
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func (r *LocalRunner) Exec(ctx context.Context, opts ExecOptions) error {
	args := []string{"compose", "exec", opts.Service}
	args = append(args, opts.Cmd...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	// ComposeDir is already the full stack path for local runners; don't append Stack again
	cmd.Dir = opts.ComposeDir
	cmd.Stdin = opts.Stdin
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr
	return cmd.Run()
}

func (r *LocalRunner) Ping(_ context.Context) error {
	cmd := exec.Command("docker", "info")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker is not reachable locally: %w", err)
	}
	return nil
}

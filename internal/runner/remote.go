package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkmode_dev/dockflux/internal/inventory"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type RemoteRunner struct {
	host   *inventory.Host
	client *ssh.Client
}

func newRemoteRunner(host *inventory.Host) (*RemoteRunner, error) {
	keyBytes, err := os.ReadFile(host.Key)
	if err != nil {
		return nil, fmt.Errorf("reading SSH key %s: %w", host.Key, err)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parsing SSH key: %w", err)
	}

	hostKeyCallback, err := buildHostKeyCallback()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"WARNING: could not load ~/.ssh/known_hosts (%v); SSH host key verification is DISABLED — add the host key first with ssh-keyscan\n", err)
		hostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec
	}

	cfg := &ssh.ClientConfig{
		User:            host.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKeyCallback,
	}

	addr := fmt.Sprintf("%s:%d", host.Host, host.Port)
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("SSH dial %s: %w", addr, err)
	}

	return &RemoteRunner{host: host, client: client}, nil
}

func buildHostKeyCallback() (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
	return knownhosts.New(knownHostsPath)
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes.
// This is the standard POSIX shell quoting approach — safe for any path or name.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (r *RemoteRunner) CopyStack(ctx context.Context, opts RunOptions) error {
	if opts.DryRun {
		fmt.Printf("[dry-run] sftp: copy %s → %s:%s/%s\n",
			opts.StackPath, r.host.Host, opts.ComposeDir, opts.Stack)
		return nil
	}

	sftpClient, err := sftp.NewClient(r.client)
	if err != nil {
		return fmt.Errorf("creating SFTP client: %w", err)
	}
	defer sftpClient.Close()

	remoteStackDir := opts.ComposeDir + "/" + opts.Stack

	return filepath.Walk(opts.StackPath, func(localPath string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		rel, err := filepath.Rel(opts.StackPath, localPath)
		if err != nil {
			return err
		}
		remotePath := remoteStackDir + "/" + rel

		if info.IsDir() {
			return sftpClient.MkdirAll(remotePath)
		}

		src, err := os.Open(localPath)
		if err != nil {
			return fmt.Errorf("opening %s: %w", localPath, err)
		}
		defer src.Close()

		dst, err := sftpClient.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
		if err != nil {
			return fmt.Errorf("creating remote %s: %w", remotePath, err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return fmt.Errorf("copying to %s: %w", remotePath, err)
		}

		return sftpClient.Chmod(remotePath, info.Mode())
	})
}

func (r *RemoteRunner) ComposeRun(ctx context.Context, opts RunOptions) (string, error) {
	remoteDir := shellQuote(opts.ComposeDir + "/" + opts.Stack)

	var sb strings.Builder

	if opts.Pull {
		out, err := r.sshRun(ctx, fmt.Sprintf("cd %s && docker compose pull", remoteDir), opts.DryRun)
		sb.WriteString(out)
		if err != nil {
			return sb.String(), fmt.Errorf("docker compose pull: %w", err)
		}
	}

	cmd := fmt.Sprintf("cd %s && docker compose %s", remoteDir, string(opts.Action))
	out, err := r.sshRun(ctx, cmd, opts.DryRun)
	sb.WriteString(out)
	return sb.String(), err
}

func (r *RemoteRunner) sshRun(ctx context.Context, command string, dryRun bool) (string, error) {
	if dryRun {
		return fmt.Sprintf("[dry-run] ssh %s: %s\n", r.host.Host, command), nil
	}

	session, err := r.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new SSH session: %w", err)
	}
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf
	session.Stderr = &buf

	done := make(chan error, 1)
	go func() { done <- session.Run(command) }()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGINT)
		<-done
		return buf.String(), ctx.Err()
	case err := <-done:
		return buf.String(), err
	}
}

func (r *RemoteRunner) Logs(ctx context.Context, opts LogOptions) error {
	session, err := r.client.NewSession()
	if err != nil {
		return fmt.Errorf("new SSH session: %w", err)
	}
	defer session.Close()

	session.Stdout = opts.Writer
	session.Stderr = opts.Writer

	followFlag := ""
	if opts.Follow {
		followFlag = "--follow"
	}
	remoteDir := shellQuote(opts.ComposeDir + "/" + opts.Stack)
	cmd := fmt.Sprintf("cd %s && docker compose logs %s", remoteDir, followFlag)

	if err := session.Start(cmd); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() { done <- session.Wait() }()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGINT)
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func (r *RemoteRunner) Exec(ctx context.Context, opts ExecOptions) error {
	session, err := r.client.NewSession()
	if err != nil {
		return fmt.Errorf("new SSH session: %w", err)
	}
	defer session.Close()

	session.Stdin = opts.Stdin
	session.Stdout = opts.Stdout
	session.Stderr = opts.Stderr

	if err := session.RequestPty("xterm-256color", 40, 80, ssh.TerminalModes{}); err != nil {
		return fmt.Errorf("requesting pty: %w", err)
	}

	remoteDir := shellQuote(opts.ComposeDir + "/" + opts.Stack)
	quotedArgs := make([]string, len(opts.Cmd))
	for i, arg := range opts.Cmd {
		quotedArgs[i] = shellQuote(arg)
	}
	cmdStr := fmt.Sprintf("cd %s && docker compose exec %s %s",
		remoteDir, shellQuote(opts.Service), strings.Join(quotedArgs, " "))

	done := make(chan error, 1)
	go func() { done <- session.Run(cmdStr) }()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGINT)
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// EnsureDir verifies that dir is writable on host, creating it and fixing
// ownership via sudo if not. Returns nil when the directory is ready.
func EnsureDir(ctx context.Context, host *inventory.Host, dir string) error {
	r, err := newRemoteRunner(host)
	if err != nil {
		return err
	}
	defer r.client.Close()

	// Check: does the dir exist and is it writable?
	out, err := r.sshRun(ctx, fmt.Sprintf("test -d %s && test -w %s && echo ok", shellQuote(dir), shellQuote(dir)), false)
	if err == nil && strings.TrimSpace(out) == "ok" {
		return nil
	}

	// Dir exists but isn't writable — fix ownership. Create it first only if missing.
	fix := fmt.Sprintf(
		"if [ ! -d %s ]; then sudo mkdir -p %s; fi && sudo chown $(id -un):$(id -gn) %s",
		shellQuote(dir), shellQuote(dir), shellQuote(dir),
	)
	if _, err := r.sshRun(ctx, fix, false); err != nil {
		return fmt.Errorf("could not fix permissions on %s (try: sudo chown $USER:$USER %s): %w", dir, dir, err)
	}
	return nil
}

func (r *RemoteRunner) Ping(ctx context.Context) error {
	dialer := net.Dialer{}
	addr := fmt.Sprintf("%s:%d", r.host.Host, r.host.Port)
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("cannot reach %s: %w", addr, err)
	}
	conn.Close()
	return nil
}

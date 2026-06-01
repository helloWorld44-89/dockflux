package importer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/helloWorld44-89/dockflux/internal/inventory"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// composeFiles lists the standard filenames we attempt to pull from each stack dir.
// .env files are intentionally excluded — store secrets with 'dockflux secrets set'.
var composeFiles = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
	"docker-compose.override.yml",
	"docker-compose.override.yaml",
}

// StackResult describes the outcome of importing one stack directory.
type StackResult struct {
	Stack   string
	Files   []string // filenames downloaded; nil means the stack was skipped
	Skipped bool     // true when the local dir already exists and force=false
}

// ImportFromHost connects to host via SSH/SFTP, lists stack directories under
// host.ComposeDir, and downloads compose files into destDir/<stack>/.
// Directories that already exist locally are skipped unless force is true.
func ImportFromHost(ctx context.Context, host *inventory.Host, destDir string, force bool) ([]StackResult, error) {
	client, err := dialSSH(host)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return nil, fmt.Errorf("opening SFTP session: %w", err)
	}
	defer sftpClient.Close()

	entries, err := sftpClient.ReadDir(host.ComposeDir)
	if err != nil {
		return nil, fmt.Errorf("listing %s on %s: %w", host.ComposeDir, host.Host, err)
	}

	var results []StackResult

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		stackName := entry.Name()
		localStackDir := filepath.Join(destDir, stackName)

		if !force {
			if _, err := os.Stat(localStackDir); err == nil {
				results = append(results, StackResult{Stack: stackName, Skipped: true})
				continue
			}
		}

		if err := os.MkdirAll(localStackDir, 0755); err != nil {
			return results, fmt.Errorf("creating %s: %w", localStackDir, err)
		}

		remoteStackDir := host.ComposeDir + "/" + stackName
		var downloaded []string

		for _, fname := range composeFiles {
			remotePath := remoteStackDir + "/" + fname
			localPath := filepath.Join(localStackDir, fname)

			if err := downloadFile(sftpClient, remotePath, localPath); err == nil {
				downloaded = append(downloaded, fname)
			}
			// non-nil err means file doesn't exist on remote — skip silently
		}

		results = append(results, StackResult{Stack: stackName, Files: downloaded})
	}

	return results, nil
}

func downloadFile(sftpClient *sftp.Client, remotePath, localPath string) error {
	src, err := sftpClient.Open(remotePath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(localPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

func dialSSH(host *inventory.Host) (*ssh.Client, error) {
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
			"WARNING: could not load ~/.ssh/known_hosts (%v); SSH host key verification is DISABLED\n", err)
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
	return client, nil
}

func buildHostKeyCallback() (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return knownhosts.New(filepath.Join(home, ".ssh", "known_hosts"))
}

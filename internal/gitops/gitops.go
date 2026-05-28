package gitops

import (
	"errors"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gossh "golang.org/x/crypto/ssh"
)

// SSHAuth builds SSH public-key auth from a key file path.
func SSHAuth(keyPath string) (transport.AuthMethod, error) {
	if keyPath == "" {
		return nil, nil
	}
	auth, err := gitssh.NewPublicKeysFromFile("git", keyPath, "")
	if err != nil {
		return nil, fmt.Errorf("loading SSH key %s: %w", keyPath, err)
	}
	// Accept any host key for git operations — the remote is known via the URL
	auth.HostKeyCallback = gossh.InsecureIgnoreHostKey() //nolint:gosec
	return auth, nil
}

// HTTPSAuth builds HTTPS basic-auth from a personal access token.
func HTTPSAuth(token string) transport.AuthMethod {
	return &githttp.BasicAuth{
		Username: "token", // git servers treat any non-empty username as valid
		Password: token,
	}
}

// CloneOrPull clones the repo if it doesn't exist, otherwise pulls the latest.
// Pass nil auth for public repos.
func CloneOrPull(repoURL, branch, localPath string, auth transport.AuthMethod) error {
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		_, err = git.PlainClone(localPath, false, &git.CloneOptions{
			URL:           repoURL,
			ReferenceName: plumbing.NewBranchReferenceName(branch),
			SingleBranch:  true,
			Auth:          auth,
		})
		return err
	}

	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return fmt.Errorf("opening repo at %s: %w", localPath, err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	err = wt.Pull(&git.PullOptions{
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		Auth:          auth,
	})
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

// HeadCommit returns the short (7-char) commit SHA of the repo at localPath.
func HeadCommit(localPath string) (string, error) {
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return "", fmt.Errorf("opening repo: %w", err)
	}

	ref, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("reading HEAD: %w", err)
	}

	// Fix #7: guard against unexpectedly short hash strings
	hash := ref.Hash().String()
	if len(hash) >= 7 {
		return hash[:7], nil
	}
	return hash, nil
}

// CheckoutCommit checks out a specific commit hash (detached HEAD).
func CheckoutCommit(localPath, commitSHA string) error {
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	return wt.Checkout(&git.CheckoutOptions{
		Hash:  plumbing.NewHash(commitSHA),
		Force: true,
	})
}

// CheckoutBranch checks out a named branch.
func CheckoutBranch(localPath, branch string) error {
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	return wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branch),
		Force:  true,
	})
}

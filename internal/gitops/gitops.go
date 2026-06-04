package gitops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	gogitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
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

// InitRepo creates a new git repo at localPath with a stacks/ directory and
// .gitignore, then makes an initial commit.
func InitRepo(localPath, branch string) error {
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	repo, err := git.PlainInit(localPath, false)
	if err != nil {
		return fmt.Errorf("git init: %w", err)
	}

	stacksDir := filepath.Join(localPath, "stacks")
	if err := os.MkdirAll(stacksDir, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stacksDir, ".gitkeep"), nil, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(localPath, ".gitignore"), []byte("*.env\n.env*\n!.env.example\n"), 0644); err != nil {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	if _, err := wt.Add("."); err != nil {
		return err
	}
	_, err = wt.Commit("init: initial dockflux stacks repo", &git.CommitOptions{
		Author: &object.Signature{Name: "dockflux", Email: "dockflux@localhost", When: time.Now()},
	})
	if err != nil {
		return err
	}

	// rename default branch to the requested name
	head, err := repo.Head()
	if err != nil {
		return err
	}
	ref := plumbing.NewHashReference(plumbing.NewBranchReferenceName(branch), head.Hash())
	if err := repo.Storer.SetReference(ref); err != nil {
		return err
	}
	symref := plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName(branch))
	return repo.Storer.SetReference(symref)
}

// AddRemote adds a named remote to an existing local repo.
func AddRemote(localPath, name, remoteURL string) error {
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return err
	}
	_, err = repo.CreateRemote(&gogitconfig.RemoteConfig{
		Name: name,
		URLs: []string{remoteURL},
	})
	return err
}

// PushRepo pushes all refs to the named remote.
func PushRepo(localPath string, auth transport.AuthMethod) error {
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return err
	}
	return repo.Push(&git.PushOptions{
		RemoteName: "origin",
		Auth:       auth,
	})
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

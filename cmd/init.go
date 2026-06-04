package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	gitopstransport "github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/darkmode_dev/dockflux/internal/gitops"
	"github.com/darkmode_dev/dockflux/internal/importer"
	"github.com/darkmode_dev/dockflux/internal/inventory"
	"github.com/darkmode_dev/dockflux/internal/runner"
	"github.com/darkmode_dev/dockflux/internal/secrets"
	"github.com/darkmode_dev/dockflux/internal/ui"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Interactive setup wizard — creates dockflux.yml, inventory.yml, and secrets store",
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	printBanner()

	// ── Step 1: Repo mode ────────────────────────────────────────────────────
	var (
		repoURL       string
		authMethod    = "ssh"
		sshKeyPath    = expandHome("~/.ssh/id_rsa")
		sshPassphrase string
		httpsToken    string
		branch        = "main"
		localPath     = expandHome("~/.dockflux/repo")
		createNew     bool
	)

	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Step 1 of 4 — Repository").
				Description("Where are your Docker Compose stacks stored?"),
			huh.NewSelect[bool]().
				Title("Repo setup").
				Options(
					huh.NewOption("I have an existing git repo", false),
					huh.NewOption("Create a new stacks repo for me", true),
				).
				Value(&createNew),
		),
	).WithTheme(huh.ThemeDracula()).Run(); err != nil {
		return err
	}

	if createNew {
		if err := runInitNewRepo(cmd, &repoURL, &authMethod, &sshKeyPath, &sshPassphrase, &httpsToken, &branch, &localPath); err != nil {
			return err
		}
	} else {
		if err := runInitExistingRepo(&repoURL, &authMethod, &sshKeyPath, &sshPassphrase, &httpsToken, &branch, &localPath); err != nil {
			return err
		}
	}

	// ── Step 3: Secrets master password ──────────────────────────────────────
	var masterPass, masterPass2 string

	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Step 3 of 4 — Secrets store").
				Description("dockflux encrypts your credentials and .env values with AES-256-GCM.\nSet DOCKFLUX_SECRETS_PASSWORD in the environment for headless/watch mode."),
			huh.NewInput().
				Title("Master password").
				EchoMode(huh.EchoModePassword).
				Value(&masterPass).
				Validate(func(s string) error {
					if len(s) < 8 {
						return fmt.Errorf("password must be at least 8 characters")
					}
					return nil
				}),
			huh.NewInput().
				Title("Confirm master password").
				EchoMode(huh.EchoModePassword).
				Value(&masterPass2).
				Validate(func(s string) error {
					if s != masterPass {
						return fmt.Errorf("passwords do not match")
					}
					return nil
				}),
		),
	).WithTheme(huh.ThemeDracula()).Run(); err != nil {
		return err
	}

	// ── Step 4: Hosts ─────────────────────────────────────────────────────────
	var remoteHosts []hostEntry

	for {
		var addHost bool
		prompt := "Add a remote host?"
		if len(remoteHosts) > 0 {
			prompt = "Add another remote host?"
		}
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewNote().Title("Step 4 of 4 — Hosts"),
				huh.NewConfirm().
					Title(prompt).
					Description("The local host is always included. Add remote hosts reachable via SSH.").
					Value(&addHost),
			),
		).WithTheme(huh.ThemeDracula()).Run(); err != nil {
			return err
		}

		if !addHost {
			break
		}

		var h hostEntry
		h.key = sshKeyPath // default to the git key
		h.user = "deploy"
		h.composeDir = "/opt/stacks"

		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().Title("Host alias").Placeholder("web01").Value(&h.alias).
					Validate(func(s string) error {
						if s == "" {
							return fmt.Errorf("alias is required")
						}
						return nil
					}),
				huh.NewInput().Title("Address or IP").Placeholder("192.168.1.10").Value(&h.address).
					Validate(func(s string) error {
						if s == "" {
							return fmt.Errorf("address is required")
						}
						return nil
					}),
				huh.NewInput().Title("SSH user").Value(&h.user),
				huh.NewInput().Title("SSH key path").Value(&h.key),
				huh.NewInput().Title("Groups").Description("Comma-separated, e.g. production,web").Value(&h.groups),
				huh.NewInput().Title("Remote compose directory").Value(&h.composeDir),
			),
		).WithTheme(huh.ThemeDracula()).Run(); err != nil {
			return err
		}

		remoteHosts = append(remoteHosts, h)
	}

	// ── Write config files ────────────────────────────────────────────────────
	fmt.Println()
	stop := ui.Spinner("Writing configuration files")

	if err := os.MkdirAll(".dockflux", 0755); err != nil {
		stop(false, "Failed to create .dockflux directory")
		return err
	}

	secretsPath := secrets.DefaultPath()

	cfgContent := buildDockfluxYML(repoURL, authMethod, branch, localPath, secretsPath)
	if err := writeIfNotExists(".dockflux/dockflux.yml", cfgContent); err != nil {
		stop(false, "Failed to write .dockflux/dockflux.yml")
		return err
	}

	invContent := buildInventoryYML(remoteHosts)
	if err := writeIfNotExists(".dockflux/inventory.yml", invContent); err != nil {
		stop(false, "Failed to write .dockflux/inventory.yml")
		return err
	}

	// Save credentials to encrypted secrets store
	store := secrets.New()
	if authMethod == "https" {
		store.Git.HTTPSToken = httpsToken
	} else if sshPassphrase != "" {
		store.Git.KeyPassphrase = sshPassphrase
	}
	if err := secrets.Save(secretsPath, masterPass, store); err != nil {
		stop(false, "Failed to create secrets store")
		return err
	}

	stop(true, "Configuration files written")

	// ── Initial sync ──────────────────────────────────────────────────────────
	var doSync bool
	if createNew {
		// Repo was just created locally — no need to clone
		doSync = true
	} else if err := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Clone the stacks repo now?").
				Description(fmt.Sprintf("Cloning %s → %s", repoURL, localPath)).
				Value(&doSync),
		),
	).WithTheme(huh.ThemeDracula()).Run(); err != nil {
		return err
	}

	if doSync && !createNew {
		syncStop := ui.Spinner(fmt.Sprintf("Cloning %s", repoURL))

		// Fix #4: build auth based on the method the user actually chose
		var cloneAuth gitopstransport.AuthMethod
		if authMethod == "ssh" {
			var authErr error
			cloneAuth, authErr = gitops.SSHAuth(sshKeyPath)
			if authErr != nil {
				syncStop(false, "Invalid SSH key")
				return authErr
			}
		} else {
			cloneAuth = gitops.HTTPSAuth(httpsToken)
		}

		if err := gitops.CloneOrPull(repoURL, branch, localPath, cloneAuth); err != nil {
			syncStop(false, "Clone failed")
			ui.Warn("You can retry later with: dockflux sync")
		} else {
			commit, _ := gitops.HeadCommit(localPath)
			syncStop(true, fmt.Sprintf("Cloned at %s", commit))
		}
	}

	// Write inventory.yml into the repo so it travels with the stacks.
	if doSync {
		repoInvPath := filepath.Join(expandHome(localPath), "inventory.yml")
		if err := writeIfNotExists(repoInvPath, invContent); err != nil {
			ui.Warn("Could not write inventory.yml to repo: %v", err)
		}
	}

	// ── Check / fix remote compose_dir permissions ────────────────────────────
	if len(remoteHosts) > 0 {
		fmt.Println()
		pterm.DefaultSection.Println("Checking remote permissions")
		for _, h := range remoteHosts {
			host := hostEntryToInventoryHost(h)
			stop := ui.Spinner(fmt.Sprintf("Checking %s (%s)", host.Name, host.Host))
			if err := runner.EnsureDir(rootCmd.Context(), host, h.composeDir); err != nil {
				stop(false, fmt.Sprintf("%s: %v", host.Name, err))
			} else {
				stop(true, fmt.Sprintf("%s: %s is writable", host.Name, h.composeDir))
			}
		}
	}

	// ── Import existing stacks ────────────────────────────────────────────────
	if doSync && len(remoteHosts) > 0 {
		var doImport bool
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Import existing compose stacks from your hosts?").
					Description("Downloads docker-compose.yml files from each host's compose directory\ninto your local repo so you can start managing them with dockflux.\n.env files are skipped — store secrets later with 'dockflux secrets set'.").
					Value(&doImport),
			),
		).WithTheme(huh.ThemeDracula()).Run(); err != nil {
			return err
		}

		if doImport {
			stacksDir := filepath.Join(expandHome(localPath), "stacks")
			if err := os.MkdirAll(stacksDir, 0755); err != nil {
				ui.Warn("Could not create stacks directory: %v", err)
			} else {
				for _, h := range remoteHosts {
					host := hostEntryToInventoryHost(h)
					stop := ui.Spinner(fmt.Sprintf("Importing from %s (%s)", host.Name, host.Host))
					results, err := importer.ImportFromHost(rootCmd.Context(), host, stacksDir, false)
					if err != nil {
						stop(false, fmt.Sprintf("%s: %v", host.Name, err))
						continue
					}
					imported := 0
					for _, r := range results {
						if len(r.Files) > 0 {
							imported++
							pterm.Success.Printf("  %-24s %v\n", r.Stack, r.Files)
						}
					}
					stop(true, fmt.Sprintf("%s: imported %d stack(s)", host.Name, imported))
				}
				pterm.Println()
				pterm.Info.Printf("Stacks written to %s — review, then commit to your git repo.\n", stacksDir)
			}
		}
	}

	// ── Summary ───────────────────────────────────────────────────────────────
	printSummary(remoteHosts, secretsPath, doSync)
	return nil
}

func runInitExistingRepo(repoURL, authMethod, sshKeyPath, sshPassphrase, httpsToken, branch, localPath *string) error {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Step 1 of 4 — Repository").
				Description("Where are your Docker Compose stacks stored?"),
			huh.NewInput().
				Title("Git repo URL").
				Placeholder("git@github.com:you/stacks.git").
				Value(repoURL).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("repo URL is required")
					}
					return nil
				}),
			huh.NewSelect[string]().
				Title("Authentication method").
				Options(
					huh.NewOption("SSH key", "ssh"),
					huh.NewOption("HTTPS personal access token", "https"),
				).
				Value(authMethod),
			huh.NewInput().
				Title("Branch").
				Value(branch),
			huh.NewInput().
				Title("Local cache path").
				Description("Where the repo will be cloned on this machine").
				Value(localPath),
		),
		huh.NewGroup(
			huh.NewNote().Title("Step 2 of 4 — SSH credentials"),
			huh.NewInput().
				Title("SSH key path").
				Value(sshKeyPath).
				Validate(func(s string) error {
					expanded := expandHome(s)
					if _, err := os.Stat(expanded); err != nil {
						return fmt.Errorf("key file not found: %s", expanded)
					}
					return nil
				}),
			huh.NewInput().
				Title("SSH key passphrase").
				Description("Leave empty if your key has no passphrase").
				EchoMode(huh.EchoModePassword).
				Value(sshPassphrase),
		).WithHideFunc(func() bool { return *authMethod != "ssh" }),
		huh.NewGroup(
			huh.NewNote().Title("Step 2 of 4 — HTTPS credentials"),
			huh.NewInput().
				Title("Personal access token").
				Description("GitHub/GitLab PAT with repo read access").
				EchoMode(huh.EchoModePassword).
				Value(httpsToken).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("token is required for HTTPS auth")
					}
					return nil
				}),
		).WithHideFunc(func() bool { return *authMethod != "https" }),
	).WithTheme(huh.ThemeDracula()).Run()
}

func runInitNewRepo(cmd *cobra.Command, repoURL, authMethod, sshKeyPath, sshPassphrase, httpsToken, branch, localPath *string) error {
	// Ask where to create the local repo
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Step 1 of 4 — Create new stacks repo").
				Description("dockflux will scaffold a git repo with a stacks/ directory.\nYou'll then push it to GitHub/GitLab to use as your source of truth."),
			huh.NewInput().
				Title("Local path for your new repo").
				Placeholder(expandHome("~/stacks")).
				Value(localPath).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("path is required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Default branch name").
				Value(branch),
		),
	).WithTheme(huh.ThemeDracula()).Run(); err != nil {
		return err
	}

	expanded := expandHome(*localPath)

	// Scaffold the repo
	stop := ui.Spinner(fmt.Sprintf("Creating repo at %s", expanded))
	if err := gitops.InitRepo(expanded, *branch); err != nil {
		stop(false, "Failed to create repo")
		return err
	}
	stop(true, fmt.Sprintf("Repo created at %s", expanded))

	// Instruct the user to create a remote
	pterm.Println()
	pterm.Info.Println("Next: create an empty repo on GitHub or GitLab (no README, no .gitignore),")
	pterm.Info.Println("then paste its URL below.")
	pterm.Println()

	// Ask for remote URL + auth
	var remoteURL string
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().Title("Step 2 of 4 — Remote & authentication"),
			huh.NewInput().
				Title("Remote repo URL").
				Placeholder("git@github.com:you/stacks.git").
				Value(&remoteURL).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("remote URL is required")
					}
					return nil
				}),
			huh.NewSelect[string]().
				Title("Authentication method").
				Options(
					huh.NewOption("SSH key", "ssh"),
					huh.NewOption("HTTPS personal access token", "https"),
				).
				Value(authMethod),
		),
		huh.NewGroup(
			huh.NewNote().Title("Step 2 of 4 — SSH credentials"),
			huh.NewInput().
				Title("SSH key path").
				Value(sshKeyPath).
				Validate(func(s string) error {
					expanded := expandHome(s)
					if _, err := os.Stat(expanded); err != nil {
						return fmt.Errorf("key file not found: %s", expanded)
					}
					return nil
				}),
			huh.NewInput().
				Title("SSH key passphrase").
				Description("Leave empty if your key has no passphrase").
				EchoMode(huh.EchoModePassword).
				Value(sshPassphrase),
		).WithHideFunc(func() bool { return *authMethod != "ssh" }),
		huh.NewGroup(
			huh.NewNote().Title("Step 2 of 4 — HTTPS credentials"),
			huh.NewInput().
				Title("Personal access token").
				Description("GitHub/GitLab PAT with repo write access").
				EchoMode(huh.EchoModePassword).
				Value(httpsToken).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("token is required for HTTPS auth")
					}
					return nil
				}),
		).WithHideFunc(func() bool { return *authMethod != "https" }),
	).WithTheme(huh.ThemeDracula()).Run(); err != nil {
		return err
	}

	*repoURL = remoteURL

	// Add remote
	if err := gitops.AddRemote(expanded, "origin", remoteURL); err != nil {
		ui.Warn("Could not add remote: %v", err)
	}

	// Offer to push the initial commit
	var doPush bool
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Push the initial commit now?").
				Description(fmt.Sprintf("Pushes the scaffolded repo to %s", remoteURL)).
				Value(&doPush),
		),
	).WithTheme(huh.ThemeDracula()).Run(); err != nil {
		return err
	}

	if doPush {
		var pushAuth gitopstransport.AuthMethod
		var authErr error
		if *authMethod == "ssh" {
			pushAuth, authErr = gitops.SSHAuth(*sshKeyPath)
		} else {
			pushAuth = gitops.HTTPSAuth(*httpsToken)
		}
		if authErr != nil {
			ui.Warn("Invalid SSH key: %v", authErr)
		} else {
			pushStop := ui.Spinner("Pushing to remote...")
			if err := gitops.PushRepo(expanded, pushAuth); err != nil {
				pushStop(false, "Push failed — run 'git push -u origin "+*branch+"' manually")
				ui.Warn("%v", err)
			} else {
				pushStop(true, "Pushed successfully")
			}
		}
	}

	// Point localPath at the repo we just created so sync is skipped later
	*localPath = expanded
	return nil
}

func hostEntryToInventoryHost(h hostEntry) *inventory.Host {
	return &inventory.Host{
		Name:       h.alias,
		Type:       inventory.HostTypeRemote,
		Host:       h.address,
		Port:       22,
		User:       h.user,
		Key:        expandHome(h.key),
		ComposeDir: h.composeDir,
	}
}

func printBanner() {
	pterm.Println()
	pterm.DefaultBigText.WithLetters(
		pterm.NewLettersFromStringWithStyle("dock", pterm.NewStyle(pterm.FgCyan)),
		pterm.NewLettersFromStringWithStyle("flux", pterm.NewStyle(pterm.FgBlue)),
	).Render()
	pterm.DefaultBasicText.Println("Deploy Docker Compose stacks from git to a fleet of hosts")
	pterm.Println()
}

func printSummary(remoteHosts []hostEntry, secretsPath string, synced bool) {
	pterm.Println()
	pterm.DefaultSection.Println("Setup complete!")

	items := []pterm.BulletListItem{
		{Level: 0, Text: ".dockflux/dockflux.yml", TextStyle: pterm.NewStyle(pterm.FgGreen)},
		{Level: 0, Text: ".dockflux/inventory.yml", TextStyle: pterm.NewStyle(pterm.FgGreen)},
		{Level: 0, Text: secretsPath + " (encrypted)", TextStyle: pterm.NewStyle(pterm.FgGreen)},
	}
	_ = pterm.DefaultBulletList.WithItems(items).Render()

	pterm.Println()
	pterm.DefaultSection.Println("Next steps")

	nextSteps := []pterm.BulletListItem{}
	if !synced {
		nextSteps = append(nextSteps, pterm.BulletListItem{Level: 0, Text: "dockflux sync"})
	}
	nextSteps = append(nextSteps,
		pterm.BulletListItem{Level: 0, Text: "dockflux hosts ping"},
		pterm.BulletListItem{Level: 0, Text: "dockflux deploy <stack> --local"},
		pterm.BulletListItem{Level: 0, Text: "dockflux secrets set <stack> DB_PASSWORD  # store .env secrets"},
		pterm.BulletListItem{Level: 0, Text: "sudo dockflux service install              # enable watch on boot"},
	)
	_ = pterm.DefaultBulletList.WithItems(nextSteps).Render()

	pterm.Println()
	pterm.Info.Println("For the watch daemon, export DOCKFLUX_SECRETS_PASSWORD in the service unit.")
	pterm.Println()
}

func buildDockfluxYML(repoURL, authMethod, branch, localPath, secretsPath string) string {
	keyLine := ""
	if authMethod == "ssh" {
		keyLine = "\n  key: ~/.ssh/id_rsa"
	}
	return fmt.Sprintf(`repo:
  url: %s
  branch: %s
  local_path: %s%s

stacks_dir: stacks/
state_file: ./dockflux.lock
secrets_file: %s
`, repoURL, branch, localPath, keyLine, secretsPath)
}

type hostEntry struct {
	alias      string
	address    string
	user       string
	key        string
	groups     string
	composeDir string
}

func buildInventoryYML(remoteHosts []hostEntry) string {
	sb := &strings.Builder{}
	sb.WriteString("hosts:\n  local:\n    type: local\n")
	for _, h := range remoteHosts {
		sb.WriteString(fmt.Sprintf("\n  %s:\n", h.alias))
		sb.WriteString(fmt.Sprintf("    host: %s\n", h.address))
		sb.WriteString(fmt.Sprintf("    user: %s\n", h.user))
		sb.WriteString(fmt.Sprintf("    key: %s\n", h.key))
		sb.WriteString(fmt.Sprintf("    compose_dir: %s\n", h.composeDir))
		if h.groups != "" {
			sb.WriteString("    groups:\n")
			for _, g := range strings.Split(h.groups, ",") {
				sb.WriteString(fmt.Sprintf("      - %s\n", strings.TrimSpace(g)))
			}
		}
	}
	return sb.String()
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func writeIfNotExists(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		ui.Warn("%s already exists, skipping", path)
		return nil
	}
	return os.WriteFile(path, []byte(content), 0644)
}

package secretscmd

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/helloWorld44-89/dockflux/internal/config"
	"github.com/helloWorld44-89/dockflux/internal/secrets"
	"github.com/helloWorld44-89/dockflux/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var editCmd = &cobra.Command{
	Use:   "edit <stack>",
	Short: "Pick keys for a stack and edit their values interactively",
	Args:  cobra.ExactArgs(1),
	RunE:  runEdit,
}

func runEdit(cmd *cobra.Command, args []string) error {
	stackName := args[0]

	cfgPath := viper.ConfigFileUsed()
	if cfgPath == "" {
		cfgPath = config.FindConfigFile()
	}
	if cfgPath == "" {
		cfgPath = ".dockflux/dockflux.yml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	stackPath := filepath.Join(cfg.Repo.LocalPath, cfg.StacksDir, stackName)
	exampleKeys, err := secrets.ExampleKeys(stackPath)
	if err != nil {
		return err
	}

	password, err := secrets.PromptPassword("Master password")
	if err != nil {
		return err
	}

	store, err := secrets.Load(cfg.SecretsFile, password)
	if err != nil {
		return err
	}
	existing := store.GetStackSecrets(stackName)

	// Build the candidate key list: .env.example keys first (in file order),
	// then any keys already set that aren't in .env.example.
	seen := make(map[string]bool)
	var keys []string
	for _, k := range exampleKeys {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	for k := range existing {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}

	selected, err := selectKeys(stackName, keys, existing)
	if err != nil {
		return err
	}

	newKeys, err := promptNewKeys(seen)
	if err != nil {
		return err
	}
	selected = append(selected, newKeys...)

	if len(selected) == 0 {
		ui.Info("No keys selected, nothing changed.")
		return nil
	}

	for _, key := range selected {
		value, err := promptKeyValue(stackName, key, existing[key] != "")
		if err != nil {
			return err
		}
		store.SetStackSecret(stackName, key, value)
	}

	if err := secrets.Save(cfg.SecretsFile, password, store); err != nil {
		return err
	}

	ui.Success("Updated %d key(s) for %s", len(selected), stackName)
	return nil
}

// selectKeys shows a checklist of the stack's known keys and returns the
// ones the user picked to edit. Returns nil if there are no known keys yet.
func selectKeys(stackName string, keys []string, existing map[string]string) ([]string, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	options := make([]huh.Option[string], len(keys))
	for i, k := range keys {
		label := k
		if existing[k] != "" {
			label = k + " (set)"
		}
		options[i] = huh.NewOption(label, k)
	}

	var selected []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(fmt.Sprintf("Select %s keys to edit", stackName)).
				Options(options...).
				Value(&selected),
		),
	)
	if err := form.Run(); err != nil {
		return nil, err
	}
	return selected, nil
}

// promptNewKeys repeatedly asks whether to add a key not already known,
// skipping any name already present in seen.
func promptNewKeys(seen map[string]bool) ([]string, error) {
	var added []string
	for {
		var addMore bool
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Add a new key?").
					Value(&addMore),
			),
		).Run(); err != nil {
			return nil, err
		}
		if !addMore {
			return added, nil
		}

		var key string
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Key name").
					Value(&key).
					Validate(func(s string) error {
						if s == "" {
							return fmt.Errorf("key cannot be empty")
						}
						if seen[s] {
							return fmt.Errorf("key %q already selected", s)
						}
						return nil
					}),
			),
		).Run(); err != nil {
			return nil, err
		}

		seen[key] = true
		added = append(added, key)
	}
}

func promptKeyValue(stackName, key string, alreadySet bool) (string, error) {
	title := fmt.Sprintf("Value for %s/%s", stackName, key)
	if alreadySet {
		title += " (currently set)"
	}

	var value string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(title).
				EchoMode(huh.EchoModePassword).
				Value(&value).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("value cannot be empty")
					}
					return nil
				}),
		),
	).Run()
	return value, err
}

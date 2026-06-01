package cmd

import (
	"os"

	"github.com/helloWorld44-89/dockflux/internal/secrets"
)

// loadStackSecrets loads secrets for stackName from the encrypted store.
// Returns nil (not an error) if the secrets file does not exist yet.
func loadStackSecrets(secretsPath, stackName string) (map[string]string, error) {
	if _, err := os.Stat(secretsPath); os.IsNotExist(err) {
		return nil, nil
	}

	password, err := secrets.PromptPassword("Secrets master password")
	if err != nil {
		return nil, err
	}

	store, err := secrets.Load(secretsPath, password)
	if err != nil {
		return nil, err
	}

	return store.GetStackSecrets(stackName), nil
}

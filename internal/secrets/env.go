package secrets

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateEnv reads .env.example from stackPath and fills in values from
// stackSecrets. Lines without a matching secret are left as-is (placeholder
// stays in the output so the container still gets the variable, just with the
// example value). Returns nil if no .env.example file exists.
func GenerateEnv(stackPath string, stackSecrets map[string]string) ([]byte, error) {
	examplePath := filepath.Join(stackPath, ".env.example")
	f, err := os.Open(examplePath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("opening .env.example: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Pass through comments and blank lines unchanged
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			buf.WriteString(line + "\n")
			continue
		}

		// Split on first '=' to get key
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			buf.WriteString(line + "\n")
			continue
		}

		key := strings.TrimSpace(parts[0])
		if val, ok := stackSecrets[key]; ok {
			buf.WriteString(key + "=" + val + "\n")
		} else {
			// Keep the example value so compose doesn't error on a missing var
			buf.WriteString(line + "\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading .env.example: %w", err)
	}

	return buf.Bytes(), nil
}

// InjectEnv writes a generated .env file into stackPath and returns a cleanup
// function that removes it. If no .env.example exists, both return values are
// nil. The caller must call cleanup() after the stack files have been copied.
func InjectEnv(stackPath string, stackSecrets map[string]string) (func(), error) {
	content, err := GenerateEnv(stackPath, stackSecrets)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return func() {}, nil
	}

	envPath := filepath.Join(stackPath, ".env")
	if err := os.WriteFile(envPath, content, 0600); err != nil {
		return nil, fmt.Errorf("writing .env: %w", err)
	}

	cleanup := func() { os.Remove(envPath) }
	return cleanup, nil
}

// PromptPassword reads the master password from DOCKFLUX_SECRETS_PASSWORD env
// var, falling back to an interactive terminal prompt.
func PromptPassword(prompt string) (string, error) {
	if pass := os.Getenv("DOCKFLUX_SECRETS_PASSWORD"); pass != "" {
		return pass, nil
	}

	// Use huh masked input for interactive prompt
	var password string
	err := newPasswordForm(prompt, &password).Run()
	return password, err
}

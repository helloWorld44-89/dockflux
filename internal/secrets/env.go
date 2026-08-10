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
// example value). If no .env.example exists but stackSecrets is non-empty,
// all secrets are written directly to the output.
func GenerateEnv(stackPath string, stackSecrets map[string]string) ([]byte, error) {
	examplePath := filepath.Join(stackPath, ".env.example")
	f, err := os.Open(examplePath)
	if os.IsNotExist(err) {
		if len(stackSecrets) == 0 {
			return nil, nil
		}
		var buf bytes.Buffer
		for k, v := range stackSecrets {
			buf.WriteString(k + "=" + v + "\n")
		}
		return buf.Bytes(), nil
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

// ExampleKeys returns the variable names declared in stackPath/.env.example,
// in file order. Returns nil if no .env.example exists.
func ExampleKeys(stackPath string) ([]string, error) {
	examplePath := filepath.Join(stackPath, ".env.example")
	f, err := os.Open(examplePath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("opening .env.example: %w", err)
	}
	defer f.Close()

	var keys []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		keys = append(keys, strings.TrimSpace(parts[0]))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading .env.example: %w", err)
	}
	return keys, nil
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

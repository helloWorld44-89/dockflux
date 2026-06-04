package secrets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkmode_dev/dockflux/internal/secrets"
)

func TestGenerateEnv_WithExample(t *testing.T) {
	dir := t.TempDir()
	example := "DB_URL=postgres://localhost/db\nAPI_KEY=changeme\n# comment\nEMPTY=\n"
	if err := os.WriteFile(filepath.Join(dir, ".env.example"), []byte(example), 0644); err != nil {
		t.Fatal(err)
	}

	content, err := secrets.GenerateEnv(dir, map[string]string{
		"DB_URL": "postgres://prod/db",
	})
	if err != nil {
		t.Fatalf("GenerateEnv: %v", err)
	}

	result := string(content)
	if !strings.Contains(result, "DB_URL=postgres://prod/db") {
		t.Errorf("expected injected DB_URL; got:\n%s", result)
	}
	if !strings.Contains(result, "API_KEY=changeme") {
		t.Errorf("expected placeholder API_KEY preserved; got:\n%s", result)
	}
	if !strings.Contains(result, "# comment") {
		t.Errorf("expected comment line preserved; got:\n%s", result)
	}
}

func TestGenerateEnv_NoExample_WithSecrets(t *testing.T) {
	dir := t.TempDir()

	content, err := secrets.GenerateEnv(dir, map[string]string{"DB_URL": "postgres://prod/db"})
	if err != nil {
		t.Fatalf("GenerateEnv: %v", err)
	}
	if content == nil {
		t.Fatal("expected secrets written directly when no .env.example exists, got nil")
	}
	if !strings.Contains(string(content), "DB_URL=postgres://prod/db") {
		t.Errorf("expected DB_URL in output; got:\n%s", content)
	}
}

func TestGenerateEnv_NoExample_NoSecrets_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	content, err := secrets.GenerateEnv(dir, nil)
	if err != nil {
		t.Fatalf("GenerateEnv: %v", err)
	}
	if content != nil {
		t.Errorf("expected nil when no .env.example and no secrets; got %q", content)
	}
}

func TestGenerateEnv_NoExample_EmptySecrets_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	content, err := secrets.GenerateEnv(dir, map[string]string{})
	if err != nil {
		t.Fatalf("GenerateEnv: %v", err)
	}
	if content != nil {
		t.Errorf("expected nil for empty secrets map; got %q", content)
	}
}

func TestInjectEnv_WritesFileAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	example := "KEY=placeholder\n"
	if err := os.WriteFile(filepath.Join(dir, ".env.example"), []byte(example), 0644); err != nil {
		t.Fatal(err)
	}

	cleanup, err := secrets.InjectEnv(dir, map[string]string{"KEY": "injected"})
	if err != nil {
		t.Fatalf("InjectEnv: %v", err)
	}

	envPath := filepath.Join(dir, ".env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		t.Fatal("expected .env to exist after InjectEnv")
	}

	data, _ := os.ReadFile(envPath)
	if !strings.Contains(string(data), "KEY=injected") {
		t.Errorf("expected injected value in .env; got %q", data)
	}

	cleanup()

	if _, err := os.Stat(envPath); !os.IsNotExist(err) {
		t.Error("expected .env to be removed after cleanup()")
	}
}

func TestInjectEnv_NoExampleNoSecrets_NoFileWritten(t *testing.T) {
	dir := t.TempDir()
	cleanup, err := secrets.InjectEnv(dir, nil)
	if err != nil {
		t.Fatalf("InjectEnv: %v", err)
	}
	cleanup()
	if _, err := os.Stat(filepath.Join(dir, ".env")); !os.IsNotExist(err) {
		t.Error("expected no .env file when there is nothing to inject")
	}
}

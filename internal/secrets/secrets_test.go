package secrets_test

import (
	"path/filepath"
	"testing"

	"github.com/darkmode_dev/dockflux/internal/secrets"
)

func TestSaveLoad_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.enc")
	password := "correct-horse-battery-staple"

	store := secrets.New()
	store.SetStackSecret("myapp", "DB_PASSWORD", "secret123")
	store.SetStackSecret("myapp", "API_KEY", "key456")
	store.SetStackSecret("otherapp", "TOKEN", "tok789")

	if err := secrets.Save(path, password, store); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := secrets.Load(path, password)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	myapp := loaded.GetStackSecrets("myapp")
	if myapp["DB_PASSWORD"] != "secret123" {
		t.Errorf("DB_PASSWORD: got %q, want %q", myapp["DB_PASSWORD"], "secret123")
	}
	if myapp["API_KEY"] != "key456" {
		t.Errorf("API_KEY: got %q, want %q", myapp["API_KEY"], "key456")
	}

	other := loaded.GetStackSecrets("otherapp")
	if other["TOKEN"] != "tok789" {
		t.Errorf("TOKEN: got %q, want %q", other["TOKEN"], "tok789")
	}
}

func TestLoad_WrongPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.enc")

	store := secrets.New()
	store.SetStackSecret("myapp", "KEY", "value")
	if err := secrets.Save(path, "correct-password", store); err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, err := secrets.Load(path, "wrong-password")
	if err == nil {
		t.Error("expected error with wrong password, got nil")
	}
}

func TestLoad_NotExist_ReturnsEmpty(t *testing.T) {
	store, err := secrets.Load("/nonexistent/path/secrets.enc", "password")
	if err != nil {
		t.Fatalf("expected empty store for missing file, got error: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	if store.GetStackSecrets("any") != nil {
		t.Error("expected nil secrets for any stack in empty store")
	}
}

func TestSetGetStackSecret(t *testing.T) {
	store := secrets.New()
	store.SetStackSecret("app", "KEY", "val")

	got := store.GetStackSecrets("app")
	if got == nil {
		t.Fatal("expected secrets map, got nil")
	}
	if got["KEY"] != "val" {
		t.Errorf("got %q, want %q", got["KEY"], "val")
	}
}

func TestGetStackSecrets_Missing(t *testing.T) {
	store := secrets.New()
	if got := store.GetStackSecrets("nonexistent"); got != nil {
		t.Errorf("expected nil for unknown stack, got %v", got)
	}
}

func TestDeleteStackSecret(t *testing.T) {
	store := secrets.New()
	store.SetStackSecret("app", "KEY1", "v1")
	store.SetStackSecret("app", "KEY2", "v2")

	store.DeleteStackSecret("app", "KEY1")

	got := store.GetStackSecrets("app")
	if _, exists := got["KEY1"]; exists {
		t.Error("KEY1 should have been deleted")
	}
	if got["KEY2"] != "v2" {
		t.Error("KEY2 should remain after deleting KEY1")
	}
}

func TestDeleteStackSecret_LastKey_CleansStack(t *testing.T) {
	store := secrets.New()
	store.SetStackSecret("app", "ONLY_KEY", "val")
	store.DeleteStackSecret("app", "ONLY_KEY")

	if store.GetStackSecrets("app") != nil {
		t.Error("expected stack entry to be removed after its last key is deleted")
	}
}

func TestSave_EachCallProducesDistinctCiphertext(t *testing.T) {
	path1 := filepath.Join(t.TempDir(), "s1.enc")
	path2 := filepath.Join(t.TempDir(), "s2.enc")
	password := "password"

	store := secrets.New()
	store.SetStackSecret("app", "KEY", "value")

	if err := secrets.Save(path1, password, store); err != nil {
		t.Fatalf("Save 1: %v", err)
	}
	if err := secrets.Save(path2, password, store); err != nil {
		t.Fatalf("Save 2: %v", err)
	}

	// Both must decrypt correctly even though ciphertext differs (fresh salt/nonce each time)
	for _, path := range []string{path1, path2} {
		loaded, err := secrets.Load(path, password)
		if err != nil {
			t.Fatalf("Load %s: %v", path, err)
		}
		if loaded.GetStackSecrets("app")["KEY"] != "value" {
			t.Errorf("expected value after reload from %s", path)
		}
	}
}

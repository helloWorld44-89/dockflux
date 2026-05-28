package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/argon2"
	"gopkg.in/yaml.v3"
)

const (
	saltLen  = 32
	nonceLen = 12
	keyLen   = 32 // AES-256
)

// GitSecrets holds credentials for the stacks git repository.
type GitSecrets struct {
	HTTPSToken    string `yaml:"https_token,omitempty"`
	KeyPassphrase string `yaml:"key_passphrase,omitempty"`
}

// Store is the in-memory representation of the encrypted secrets file.
type Store struct {
	Git    GitSecrets                   `yaml:"git,omitempty"`
	Stacks map[string]map[string]string `yaml:"stacks,omitempty"` // stack → key → value
}

// DefaultPath returns ~/.dockflux/secrets.enc, falling back to ./.dockflux/secrets.enc
// if the home directory cannot be determined.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".dockflux", "secrets.enc")
	}
	return filepath.Join(home, ".dockflux", "secrets.enc")
}

// New returns an empty Store.
func New() *Store {
	return &Store{Stacks: make(map[string]map[string]string)}
}

// Load decrypts and parses the secrets file at path using password.
// Returns an empty store if the file does not exist yet.
func Load(path, password string) (*Store, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return New(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading secrets: %w", err)
	}

	if len(data) < saltLen+nonceLen {
		return nil, fmt.Errorf("secrets file is too short or corrupt")
	}

	salt := data[:saltLen]
	nonce := data[saltLen : saltLen+nonceLen]
	ciphertext := data[saltLen+nonceLen:]

	key := deriveKey(password, salt)
	plaintext, err := aesgcmDecrypt(key, nonce, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypting secrets (wrong password?): %w", err)
	}

	var store Store
	if err := yaml.Unmarshal(plaintext, &store); err != nil {
		return nil, fmt.Errorf("parsing secrets: %w", err)
	}
	if store.Stacks == nil {
		store.Stacks = make(map[string]map[string]string)
	}
	return &store, nil
}

// Save encrypts the store and writes it atomically to path.
func Save(path, password string, store *Store) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	plaintext, err := yaml.Marshal(store)
	if err != nil {
		return fmt.Errorf("marshalling secrets: %w", err)
	}

	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return err
	}

	key := deriveKey(password, salt)
	nonce, ciphertext, err := aesgcmEncrypt(key, plaintext)
	if err != nil {
		return err
	}

	// Layout: salt | nonce | ciphertext
	out := make([]byte, 0, saltLen+nonceLen+len(ciphertext))
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)

	// Atomic write
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".dockflux-secrets-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	tmp.Close()

	if err := os.Chmod(tmpPath, 0600); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, path)
}

// SetStackSecret sets a key/value pair for a stack.
func (s *Store) SetStackSecret(stack, key, value string) {
	if s.Stacks == nil {
		s.Stacks = make(map[string]map[string]string)
	}
	if s.Stacks[stack] == nil {
		s.Stacks[stack] = make(map[string]string)
	}
	s.Stacks[stack][key] = value
}

// GetStackSecrets returns all secrets for a stack (nil if none).
func (s *Store) GetStackSecrets(stack string) map[string]string {
	if s.Stacks == nil {
		return nil
	}
	return s.Stacks[stack]
}

// DeleteStackSecret removes a key from a stack's secrets.
func (s *Store) DeleteStackSecret(stack, key string) {
	if s.Stacks == nil {
		return
	}
	delete(s.Stacks[stack], key)
	if len(s.Stacks[stack]) == 0 {
		delete(s.Stacks, stack)
	}
}

func deriveKey(password string, salt []byte) []byte {
	// Argon2id: time=3, memory=64MB, threads=4
	return argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, keyLen)
}

func aesgcmEncrypt(key, plaintext []byte) (nonce, ciphertext []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return nonce, ciphertext, nil
}

func aesgcmDecrypt(key, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}

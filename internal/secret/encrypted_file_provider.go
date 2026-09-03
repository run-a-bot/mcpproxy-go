package secret

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const encryptedFileVersion = 1

type encryptedFileContents struct {
	Version int               `json:"version"`
	Entries map[string]string `json:"entries"`
}

// EncryptedFileProvider provides durable keyring semantics on headless systems
// where an operating-system keyring is unavailable. Ciphertext and its
// generated key are stored in separate mode-0600 files.
type EncryptedFileProvider struct {
	path    string
	keyPath string
	mutex   sync.Mutex
}

// NewEncryptedFileProvider creates a keyring-compatible encrypted file store.
func NewEncryptedFileProvider(path string) *EncryptedFileProvider {
	return &EncryptedFileProvider{path: path, keyPath: path + ".key"}
}

func (p *EncryptedFileProvider) CanResolve(secretType string) bool {
	return secretType == SecretTypeKeyring
}

func (p *EncryptedFileProvider) IsAvailable() bool {
	return p.path != ""
}

func (p *EncryptedFileProvider) Resolve(ctx context.Context, ref Ref) (string, error) {
	if !p.CanResolve(ref.Type) {
		return "", fmt.Errorf("encrypted file provider cannot resolve secret type: %s", ref.Type)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	key, contents, err := p.load()
	if err != nil {
		return "", err
	}
	ciphertext, found := contents.Entries[ref.Name]
	if !found {
		return "", fmt.Errorf("secret %q not found", ref.Name)
	}
	return decryptFileSecret(key, ref.Name, ciphertext)
}

func (p *EncryptedFileProvider) Store(ctx context.Context, ref Ref, value string) error {
	if !p.CanResolve(ref.Type) {
		return fmt.Errorf("encrypted file provider cannot store secret type: %s", ref.Type)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	key, contents, err := p.load()
	if err != nil {
		return err
	}
	ciphertext, err := encryptFileSecret(key, ref.Name, value)
	if err != nil {
		return err
	}
	contents.Entries[ref.Name] = ciphertext
	return p.save(contents)
}

func (p *EncryptedFileProvider) Delete(ctx context.Context, ref Ref) error {
	if !p.CanResolve(ref.Type) {
		return fmt.Errorf("encrypted file provider cannot delete secret type: %s", ref.Type)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	_, contents, err := p.load()
	if err != nil {
		return err
	}
	if _, found := contents.Entries[ref.Name]; !found {
		return fmt.Errorf("secret %q not found", ref.Name)
	}
	delete(contents.Entries, ref.Name)
	return p.save(contents)
}

func (p *EncryptedFileProvider) List(ctx context.Context) ([]Ref, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	_, contents, err := p.load()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(contents.Entries))
	for name := range contents.Entries {
		names = append(names, name)
	}
	sort.Strings(names)

	references := make([]Ref, 0, len(names))
	for _, name := range names {
		references = append(references, Ref{
			Type:     SecretTypeKeyring,
			Name:     name,
			Original: fmt.Sprintf("${keyring:%s}", name),
		})
	}
	return references, nil
}

func (p *EncryptedFileProvider) load() ([]byte, encryptedFileContents, error) {
	key, err := p.loadOrCreateKey()
	if err != nil {
		return nil, encryptedFileContents{}, err
	}

	contents := encryptedFileContents{Version: encryptedFileVersion, Entries: map[string]string{}}
	data, err := os.ReadFile(p.path)
	if errors.Is(err, os.ErrNotExist) {
		return key, contents, nil
	}
	if err != nil {
		return nil, encryptedFileContents{}, fmt.Errorf("read encrypted secret store: %w", err)
	}
	if err := enforceOwnerOnlyPermissions(p.path); err != nil {
		return nil, encryptedFileContents{}, err
	}
	if err := json.Unmarshal(data, &contents); err != nil {
		return nil, encryptedFileContents{}, fmt.Errorf("decode encrypted secret store: %w", err)
	}
	if contents.Version != encryptedFileVersion {
		return nil, encryptedFileContents{}, fmt.Errorf("unsupported encrypted secret store version %d", contents.Version)
	}
	if contents.Entries == nil {
		contents.Entries = map[string]string{}
	}
	return key, contents, nil
}

func (p *EncryptedFileProvider) loadOrCreateKey() ([]byte, error) {
	key, err := os.ReadFile(p.keyPath)
	if err == nil {
		if len(key) != 32 {
			return nil, fmt.Errorf("encrypted secret store key must contain 32 bytes")
		}
		if err := enforceOwnerOnlyPermissions(p.keyPath); err != nil {
			return nil, err
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read encrypted secret store key: %w", err)
	}
	if info, statErr := os.Stat(p.path); statErr == nil && info.Size() > 0 {
		return nil, fmt.Errorf("encrypted secret store key is missing for existing ciphertext")
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect encrypted secret store: %w", statErr)
	}
	if err := os.MkdirAll(filepath.Dir(p.keyPath), 0o700); err != nil {
		return nil, fmt.Errorf("create encrypted secret store directory: %w", err)
	}
	key = make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate encrypted secret store key: %w", err)
	}
	file, err := os.OpenFile(p.keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create encrypted secret store key: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("secure encrypted secret store key: %w", err)
	}
	if _, err := file.Write(key); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("write encrypted secret store key: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("sync encrypted secret store key: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close encrypted secret store key: %w", err)
	}
	return key, nil
}

func (p *EncryptedFileProvider) save(contents encryptedFileContents) error {
	directory := filepath.Dir(p.path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create encrypted secret store directory: %w", err)
	}
	file, err := os.CreateTemp(directory, ".mcpproxy-keyring-*")
	if err != nil {
		return fmt.Errorf("create encrypted secret store temporary file: %w", err)
	}
	temporaryPath := file.Name()
	defer os.Remove(temporaryPath)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("secure encrypted secret store temporary file: %w", err)
	}
	if err := json.NewEncoder(file).Encode(contents); err != nil {
		_ = file.Close()
		return fmt.Errorf("encode encrypted secret store: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync encrypted secret store: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close encrypted secret store: %w", err)
	}
	if err := os.Rename(temporaryPath, p.path); err != nil {
		return fmt.Errorf("replace encrypted secret store: %w", err)
	}
	return enforceOwnerOnlyPermissions(p.path)
}

func enforceOwnerOnlyPermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect encrypted secret store file: %w", err)
	}
	if info.Mode().Perm()&0o077 == 0 {
		return nil
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("repair encrypted secret store permissions: %w", err)
	}
	info, err = os.Stat(path)
	if err != nil {
		return fmt.Errorf("verify encrypted secret store permissions: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("encrypted secret store permissions must not grant group or other access")
	}
	return nil
}

func encryptFileSecret(key []byte, name, value string) (string, error) {
	aead, err := newFileSecretAEAD(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate secret nonce: %w", err)
	}
	sealed := aead.Seal(nonce, nonce, []byte(value), []byte(name))
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func decryptFileSecret(key []byte, name, encoded string) (string, error) {
	aead, err := newFileSecretAEAD(key)
	if err != nil {
		return "", err
	}
	sealed, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode encrypted secret %q: %w", name, err)
	}
	if len(sealed) < aead.NonceSize() {
		return "", fmt.Errorf("encrypted secret %q is truncated", name)
	}
	nonce, ciphertext := sealed[:aead.NonceSize()], sealed[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, ciphertext, []byte(name))
	if err != nil {
		return "", fmt.Errorf("decrypt secret %q: %w", name, err)
	}
	return string(plaintext), nil
}

func newFileSecretAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("initialize secret cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("initialize secret encryption: %w", err)
	}
	return aead, nil
}

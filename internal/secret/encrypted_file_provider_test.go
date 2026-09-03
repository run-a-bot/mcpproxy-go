package secret

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncryptedFileProviderPersistsWithoutPlaintext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keyring.json")
	provider := NewEncryptedFileProvider(path)
	reference := Ref{Type: SecretTypeKeyring, Name: "github-token"}
	const value = "plain-secret-value"

	if err := provider.Store(context.Background(), reference, value); err != nil {
		t.Fatalf("store secret: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read encrypted file: %v", err)
	}
	if strings.Contains(string(data), value) {
		t.Fatal("encrypted secret store contains plaintext value")
	}

	freshProvider := NewEncryptedFileProvider(path)
	resolved, err := freshProvider.Resolve(context.Background(), reference)
	if err != nil {
		t.Fatalf("resolve persisted secret: %v", err)
	}
	if resolved != value {
		t.Fatalf("resolved value = %q, want %q", resolved, value)
	}

	references, err := freshProvider.List(context.Background())
	if err != nil {
		t.Fatalf("list secrets: %v", err)
	}
	if len(references) != 1 || references[0].Name != reference.Name || references[0].Original != "${keyring:github-token}" {
		t.Fatalf("unexpected references: %#v", references)
	}

	if err := freshProvider.Delete(context.Background(), reference); err != nil {
		t.Fatalf("delete secret: %v", err)
	}
	if _, err := freshProvider.Resolve(context.Background(), reference); err == nil {
		t.Fatal("deleted secret still resolves")
	}

	for _, filename := range []string{path, path + ".key"} {
		info, err := os.Stat(filename)
		if err != nil {
			t.Fatalf("stat %s: %v", filename, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("permissions for %s = %o, want 600", filename, info.Mode().Perm())
		}
	}
}

func TestNewResolverUsesConfiguredEncryptedFileProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keyring.json")
	t.Setenv("MCPPROXY_KEYRING_FILE", path)
	reference := Ref{Type: SecretTypeKeyring, Name: "headless-token"}

	resolver := NewResolver()
	if err := resolver.Store(context.Background(), reference, "value"); err != nil {
		t.Fatalf("store through configured resolver: %v", err)
	}
	resolved, err := NewResolver().Resolve(context.Background(), reference)
	if err != nil {
		t.Fatalf("resolve through fresh resolver: %v", err)
	}
	if resolved != "value" {
		t.Fatalf("resolved value = %q, want value", resolved)
	}
}

func TestEncryptedFileProviderRepairsPermissionsAfterVolumeRemount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keyring.json")
	provider := NewEncryptedFileProvider(path)
	reference := Ref{Type: SecretTypeKeyring, Name: "remounted-token"}
	if err := provider.Store(context.Background(), reference, "value"); err != nil {
		t.Fatalf("store secret: %v", err)
	}
	for _, filename := range []string{path, path + ".key"} {
		if err := os.Chmod(filename, 0o660); err != nil {
			t.Fatalf("simulate remount permissions for %s: %v", filename, err)
		}
	}

	resolved, err := NewEncryptedFileProvider(path).Resolve(context.Background(), reference)
	if err != nil {
		t.Fatalf("resolve after remount: %v", err)
	}
	if resolved != "value" {
		t.Fatalf("resolved value = %q, want value", resolved)
	}
	for _, filename := range []string{path, path + ".key"} {
		info, err := os.Stat(filename)
		if err != nil {
			t.Fatalf("stat %s: %v", filename, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("permissions for %s = %o, want repaired 600", filename, info.Mode().Perm())
		}
	}
}

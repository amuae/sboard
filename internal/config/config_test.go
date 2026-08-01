package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreatePersistsJWTSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")

	first, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate() error = %v", err)
	}
	if first.Security.JWTSecret == "" {
		t.Fatal("generated JWT secret is empty")
	}

	second, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("second LoadOrCreate() error = %v", err)
	}
	if second.Security.JWTSecret != first.Security.JWTSecret {
		t.Fatal("JWT secret changed after reloading the config")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("config permissions = %o, want 600", got)
	}
}

func TestLoadOrCreatePersistsMissingJWTSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	input := []byte("server:\n  listen: 127.0.0.1:9000\nsecurity:\n  jwt_expire_hour: 24\n")
	if err := os.WriteFile(path, input, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	first, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate() error = %v", err)
	}
	second, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("second LoadOrCreate() error = %v", err)
	}
	if first.Security.JWTSecret == "" || second.Security.JWTSecret != first.Security.JWTSecret {
		t.Fatal("missing JWT secret was not persisted")
	}
	if second.Server.Listen != "127.0.0.1:9000" {
		t.Fatalf("listen = %q, want 127.0.0.1:9000", second.Server.Listen)
	}
}

func TestLoadOrCreateRejectsInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("security: [invalid"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadOrCreate(path); err == nil {
		t.Fatal("LoadOrCreate() accepted invalid YAML")
	}
}

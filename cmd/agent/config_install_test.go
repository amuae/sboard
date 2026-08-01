package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallValidatedConfigPreservesMetadataAndCanRollback(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(configPath, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(configPath, 0640); err != nil {
		t.Fatal(err)
	}
	originalInfo, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}

	var stagedPath string
	installed, err := installValidatedConfig(configPath, []byte("new"), func(path string) error {
		stagedPath = path
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if string(content) != "new" {
			t.Fatalf("staged content = %q, want new", content)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if stagedPath == "" {
		t.Fatal("validator was not called")
	}

	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(mustReadFile(t, configPath)) != "new" {
		t.Fatalf("installed content = %q, want new", mustReadFile(t, configPath))
	}
	if info.Mode().Perm() != originalInfo.Mode().Perm() {
		t.Fatalf("installed mode = %o, want %o", info.Mode().Perm(), originalInfo.Mode().Perm())
	}
	if installed.backupPath == "" {
		t.Fatal("expected backup path")
	}

	if err := rollbackInstalledConfig(installed); err != nil {
		t.Fatal(err)
	}
	if got := string(mustReadFile(t, configPath)); got != "old" {
		t.Fatalf("rolled back content = %q, want old", got)
	}
}

func TestInstallValidatedConfigRejectsBeforeReplacing(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(configPath, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := installValidatedConfig(configPath, []byte("bad"), func(string) error {
		return errors.New("invalid config")
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if got := string(mustReadFile(t, configPath)); got != "old" {
		t.Fatalf("config after rejected update = %q, want old", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("temporary files left after rejection: %d", len(entries))
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyFileChecksum(t *testing.T) {
	dir := t.TempDir()
	archiveName := "sboard-agent_linux_amd64.zip"
	archivePath := filepath.Join(dir, archiveName)
	content := []byte("release archive")
	if err := os.WriteFile(archivePath, content, 0600); err != nil {
		t.Fatalf("WriteFile(archive) error = %v", err)
	}

	sum := sha256.Sum256(content)
	checksumsPath := filepath.Join(dir, "checksums.txt")
	checksums := fmt.Sprintf("%x  other.zip\n%x  %s\n", sha256.Sum256([]byte("other")), sum, archiveName)
	if err := os.WriteFile(checksumsPath, []byte(checksums), 0600); err != nil {
		t.Fatalf("WriteFile(checksums) error = %v", err)
	}

	if err := verifyFileChecksum(archivePath, checksumsPath, archiveName); err != nil {
		t.Fatalf("verifyFileChecksum() error = %v", err)
	}
}

func TestVerifyFileChecksumRejectsMismatch(t *testing.T) {
	dir := t.TempDir()
	archiveName := "sboard-agent_linux_amd64.zip"
	archivePath := filepath.Join(dir, archiveName)
	if err := os.WriteFile(archivePath, []byte("tampered"), 0600); err != nil {
		t.Fatalf("WriteFile(archive) error = %v", err)
	}

	checksumsPath := filepath.Join(dir, "checksums.txt")
	expected := sha256.Sum256([]byte("original"))
	if err := os.WriteFile(checksumsPath, []byte(fmt.Sprintf("%x *%s\n", expected, archiveName)), 0600); err != nil {
		t.Fatalf("WriteFile(checksums) error = %v", err)
	}

	if err := verifyFileChecksum(archivePath, checksumsPath, archiveName); err == nil {
		t.Fatal("verifyFileChecksum() accepted a mismatched archive")
	}
}

func TestVerifyFileChecksumRejectsMissingEntry(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "agent.zip")
	checksumsPath := filepath.Join(dir, "checksums.txt")
	if err := os.WriteFile(archivePath, []byte("archive"), 0600); err != nil {
		t.Fatalf("WriteFile(archive) error = %v", err)
	}
	if err := os.WriteFile(checksumsPath, []byte("bad entry\n"), 0600); err != nil {
		t.Fatalf("WriteFile(checksums) error = %v", err)
	}

	if err := verifyFileChecksum(archivePath, checksumsPath, "agent.zip"); err == nil {
		t.Fatal("verifyFileChecksum() accepted a missing checksum entry")
	}
}

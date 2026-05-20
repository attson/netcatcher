package updater

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupStaleRemovesStagingDirs(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(dir, "updates", "1.4.0")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staging, "leftover"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := CleanupStale(dir); err != nil {
		t.Fatalf("CleanupStale: %v", err)
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("expected staging dir gone, got %v", err)
	}
}

func TestCleanupStaleHandlesMissingDir(t *testing.T) {
	if err := CleanupStale(t.TempDir()); err != nil {
		t.Fatalf("expected no error for empty configDir, got %v", err)
	}
}

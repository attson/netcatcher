package updater

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// CleanupStale removes leftover staging dirs from a previous unfinished
// install. Safe to call on every startup. Does not touch <exe>.old —
// the Windows installer overwrites it on the next install, and removing
// it here would race with a still-running helper.
func CleanupStale(configDir string) error {
	root := stagingDir(configDir)
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read staging dir: %w", err)
	}
	var firstErr error
	for _, e := range entries {
		path := filepath.Join(root, e.Name())
		if err := os.RemoveAll(path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

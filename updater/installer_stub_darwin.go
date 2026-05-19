//go:build darwin

package updater

import (
	"context"
	"errors"
	"log/slog"
)

// installWindows is a stub on darwin so updater.go can reference the
// symbol regardless of build target. The real implementation lives in
// installer_windows.go.
func installWindows(_ context.Context, _ string, _ *slog.Logger) error {
	return errors.New("windows installer not available on darwin builds")
}

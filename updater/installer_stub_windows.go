//go:build windows

package updater

import (
	"context"
	"errors"
	"log/slog"
)

// installDarwin is a stub on windows so updater.go can reference the
// symbol regardless of build target. The real implementation lives in
// installer_darwin.go.
func installDarwin(_ context.Context, _ string, _ *slog.Logger) error {
	return errors.New("darwin installer not available on windows builds")
}

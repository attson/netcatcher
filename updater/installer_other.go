//go:build !darwin && !windows

package updater

import (
	"context"
	"errors"
	"log/slog"
)

func installDarwin(_ context.Context, _ string, _ *slog.Logger) error {
	return errors.New("darwin installer not available on this build")
}

func installWindows(_ context.Context, _ string, _ *slog.Logger) error {
	return errors.New("windows installer not available on this build")
}

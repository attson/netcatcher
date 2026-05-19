//go:build darwin

package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"netcatcher/updater/scripts"
)

// installDarwin extracts the .app.tar.gz into a sibling staging dir,
// writes the install-darwin.sh helper, launches it detached, and returns.
// The caller must invoke wailsApp.Quit() afterwards so the helper can
// swap the bundle.
func installDarwin(ctx context.Context, assetPath string, logger *slog.Logger) error {
	currentApp, err := resolveCurrentAppBundle()
	if err != nil {
		return err
	}

	stagingRoot := filepath.Dir(assetPath)
	stagedApp := filepath.Join(stagingRoot, "NetCatcher.app")
	_ = os.RemoveAll(stagedApp)

	if err := extractTarGz(assetPath, stagingRoot); err != nil {
		return fmt.Errorf("extract %s: %w", assetPath, err)
	}
	if _, err := os.Stat(stagedApp); err != nil {
		return fmt.Errorf("expected NetCatcher.app inside %s: %w", assetPath, err)
	}

	scriptPath := filepath.Join(stagingRoot, "install.sh")
	if err := os.WriteFile(scriptPath, scripts.InstallDarwin, 0o755); err != nil {
		return fmt.Errorf("write helper: %w", err)
	}

	pid := strconv.Itoa(os.Getpid())
	cmd := exec.Command("/bin/bash", scriptPath, pid, stagedApp, currentApp)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	// Detach so the script outlives the parent process.
	cmd.SysProcAttr = detachAttrs()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch helper: %w", err)
	}
	// Release; we deliberately do not Wait().
	_ = cmd.Process.Release()
	logger.Info("updater: helper script launched", "pid", cmd.Process.Pid, "currentApp", currentApp)
	return nil
}

// resolveCurrentAppBundle walks up from the running binary to the .app.
// Returns an error if the binary is not inside a .app bundle (dev runs).
func resolveCurrentAppBundle() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("os.Executable: %w", err)
	}
	// .../NetCatcher.app/Contents/MacOS/NetCatcher
	app := filepath.Dir(filepath.Dir(filepath.Dir(exe)))
	if !strings.HasSuffix(app, ".app") {
		return "", errors.New("not running from a .app bundle (dev build); refusing to install")
	}
	return app, nil
}

func extractTarGz(archive, destRoot string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(destRoot, hdr.Name)
		if !strings.HasPrefix(target, filepath.Clean(destRoot)+string(os.PathSeparator)) {
			return fmt.Errorf("tar path escape: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		}
	}
	return nil
}

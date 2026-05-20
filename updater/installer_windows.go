//go:build windows

package updater

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"netcatcher/updater/scripts"
)

func installWindows(ctx context.Context, assetPath string, logger *slog.Logger) error {
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("os.Executable: %w", err)
	}

	stagingRoot := filepath.Dir(assetPath)
	stagedExe := filepath.Join(stagingRoot, "NetCatcher-new.exe")
	if err := copyFile(assetPath, stagedExe); err != nil {
		return fmt.Errorf("stage exe: %w", err)
	}

	scriptPath := filepath.Join(stagingRoot, "install.ps1")
	if err := os.WriteFile(scriptPath, scripts.InstallWindows, 0o644); err != nil {
		return fmt.Errorf("write helper: %w", err)
	}

	pid := strconv.Itoa(os.Getpid())
	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile", "-ExecutionPolicy", "Bypass",
		"-File", scriptPath,
		"-ParentPid", pid,
		"-StagedExe", stagedExe,
		"-TargetExe", currentExe,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP — survive parent exit
		// and don't share its console (we run with -H=windowsgui so there's
		// no console to share anyway).
		CreationFlags: 0x00000008 | 0x00000200,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch helper: %w", err)
	}
	_ = cmd.Process.Release()
	logger.Info("updater: helper script launched", "pid", cmd.Process.Pid, "currentExe", currentExe)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

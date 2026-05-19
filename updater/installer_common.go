package updater

import (
	"path/filepath"
	"runtime"
)

// stagingDir returns the directory where downloads and staged installs
// live. configDir is the same path Config uses, so the updater stays in
// the user's app-support directory rather than scattering temp files.
func stagingDir(configDir string) string {
	return filepath.Join(configDir, "updates")
}

// downloadPath builds the on-disk path for a release asset. The .partial
// suffix is added during download and stripped after verification.
func downloadPath(configDir, version, assetName string) string {
	return filepath.Join(stagingDir(configDir), version, assetName)
}

// CurrentPlatform returns goos/goarch for use in tests and runtime calls.
// Exposed so the orchestrator can pass the same values to GitHub asset
// selection and installer dispatch.
func CurrentPlatform() (goos, goarch string) {
	return runtime.GOOS, runtime.GOARCH
}

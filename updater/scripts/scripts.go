// Package scripts exposes the platform helper scripts as embedded bytes.
// They are written to a temp directory at install time and launched
// detached from the main process.
package scripts

import _ "embed"

//go:embed install-darwin.sh
var InstallDarwin []byte

//go:embed install-windows.ps1
var InstallWindows []byte

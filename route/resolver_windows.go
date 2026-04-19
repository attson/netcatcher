//go:build windows

package route

// Per-domain DNS resolver redirection is macOS-only for now. On Windows the
// equivalent would be NRPT (Name Resolution Policy Table) via PowerShell, which
// we have not implemented yet. These stubs let the cross-platform code compile
// without needing build-tag gymnastics in callers.

func InstallResolverEntries(domains []string, port int) error { return nil }
func RemoveResolverEntries(domains []string) error            { return nil }

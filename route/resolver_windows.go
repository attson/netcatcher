//go:build windows

package route

import (
	"fmt"
	"os/exec"
	"strings"

	"netcatcher/llog"
)

// nrptComment tags every NRPT rule NetCatcher creates so cleanup can target
// them precisely even if the process previously crashed and left rules
// behind.
const nrptComment = "netcatcher"

// InstallResolverEntries adds a Name Resolution Policy Table rule per
// domain, routing those queries to 127.0.0.1. The `port` argument is
// ignored — NRPT has no port concept, so the forwarder must bind the
// well-known DNS port (53) on Windows.
//
// To keep reconfiguration simple we first wipe all rules tagged with
// `nrptComment` (handles stale entries from a previous crashed session),
// then add the current set.
func InstallResolverEntries(domains []string, port int) error {
	clean := filterDomains(domains)
	if len(clean) == 0 {
		return nil
	}
	_ = RemoveResolverEntries(nil) // clear stale rules; ignore any failure

	for _, d := range clean {
		ns := "." + d // leading dot = suffix match (covers the domain + subdomains)
		script := fmt.Sprintf(
			`Add-DnsClientNrptRule -Namespace '%s' -NameServers '127.0.0.1' -Comment '%s' -ErrorAction Stop`,
			ns, nrptComment,
		)
		if err := runPowerShell(script); err != nil {
			llog.Warnf("route", "add NRPT rule for %s failed: %v", ns, err)
		}
	}
	if err := FlushDNSCache(); err != nil {
		llog.Warnf("route", "refresh DNS cache after installing NRPT rules failed: %v", err)
	}
	return nil
}

// RemoveResolverEntries removes every NRPT rule NetCatcher installed
// (identified by Comment), regardless of the passed-in domains. The
// argument is kept for interface parity with the macOS implementation.
func RemoveResolverEntries(domains []string) error {
	script := fmt.Sprintf(
		`Get-DnsClientNrptRule | Where-Object { $_.Comment -eq '%s' } | Remove-DnsClientNrptRule -Force -ErrorAction Stop`,
		nrptComment,
	)
	if err := runPowerShell(script); err != nil {
		// If nothing matched PowerShell returns success; any other error is worth surfacing.
		llog.Warnf("route", "remove NRPT rules failed: %v", err)
		return err
	}
	if err := FlushDNSCache(); err != nil {
		llog.Warnf("route", "refresh DNS cache after removing NRPT rules failed: %v", err)
	}
	return nil
}

// FlushDNSCache clears the Windows DNS client resolver cache. Browser-private
// DNS and connection pools are outside this cache.
func FlushDNSCache() error {
	cmd := exec.Command("ipconfig.exe", "/flushdns")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runPowerShell(script string) error {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func filterDomains(domains []string) []string {
	out := make([]string, 0, len(domains))
	for _, d := range domains {
		d = strings.TrimSpace(strings.ToLower(strings.Trim(d, ".")))
		if d == "" {
			continue
		}
		ok := true
		for _, r := range d {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.') {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, d)
		}
	}
	return out
}

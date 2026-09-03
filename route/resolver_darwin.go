//go:build darwin

package route

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// InstallResolverEntries writes /etc/resolver/<domain> files pointing at
// 127.0.0.1:port for each domain. Uses the privileged helper installed during
// the first-run admin authorization, so no password prompt is shown on
// subsequent calls.
func InstallResolverEntries(domains []string, port int) error {
	clean := filterDomains(domains)
	if len(clean) == 0 {
		return nil
	}
	ensureAuth()
	args := append([]string{helperPath, "install", fmt.Sprintf("%d", port)}, clean...)
	return runHelper(args)
}

// RemoveResolverEntries deletes the /etc/resolver/<domain> files created by
// InstallResolverEntries. Missing files are ignored.
func RemoveResolverEntries(domains []string) error {
	clean := filterDomains(domains)
	if len(clean) == 0 {
		return nil
	}
	ensureAuth()
	args := append([]string{helperPath, "remove"}, clean...)
	return runHelper(args)
}

// FlushDNSCache clears macOS's system resolver cache and notifies
// mDNSResponder. Browser-private DNS and connection pools are outside the
// system resolver and may still need to be refreshed separately.
func FlushDNSCache() error {
	ensureAuth()
	return runHelper([]string{helperPath, "flush"})
}

func runHelper(args []string) error {
	var cmd *exec.Cmd
	if isRoot() {
		cmd = exec.Command(args[0], args[1:]...)
	} else {
		cmd = exec.Command("sudo", append([]string{"-n"}, args...)...)
	}
	cmd.Stderr = log.Writer()
	cmd.Stdout = log.Writer()
	return cmd.Run()
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

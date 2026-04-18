//go:build darwin

package main

import (
	"os/exec"
	"regexp"
	"strings"
)

func getVPNServiceNames() map[string]string {
	result := make(map[string]string)

	out, err := exec.Command("scutil", "--nc", "list").Output()
	if err != nil {
		return result
	}

	reEntry := regexp.MustCompile(`\(([^)]+)\)\s+(\S+)\s+"([^"]+)"`)
	for _, line := range strings.Split(string(out), "\n") {
		matches := reEntry.FindStringSubmatch(line)
		if len(matches) < 4 {
			continue
		}
		uuid := matches[2]
		name := matches[3]

		ifaceOut, err := exec.Command("sh", "-c",
			`echo "show State:/Network/Service/`+uuid+`/IPv4" | scutil | grep InterfaceName | awk '{print $3}'`).Output()
		if err != nil {
			continue
		}
		ifaceName := strings.TrimSpace(string(ifaceOut))
		if ifaceName != "" {
			result[ifaceName] = name
		}
	}
	return result
}

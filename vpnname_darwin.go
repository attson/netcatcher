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

	reUUID := regexp.MustCompile(`([0-9A-Fa-f-]{36})`)
	reName := regexp.MustCompile(`"([^"]+)"`)
	for _, line := range strings.Split(string(out), "\n") {
		uuidMatch := reUUID.FindStringSubmatch(line)
		nameMatch := reName.FindStringSubmatch(line)
		if len(uuidMatch) < 2 || len(nameMatch) < 2 {
			continue
		}
		uuid := uuidMatch[1]
		name := nameMatch[1]

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

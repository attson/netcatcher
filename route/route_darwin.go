package route

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"sync"
)

const sudoersFile = "/etc/sudoers.d/netcatcher"

var authOnce sync.Once

func isRoot() bool {
	return os.Geteuid() == 0
}

func ensureAuth() {
	if isRoot() {
		return
	}
	authOnce.Do(func() {
		u, err := user.Current()
		if err != nil {
			log.Printf("[warn] get current user failed: %v", err)
			return
		}
		rule := fmt.Sprintf("%s ALL=(ALL) NOPASSWD: /sbin/route", u.Username)
		cmd := fmt.Sprintf(`do shell script "echo '%s' > %s && chmod 0440 %s" with administrator privileges`, rule, sudoersFile, sudoersFile)
		command := exec.Command("osascript", "-e", cmd)
		command.Stderr = log.Writer()
		command.Stdout = log.Writer()
		if err := command.Run(); err != nil {
			log.Printf("[error] admin authorization failed: %v", err)
		} else {
			log.Printf("[info] admin authorization granted")
		}
	})
}

func Cleanup() {
}

func runBatch(cmds []string) error {
	if len(cmds) == 0 {
		return nil
	}
	ensureAuth()
	joined := strings.Join(cmds, " ; ")
	var command *exec.Cmd
	if isRoot() {
		command = exec.Command("sh", "-c", joined)
	} else {
		command = exec.Command("sudo", "sh", "-c", joined)
	}
	command.Stderr = log.Writer()
	command.Stdout = log.Writer()
	return command.Run()
}

func addCmd(ip, gateway string, mask net.IPMask) string {
	if mask != nil {
		return fmt.Sprintf("route add -net %s %s", ip, gateway)
	}
	return fmt.Sprintf("route add -host %s %s", ip, gateway)
}

func deleteCmd(ip, gateway string, mask net.IPMask) string {
	if mask != nil {
		return fmt.Sprintf("route delete -net %s %s", ip, gateway)
	}
	return fmt.Sprintf("route delete -host %s %s", ip, gateway)
}

func AddRoute(ip, gateway string, mask net.IPMask) error {
	return runBatch([]string{addCmd(ip, gateway, mask)})
}

func DeleteRoute(ip, gateway string, mask net.IPMask) error {
	return runBatch([]string{deleteCmd(ip, gateway, mask)})
}

type RouteSpec struct {
	Ip      string
	Gateway string
	Mask    net.IPMask
}

func AddRoutes(routes []RouteSpec) error {
	cmds := make([]string, len(routes))
	for i, r := range routes {
		cmds[i] = addCmd(r.Ip, r.Gateway, r.Mask)
	}
	return runBatch(cmds)
}

func DeleteRoutes(routes []RouteSpec) error {
	cmds := make([]string, len(routes))
	for i, r := range routes {
		cmds[i] = deleteCmd(r.Ip, r.Gateway, r.Mask)
	}
	return runBatch(cmds)
}

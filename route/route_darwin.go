package route

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
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

func runRoute(args ...string) error {
	ensureAuth()
	var command *exec.Cmd
	if isRoot() {
		command = exec.Command("/sbin/route", args...)
	} else {
		command = exec.Command("sudo", append([]string{"/sbin/route"}, args...)...)
	}
	command.Stderr = log.Writer()
	command.Stdout = log.Writer()
	return command.Run()
}

func AddRoute(ip, gateway string, mask net.IPMask) error {
	if mask != nil {
		return runRoute("add", "-net", ip, gateway)
	}
	return runRoute("add", "-host", ip, gateway)
}

func DeleteRoute(ip, gateway string, mask net.IPMask) error {
	if mask != nil {
		return runRoute("delete", "-net", ip, gateway)
	}
	return runRoute("delete", "-host", ip, gateway)
}

type RouteSpec struct {
	Ip      string
	Gateway string
	Mask    net.IPMask
}

func AddRoutes(routes []RouteSpec) error {
	for _, r := range routes {
		if err := AddRoute(r.Ip, r.Gateway, r.Mask); err != nil {
			log.Printf("[warn] add route %s failed: %v", r.Ip, err)
		}
	}
	return nil
}

func DeleteRoutes(routes []RouteSpec) error {
	for _, r := range routes {
		if err := DeleteRoute(r.Ip, r.Gateway, r.Mask); err != nil {
			log.Printf("[warn] delete route %s failed: %v", r.Ip, err)
		}
	}
	return nil
}

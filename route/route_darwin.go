package route

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
)

func isRoot() bool {
	return os.Geteuid() == 0
}

func runBatch(cmds []string) error {
	if len(cmds) == 0 {
		return nil
	}
	joined := strings.Join(cmds, " ; ")
	if isRoot() {
		command := exec.Command("sh", "-c", joined)
		command.Stderr = log.Writer()
		command.Stdout = log.Writer()
		return command.Run()
	}
	script := fmt.Sprintf(`do shell script "%s" with administrator privileges`, joined)
	command := exec.Command("osascript", "-e", script)
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

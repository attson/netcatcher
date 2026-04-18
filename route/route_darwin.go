package route

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
)

func isRoot() bool {
	return os.Geteuid() == 0
}

func runRouteCmd(args string) error {
	if isRoot() {
		command := exec.Command("sh", "-c", args)
		command.Stderr = log.Writer()
		command.Stdout = log.Writer()
		return command.Run()
	}
	script := fmt.Sprintf(`do shell script "%s" with administrator privileges`, args)
	command := exec.Command("osascript", "-e", script)
	command.Stderr = log.Writer()
	command.Stdout = log.Writer()
	return command.Run()
}

func AddRoute(ip, gateway string, mask net.IPMask) error {
	var args string
	if mask != nil {
		args = fmt.Sprintf("route add -net %s %s", ip, gateway)
	} else {
		args = fmt.Sprintf("route add -host %s %s", ip, gateway)
	}
	return runRouteCmd(args)
}

func DeleteRoute(ip, gateway string, mask net.IPMask) error {
	var args string
	if mask != nil {
		args = fmt.Sprintf("route delete -net %s %s", ip, gateway)
	} else {
		args = fmt.Sprintf("route delete -host %s %s", ip, gateway)
	}
	return runRouteCmd(args)
}

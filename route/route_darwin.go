package route

import (
	"log"
	"net"
	"os/exec"
)

func AddRoute(ip, gateway string, mask net.IPMask) error {
	var command *exec.Cmd

	if mask != nil {
		command = exec.Command("route", "add", "-net", ip, gateway)
	} else {
		command = exec.Command("route", "add", "-host", ip, gateway)
	}

	command.Stderr = log.Writer()
	command.Stdout = log.Writer()

	return command.Run()
}

func DeleteRoute(ip, gateway string, mask net.IPMask) error {
	var command *exec.Cmd

	if mask != nil {
		command = exec.Command("route", "delete", "-net", ip, gateway)
	} else {
		command = exec.Command("route", "delete", "-host", ip, gateway)
	}

	command.Stderr = log.Writer()
	command.Stdout = log.Writer()

	return command.Run()
}

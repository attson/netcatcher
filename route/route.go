package route

import (
	"os"
	"os/exec"
)

func AddRoute(ip, gateway string, mask bool) error {
	var command *exec.Cmd

	if mask {
		command = exec.Command("route", "add", "-net", ip, gateway)
	} else {
		command = exec.Command("route", "add", "-host", ip, gateway)
	}

	command.Stderr = os.Stderr
	command.Stdout = os.Stdout

	return command.Run()
}

func DeleteRoute(ip, gateway string, mask bool) error {
	var command *exec.Cmd

	if mask {
		command = exec.Command("route", "delete", "-net", ip, gateway)
	} else {
		command = exec.Command("route", "delete", "-host", ip, gateway)
	}

	command.Stderr = os.Stderr
	command.Stdout = os.Stdout

	return command.Run()
}

package main

import (
	"fmt"

	nc "netcatcher/netcatcher"

	"github.com/gen2brain/beeep"
)

type Notifier struct {
	enabled bool
}

func NewNotifier() *Notifier {
	return &Notifier{enabled: true}
}

func (n *Notifier) SetEnabled(enabled bool) {
	n.enabled = enabled
}

func (n *Notifier) IsEnabled() bool {
	return n.enabled
}

func (n *Notifier) OnStatusChange(status nc.InterfaceStatus) {
	if !n.enabled {
		return
	}

	var title, message string
	if status.Connected {
		title = "Interface Connected"
		message = fmt.Sprintf("%s is now online (gateway: %s)", status.InterfaceName, status.Gateway)
	} else {
		title = "Interface Disconnected"
		message = fmt.Sprintf("%s is now offline", status.InterfaceName)
	}

	_ = beeep.Notify(title, message, "")
}

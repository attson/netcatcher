package main

import (
	"fmt"
	"sync"
	"time"

	"netcatcher/llog"
	nc "netcatcher/netcatcher"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

type Notifier struct {
	enabled   bool
	svc       *notifications.NotificationService
	mu        sync.Mutex
	lastState map[string]bool
}

func NewNotifier(svc *notifications.NotificationService) *Notifier {
	return &Notifier{enabled: true, svc: svc, lastState: make(map[string]bool)}
}

func (n *Notifier) SetEnabled(enabled bool) {
	n.enabled = enabled
}

func (n *Notifier) IsEnabled() bool {
	return n.enabled
}

func (n *Notifier) OnStatusChange(status nc.InterfaceStatus) {
	n.mu.Lock()
	prev, seen := n.lastState[status.InterfaceName]
	n.lastState[status.InterfaceName] = status.Connected
	n.mu.Unlock()
	if !seen || prev == status.Connected {
		return
	}

	if !n.enabled || n.svc == nil {
		return
	}

	var title, body string
	if status.Connected {
		title = "Interface Connected"
		body = fmt.Sprintf("%s is now online (gateway: %s)", status.InterfaceName, status.Gateway)
	} else {
		title = "Interface Disconnected"
		body = fmt.Sprintf("%s is now offline", status.InterfaceName)
	}

	id := fmt.Sprintf("netcatcher-%s-%d", status.InterfaceName, time.Now().UnixNano())
	if err := n.svc.SendNotification(notifications.NotificationOptions{
		ID:    id,
		Title: title,
		Body:  body,
	}); err != nil {
		llog.Warnf("notify", "send notification failed: %v", err)
	}
}

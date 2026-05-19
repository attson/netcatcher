//go:build darwin

package updater

import "syscall"

func detachAttrs() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

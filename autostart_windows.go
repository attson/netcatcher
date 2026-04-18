//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

const regKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const regValue = "NetCatcher"

func checkAutoStart() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(regValue)
	return err == nil
}

func setAutoStart(enabled bool) error {
	if !enabled {
		k, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.SET_VALUE)
		if err != nil {
			return nil
		}
		defer k.Close()
		_ = k.DeleteValue(regValue)
		return nil
	}

	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(regValue, execPath)
}

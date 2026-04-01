//go:build windows

package app

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const windowsRunKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
const windowsRunValueName = "BananasPOS"

func autoStartSupported() bool {
	return true
}

func autoStartDefault() bool {
	enabled, err := autoStartEnabled()
	return err == nil && enabled
}

func autoStartEnabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, windowsRunKeyPath, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, fmt.Errorf("open startup registry key: %w", err)
	}
	defer key.Close()

	value, _, err := key.GetStringValue(windowsRunValueName)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, fmt.Errorf("read startup registry value: %w", err)
	}

	return strings.TrimSpace(value) != "", nil
}

func setAutoStart(enabled bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, windowsRunKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open startup registry key: %w", err)
	}
	defer key.Close()

	if !enabled {
		if err := key.DeleteValue(windowsRunValueName); err != nil && err != registry.ErrNotExist {
			return fmt.Errorf("remove startup registry value: %w", err)
		}
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	command := windowsAutoStartCommand(executable)
	if err := key.SetStringValue(windowsRunValueName, command); err != nil {
		return fmt.Errorf("write startup registry value: %w", err)
	}
	return nil
}

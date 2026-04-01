//go:build darwin

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const launchAgentLabel = "se.bananas.pos"

func autoStartSupported() bool {
	return true
}

func autoStartDefault() bool {
	enabled, err := autoStartEnabled()
	return err == nil && enabled
}

func autoStartEnabled() (bool, error) {
	path, err := autoStartPlistPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func setAutoStart(enabled bool) error {
	path, err := autoStartPlistPath()
	if err != nil {
		return err
	}
	if !enabled {
		_ = launchctlBootout()
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove login item: %w", err)
		}
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents directory: %w", err)
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "https://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<false/>
	<key>ProcessType</key>
	<string>Interactive</string>
</dict>
</plist>
`, xmlEscape(launchAgentLabel), xmlEscape(executable))

	if err := os.WriteFile(path, []byte(plist), 0o644); err != nil {
		return fmt.Errorf("write login item: %w", err)
	}
	_ = launchctlBootout()
	if err := launchctlBootstrap(path); err != nil {
		return err
	}
	return nil
}

func autoStartPlistPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(homeDir, "Library", "LaunchAgents", launchAgentLabel+".plist"), nil
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}

func launchctlBootstrap(plistPath string) error {
	if err := exec.Command("launchctl", "bootstrap", launchctlDomain(), plistPath).Run(); err != nil {
		return fmt.Errorf("register login item: %w", err)
	}
	return nil
}

func launchctlBootout() error {
	if err := exec.Command("launchctl", "bootout", launchctlDomain()+"/"+launchAgentLabel).Run(); err != nil {
		return fmt.Errorf("unregister login item: %w", err)
	}
	return nil
}

func launchctlDomain() string {
	return "gui/" + strconv.Itoa(os.Getuid())
}

package app

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"

	jobtransform "bananas-pos/internal/transform"
)

type Config struct {
	HTTPEnabled  bool
	HTTPAddr     string
	TCPEnabled   bool
	TCPAddr      string
	TargetMode   string
	Transform    string
	ProxyHTTPURL string
	EmulatorDPMM int
}

type runtimeState struct {
	HTTPEnabled bool
	HTTPAddr    string
	TCPEnabled  bool
	TCPAddr     string
	TargetMode  string
	Transform   string
}

type settingsState struct {
	HTTPEnabled bool
	HTTPPort    string
	TCPEnabled  bool
	TCPPort     string
	TargetMode  string
	Transform   string
}

const (
	prefTargetMode  = "settings.target_mode"
	prefTransform   = "settings.transform"
	prefHTTPEnabled = "settings.http_enabled"
	prefHTTPPort    = "settings.http_port"
	prefTCPEnabled  = "settings.tcp_enabled"
	prefTCPPort     = "settings.tcp_port"
)

var modeOptions = []struct {
	key   string
	label string
}{
	{key: "system-print-queue", label: "System Print Queue"},
	{key: "http-proxy", label: "HTTP Proxy"},
	{key: "emulator", label: "Emulator"},
}

var transformOptions = []struct {
	key   string
	label string
}{
	{key: "", label: "None"},
	{key: jobtransform.TransformEpsonESCPOS, label: "Epson ESC/POS (debug)"},
}

func newRuntimeState(config Config) runtimeState {
	return runtimeState{
		HTTPEnabled: config.HTTPEnabled,
		HTTPAddr:    config.HTTPAddr,
		TCPEnabled:  config.TCPEnabled,
		TCPAddr:     config.TCPAddr,
		TargetMode:  defaultTargetMode(config.TargetMode),
		Transform:   activeTransform(config.TargetMode, config.Transform),
	}
}

func loadSettings(prefs fyne.Preferences, config Config) settingsState {
	settings := settingsState{
		HTTPEnabled: config.HTTPEnabled,
		HTTPPort:    portFromAddr(config.HTTPAddr),
		TCPEnabled:  config.TCPEnabled,
		TCPPort:     portFromAddr(config.TCPAddr),
		TargetMode:  defaultTargetMode(config.TargetMode),
		Transform:   defaultTransform(config.Transform),
	}

	if mode := strings.TrimSpace(prefs.String(prefTargetMode)); mode != "" {
		if isValidTargetMode(mode) {
			settings.TargetMode = mode
		} else {
			prefs.RemoveValue(prefTargetMode)
		}
	}
	if transform := strings.TrimSpace(prefs.String(prefTransform)); transform != "" {
		if isValidTransform(transform) {
			settings.Transform = transform
		} else {
			prefs.RemoveValue(prefTransform)
		}
	}
	if settings.TargetMode != "system-print-queue" {
		settings.Transform = ""
		prefs.RemoveValue(prefTransform)
	}

	settings.HTTPEnabled = prefs.BoolWithFallback(prefHTTPEnabled, settings.HTTPEnabled)
	if port := strings.TrimSpace(prefs.String(prefHTTPPort)); port != "" {
		settings.HTTPPort = port
	}

	settings.TCPEnabled = prefs.BoolWithFallback(prefTCPEnabled, settings.TCPEnabled)
	if port := strings.TrimSpace(prefs.String(prefTCPPort)); port != "" {
		settings.TCPPort = port
	}

	return settings
}

func (s settingsState) apply(config Config) Config {
	config.TargetMode = defaultTargetMode(s.TargetMode)
	config.Transform = activeTransform(config.TargetMode, s.Transform)
	config.HTTPEnabled = s.HTTPEnabled
	config.TCPEnabled = s.TCPEnabled
	if addr, err := replacePort(config.HTTPAddr, s.HTTPPort); err == nil {
		config.HTTPAddr = addr
	}
	if addr, err := replacePort(config.TCPAddr, s.TCPPort); err == nil {
		config.TCPAddr = addr
	}
	return config
}

func (s settingsState) persist(prefs fyne.Preferences) {
	prefs.SetString(prefTargetMode, s.TargetMode)
	if s.Transform == "" {
		prefs.RemoveValue(prefTransform)
	} else {
		prefs.SetString(prefTransform, s.Transform)
	}
	prefs.SetBool(prefHTTPEnabled, s.HTTPEnabled)
	prefs.SetString(prefHTTPPort, s.HTTPPort)
	prefs.SetBool(prefTCPEnabled, s.TCPEnabled)
	prefs.SetString(prefTCPPort, s.TCPPort)
}

func modeLabel(mode string) string {
	for _, option := range modeOptions {
		if option.key == mode {
			return option.label
		}
	}
	return mode
}

func isValidTargetMode(mode string) bool {
	for _, option := range modeOptions {
		if option.key == strings.TrimSpace(mode) {
			return true
		}
	}
	return false
}

func isValidTransform(transform string) bool {
	for _, option := range transformOptions {
		if option.key == strings.TrimSpace(transform) {
			return true
		}
	}
	return false
}

func defaultTargetMode(mode string) string {
	if isValidTargetMode(mode) {
		return strings.TrimSpace(mode)
	}
	return "system-print-queue"
}

func defaultTransform(transform string) string {
	if isValidTransform(transform) {
		return strings.TrimSpace(transform)
	}
	return ""
}

func activeTransform(mode, transform string) string {
	if mode != "system-print-queue" {
		return ""
	}
	return defaultTransform(transform)
}

func portFromAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err == nil {
		_ = host
		return port
	}
	return strings.TrimPrefix(addr, ":")
}

func replacePort(addr, port string) (string, error) {
	if strings.HasPrefix(addr, ":") || !strings.Contains(addr, ":") {
		return ":" + port, nil
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(host, port), nil
}

func validatePort(value string) error {
	port, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("must be between 1 and 65535")
	}
	return nil
}

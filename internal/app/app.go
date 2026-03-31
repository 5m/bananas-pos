package app

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"

	httpinput "bananas-pos/internal/input/http"
	tcpinput "bananas-pos/internal/input/tcp"
	"bananas-pos/internal/meta"
	"bananas-pos/internal/target"
	jobtransform "bananas-pos/internal/transform"
	"bananas-pos/internal/trayicon"
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

type App struct {
	config      Config
	fyneApp     fyne.App
	desktopApp  desktop.App
	icon        fyne.Resource
	mainWindow  fyne.Window
	settingsWin fyne.Window
	target      *target.Switcher
	targetMode  string
	transform   string
	httpSrv     *httpinput.Server
	tcpSrv      *tcpinput.Server
	trayMenu    *fyne.Menu

	mu      sync.Mutex
	exitErr error
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

func modeLabel(mode string) string {
	for _, option := range modeOptions {
		if option.key == mode {
			return option.label
		}
	}
	return mode
}

func New(config Config) (*App, error) {
	fyneApplication := fyneapp.NewWithID("bananas-pos")
	config = loadConfigFromPreferences(fyneApplication.Preferences(), config)
	config.TargetMode = defaultTargetMode(config.TargetMode)
	config.Transform = defaultTransform(config.Transform)
	icon := trayicon.Resource()
	fyneApplication.SetIcon(icon)

	mainWindow := fyneApplication.NewWindow(meta.AppName)
	mainWindow.SetCloseIntercept(func() {
		mainWindow.Hide()
	})

	app := &App{
		config:     config,
		fyneApp:    fyneApplication,
		icon:       icon,
		mainWindow: mainWindow,
	}

	var err error
	initialTarget, err := app.newTarget(config.TargetMode, config.Transform)
	if err != nil {
		return nil, err
	}
	app.target = target.NewSwitcher(initialTarget, activeTransform(config.TargetMode, config.Transform))
	app.targetMode = config.TargetMode
	app.transform = config.Transform

	if config.HTTPEnabled {
		app.httpSrv = httpinput.NewServer(config.HTTPAddr, app.target)
	}
	if config.TCPEnabled {
		app.tcpSrv = tcpinput.NewServer(config.TCPAddr, app.target)
	}

	return app, nil
}

func (a *App) Run() error {
	a.setupTray()
	a.startServers()
	if err := a.target.Start(); err != nil {
		return fmt.Errorf("start target: %w", err)
	}
	a.fyneApp.Run()
	a.shutdown()
	return a.getExitErr()
}

func (a *App) setupTray() {
	desk, ok := a.fyneApp.(desktop.App)
	if !ok {
		return
	}
	a.desktopApp = desk

	items := []*fyne.MenuItem{
		fyne.NewMenuItem("Settings...", a.showSettings),
		fyne.NewMenuItem("Quit", func() {
			a.fyneApp.Quit()
		}),
	}

	a.trayMenu = fyne.NewMenu(meta.AppName, items...)
	a.refreshTray()
}

func (a *App) startServers() {
	if a.httpSrv != nil {
		go a.runServer("http input", a.httpSrv.Start)
	}
	if a.tcpSrv != nil {
		go a.runServer("tcp input", a.tcpSrv.Start)
	}
}

func (a *App) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if a.httpSrv != nil {
		if err := a.httpSrv.Shutdown(ctx); err != nil {
			a.setExitErr(fmt.Errorf("shutdown http server: %w", err))
		}
	}
	if a.tcpSrv != nil {
		if err := a.tcpSrv.Shutdown(ctx); err != nil {
			a.setExitErr(fmt.Errorf("shutdown tcp server: %w", err))
		}
	}
	if err := a.target.Shutdown(); err != nil {
		a.setExitErr(fmt.Errorf("shutdown target: %w", err))
	}
}

func (a *App) runServer(name string, start func() error) {
	log.Printf("starting %s with target %s", name, a.target.Name())
	if err := start(); err != nil {
		a.setExitErr(fmt.Errorf("%s failed: %w", name, err))
		a.fyneApp.Quit()
	}
}

func (a *App) newTarget(mode, _ string) (target.Target, error) {
	switch mode {
	case "http-proxy":
		return target.NewProxyHTTP(a.config.ProxyHTTPURL)
	case "system-print-queue":
		return target.NewRawSpool(), nil
	case "emulator":
		return target.NewEmulator(a.fyneApp, a.icon, a.config.EmulatorDPMM, a.fyneApp.Quit), nil
	default:
		return nil, fmt.Errorf("unknown target mode %q", mode)
	}
}

func (a *App) setExitErr(err error) {
	if err == nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.exitErr == nil {
		a.exitErr = err
	}
}

func (a *App) getExitErr() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.exitErr
}

func (a *App) switchOutput(mode, transform string) {
	transform = activeTransform(mode, transform)
	if mode == a.targetMode && transform == a.transform {
		if mode == "emulator" {
			a.target.ShowWindow()
		}
		return
	}

	next, err := a.newTarget(mode, transform)
	if err != nil {
		a.setExitErr(fmt.Errorf("switch output mode: %w", err))
		return
	}

	if err := a.target.Set(next); err != nil {
		a.setExitErr(fmt.Errorf("switch output mode: %w", err))
	}
	a.target.SetTransform(activeTransform(mode, transform))
	a.targetMode = mode
	a.transform = transform
	a.fyneApp.Preferences().SetString(prefTargetMode, mode)
	if transform == "" {
		a.fyneApp.Preferences().RemoveValue(prefTransform)
	} else {
		a.fyneApp.Preferences().SetString(prefTransform, transform)
	}
	a.refreshTray()

	if mode == "emulator" {
		a.target.ShowWindow()
	}
}

func (a *App) refreshTray() {
	if a.desktopApp == nil || a.trayMenu == nil {
		return
	}
	a.trayMenu.Refresh()
	a.desktopApp.SetSystemTrayIcon(a.icon)
	a.desktopApp.SetSystemTrayMenu(a.trayMenu)
}

func (a *App) httpAddr() string {
	if a.httpSrv != nil {
		return a.httpSrv.Addr()
	}
	return a.config.HTTPAddr
}

func (a *App) tcpAddr() string {
	if a.tcpSrv != nil {
		return a.tcpSrv.Addr()
	}
	return a.config.TCPAddr
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

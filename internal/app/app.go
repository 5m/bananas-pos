package app

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	httpinput "bananas-pos/internal/input/http"
	tcpinput "bananas-pos/internal/input/tcp"
	"bananas-pos/internal/meta"
	"bananas-pos/internal/target"
	"bananas-pos/internal/trayicon"
)

type Config struct {
	HTTPEnabled  bool
	HTTPAddr     string
	TCPEnabled   bool
	TCPAddr      string
	TargetMode   string
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
	httpSrv     *httpinput.Server
	tcpSrv      *tcpinput.Server
	trayMenu    *fyne.Menu

	mu      sync.Mutex
	exitErr error
}

const (
	prefTargetMode  = "settings.target_mode"
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
	initialTarget, err := app.newTarget(config.TargetMode)
	if err != nil {
		return nil, err
	}
	app.target = target.NewSwitcher(initialTarget)
	app.targetMode = config.TargetMode

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

func (a *App) newTarget(mode string) (target.Target, error) {
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

func (a *App) switchMode(mode string) {
	if mode == a.targetMode {
		if mode == "emulator" {
			a.target.ShowWindow()
		}
		return
	}

	next, err := a.newTarget(mode)
	if err != nil {
		a.setExitErr(fmt.Errorf("switch target mode: %w", err))
		return
	}

	if err := a.target.Set(next); err != nil {
		a.setExitErr(fmt.Errorf("switch target mode: %w", err))
	}
	a.targetMode = mode
	a.fyneApp.Preferences().SetString(prefTargetMode, mode)
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

func (a *App) showSettings() {
	if a.settingsWin == nil {
		window := a.fyneApp.NewWindow(meta.AppName)
		window.SetIcon(a.icon)
		window.Resize(fyne.NewSize(360, 220))
		window.SetFixedSize(true)
		window.SetCloseIntercept(func() {
			window.Hide()
		})
		a.settingsWin = window
	}

	modeLabels := make([]string, 0, len(modeOptions))
	for _, option := range modeOptions {
		modeLabels = append(modeLabels, option.label)
	}

	modeSelect := widget.NewSelect(modeLabels, nil)
	modeSelect.SetSelected(modeLabel(a.targetMode))

	httpEnabledCheck := widget.NewCheck("", nil)
	httpEnabledCheck.SetChecked(a.config.HTTPEnabled)

	httpPortEntry := widget.NewEntry()
	httpPortEntry.SetText(savedPortOrCurrent(a.fyneApp.Preferences(), prefHTTPPort, a.httpAddr()))

	tcpEnabledCheck := widget.NewCheck("", nil)
	tcpEnabledCheck.SetChecked(a.config.TCPEnabled)

	tcpPortEntry := widget.NewEntry()
	tcpPortEntry.SetText(savedPortOrCurrent(a.fyneApp.Preferences(), prefTCPPort, a.tcpAddr()))

	note := widget.NewLabel("Input and port changes apply after restart.")
	note.Wrapping = fyne.TextWrapWord
	note.Alignment = fyne.TextAlignTrailing
	note.TextStyle = fyne.TextStyle{Italic: true}

	form := widget.NewForm(
		widget.NewFormItem("Target", modeSelect),
		widget.NewFormItem("API Enabled", httpEnabledCheck),
		widget.NewFormItem("API Port", httpPortEntry),
		widget.NewFormItem("Raw Enabled", tcpEnabledCheck),
		widget.NewFormItem("Raw Port", tcpPortEntry),
	)

	saveButton := widget.NewButton("Save", func() {
		mode := modeKeyFromLabel(modeSelect.Selected)
		if mode == "" {
			dialog.ShowError(fmt.Errorf("select a mode"), a.settingsWin)
			return
		}

		httpPort := strings.TrimSpace(httpPortEntry.Text)
		if err := validatePort(httpPort); err != nil {
			dialog.ShowError(fmt.Errorf("invalid HTTP port: %w", err), a.settingsWin)
			return
		}

		tcpPort := strings.TrimSpace(tcpPortEntry.Text)
		if err := validatePort(tcpPort); err != nil {
			dialog.ShowError(fmt.Errorf("invalid TCP port: %w", err), a.settingsWin)
			return
		}

		prefs := a.fyneApp.Preferences()
		prefs.SetString(prefTargetMode, mode)
		prefs.SetBool(prefHTTPEnabled, httpEnabledCheck.Checked)
		prefs.SetString(prefHTTPPort, httpPort)
		prefs.SetBool(prefTCPEnabled, tcpEnabledCheck.Checked)
		prefs.SetString(prefTCPPort, tcpPort)

		if mode != a.targetMode {
			a.switchMode(mode)
		}

		a.settingsWin.Hide()
	})

	cancelButton := widget.NewButton("Cancel", func() {
		a.settingsWin.Hide()
	})
	versionLabel := widget.NewLabel("Version: " + meta.Version)

	a.settingsWin.SetContent(container.NewBorder(
		nil,
		container.NewPadded(container.NewHBox(versionLabel, layout.NewSpacer(), cancelButton, saveButton)),
		nil,
		nil,
		container.NewPadded(container.NewVBox(form, note)),
	))

	a.settingsWin.Show()
	a.settingsWin.RequestFocus()
}

func loadConfigFromPreferences(prefs fyne.Preferences, config Config) Config {
	if mode := strings.TrimSpace(prefs.String(prefTargetMode)); mode != "" {
		if isValidTargetMode(mode) {
			config.TargetMode = mode
		} else {
			prefs.RemoveValue(prefTargetMode)
		}
	}

	config.HTTPEnabled = prefs.BoolWithFallback(prefHTTPEnabled, config.HTTPEnabled)
	if port := strings.TrimSpace(prefs.String(prefHTTPPort)); port != "" {
		if addr, err := replacePort(config.HTTPAddr, port); err == nil {
			config.HTTPAddr = addr
		}
	}

	config.TCPEnabled = prefs.BoolWithFallback(prefTCPEnabled, config.TCPEnabled)
	if port := strings.TrimSpace(prefs.String(prefTCPPort)); port != "" {
		if addr, err := replacePort(config.TCPAddr, port); err == nil {
			config.TCPAddr = addr
		}
	}

	return config
}

func savedPortOrCurrent(prefs fyne.Preferences, key, addr string) string {
	if port := strings.TrimSpace(prefs.String(key)); port != "" {
		return port
	}
	return portFromAddr(addr)
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

func modeKeyFromLabel(label string) string {
	for _, option := range modeOptions {
		if option.label == label {
			return option.key
		}
	}
	return ""
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

func defaultTargetMode(mode string) string {
	if isValidTargetMode(mode) {
		return strings.TrimSpace(mode)
	}
	return "system-print-queue"
}

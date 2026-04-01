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
	"bananas-pos/internal/trayicon"
)

type App struct {
	baseConfig  Config
	active      runtimeState
	fyneApp     fyne.App
	desktopApp  desktop.App
	icon        fyne.Resource
	mainWindow  fyne.Window
	settingsWin fyne.Window
	target      *target.Switcher
	httpSrv     *httpinput.Server
	tcpSrv      *tcpinput.Server
	trayMenu    *fyne.Menu

	mu      sync.Mutex
	exitErr error
}

func New(config Config) (*App, error) {
	fyneApplication := fyneapp.NewWithID("bananas-pos")
	settings := loadSettings(fyneApplication.Preferences(), config)
	config = settings.apply(config)
	if config.TargetMode == "system-print-queue" {
		printerName, err := resolveConfiguredPrinterName(config.PrinterName)
		if err != nil {
			return nil, err
		}
		config.PrinterName = printerName
	}
	icon := trayicon.Resource()
	fyneApplication.SetIcon(icon)

	mainWindow := fyneApplication.NewWindow(meta.AppName)
	mainWindow.SetCloseIntercept(func() {
		mainWindow.Hide()
	})

	app := &App{
		baseConfig: config,
		active:     newRuntimeState(config),
		fyneApp:    fyneApplication,
		icon:       icon,
		mainWindow: mainWindow,
	}

	var err error
	initialTarget, err := app.newTarget(app.active.TargetMode, app.active.PrinterName)
	if err != nil {
		return nil, err
	}
	app.target = target.NewSwitcher(initialTarget, activeTransform(app.active.TargetMode, app.active.Transform))

	if app.active.HTTPEnabled {
		app.httpSrv = httpinput.NewServer(app.active.HTTPAddr, app.target)
	}
	if app.active.TCPEnabled {
		app.tcpSrv = tcpinput.NewServer(app.active.TCPAddr, app.target)
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

	quitItem := fyne.NewMenuItem("Quit", func() {
		a.fyneApp.Quit()
	})
	quitItem.IsQuit = true

	items := []*fyne.MenuItem{
		fyne.NewMenuItem("Settings...", a.showSettings),
		quitItem,
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

func (a *App) newTarget(mode, printerName string) (target.Target, error) {
	switch mode {
	case "http-proxy":
		return target.NewProxyHTTP(a.baseConfig.ProxyHTTPURL)
	case "system-print-queue":
		if strings.TrimSpace(printerName) == "" {
			return nil, fmt.Errorf("system print queue requires a selected printer")
		}
		return target.NewRawSpool(printerName), nil
	case "emulator":
		return target.NewEmulator(a.fyneApp, a.icon, a.baseConfig.EmulatorDPMM, a.fyneApp.Quit), nil
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

func (a *App) switchOutput(mode, printerName, transform string) {
	transform = activeTransform(mode, transform)
	printerName = defaultPrinterName(mode, printerName)
	if mode == a.active.TargetMode && printerName == a.active.PrinterName && transform == a.active.Transform {
		if mode == "emulator" {
			a.target.ShowWindow()
		}
		return
	}

	next, err := a.newTarget(mode, printerName)
	if err != nil {
		a.setExitErr(fmt.Errorf("switch output mode: %w", err))
		return
	}

	if err := a.target.Set(next); err != nil {
		a.setExitErr(fmt.Errorf("switch output mode: %w", err))
		return
	}
	a.target.SetTransform(activeTransform(mode, transform))
	a.active.TargetMode = mode
	a.active.PrinterName = printerName
	a.active.Transform = transform
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
	a.desktopApp.SetSystemTrayMenu(a.trayMenu)
	a.desktopApp.SetSystemTrayIcon(a.icon)
}

func (a *App) httpAddr() string {
	if a.httpSrv != nil {
		return a.httpSrv.Addr()
	}
	return a.active.HTTPAddr
}

func (a *App) tcpAddr() string {
	if a.tcpSrv != nil {
		return a.tcpSrv.Addr()
	}
	return a.active.TCPAddr
}

func resolveConfiguredPrinterName(printerName string) (string, error) {
	printerName = strings.TrimSpace(printerName)
	if printerName != "" {
		return printerName, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	descriptor, ok := any(target.NewRawSpool("")).(target.Descriptor)
	if !ok {
		return "", fmt.Errorf("system print queue does not expose printer details")
	}

	printerName, err := descriptor.Description(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve default printer: %w", err)
	}
	printerName = strings.TrimSpace(printerName)
	if printerName == "" {
		return "", fmt.Errorf("resolve default printer: printer name is empty")
	}
	return printerName, nil
}

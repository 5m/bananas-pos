package app

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"bananas-pos/internal/meta"
	"bananas-pos/internal/target"
)

func (a *App) showSettings() {
	if a.settingsWin == nil {
		window := a.fyneApp.NewWindow(meta.AppName)
		window.SetIcon(a.icon)
		window.Resize(fyne.NewSize(420, 280))
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
	transformLabels := make([]string, 0, len(transformOptions))
	for _, option := range transformOptions {
		transformLabels = append(transformLabels, option.label)
	}

	modeSelect := widget.NewSelect(modeLabels, nil)
	modeSelect.SetSelected(modeLabel(a.targetMode))
	targetDetails := widget.NewLabel("")
	targetDetails.Wrapping = fyne.TextWrapWord
	transformSelect := widget.NewSelect(transformLabels, nil)
	transformSelect.SetSelected(transformLabel(defaultTransform(a.transform)))
	updateTargetDetails := func(mode string) {
		description := a.targetDescription(mode)
		targetDetails.SetText(description)
		if strings.TrimSpace(description) == "" {
			targetDetails.Hide()
			return
		}
		targetDetails.Show()
	}

	httpEnabledCheck := widget.NewCheck("API", nil)
	httpEnabledCheck.SetChecked(a.config.HTTPEnabled)

	httpPortEntry := widget.NewEntry()
	httpPortEntry.SetText(savedPortOrCurrent(a.fyneApp.Preferences(), prefHTTPPort, a.httpAddr()))
	httpPortField := container.NewGridWrap(fyne.NewSize(72, httpPortEntry.MinSize().Height), httpPortEntry)
	httpPortLabel := widget.NewLabel("Port")
	httpPortRow := container.NewHBox(
		httpPortLabel,
		httpPortField,
	)

	tcpEnabledCheck := widget.NewCheck("TCP", nil)
	tcpEnabledCheck.SetChecked(a.config.TCPEnabled)

	tcpPortEntry := widget.NewEntry()
	tcpPortEntry.SetText(savedPortOrCurrent(a.fyneApp.Preferences(), prefTCPPort, a.tcpAddr()))
	tcpPortField := container.NewGridWrap(fyne.NewSize(72, tcpPortEntry.MinSize().Height), tcpPortEntry)
	tcpPortLabel := widget.NewLabel("Port")
	tcpPortRow := container.NewHBox(
		tcpPortLabel,
		tcpPortField,
	)

	updatePortVisibility := func(check *widget.Check, row *fyne.Container) {
		if check.Checked {
			row.Show()
			return
		}
		row.Hide()
	}
	httpEnabledCheck.OnChanged = func(_ bool) {
		updatePortVisibility(httpEnabledCheck, httpPortRow)
	}
	tcpEnabledCheck.OnChanged = func(_ bool) {
		updatePortVisibility(tcpEnabledCheck, tcpPortRow)
	}
	updatePortVisibility(httpEnabledCheck, httpPortRow)
	updatePortVisibility(tcpEnabledCheck, tcpPortRow)

	sectionTitle := func(text string) fyne.CanvasObject {
		return widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}
	transformCard := widget.NewCard("", "", container.NewVBox(
		sectionTitle("Transform"),
		transformSelect,
	))
	updateTransformVisibility := func(mode string) {
		if mode == "system-print-queue" {
			transformCard.Show()
			return
		}
		transformCard.Hide()
	}
	modeSelect.OnChanged = func(selected string) {
		mode := modeKeyFromLabel(selected)
		if mode != "system-print-queue" {
			transformSelect.SetSelected(transformLabel(""))
		}
		updateTargetDetails(mode)
		updateTransformVisibility(mode)
	}
	updateTargetDetails(a.targetMode)
	updateTransformVisibility(a.targetMode)

	apiRow := container.NewHBox(
		httpEnabledCheck,
		layout.NewSpacer(),
		httpPortRow,
	)
	tcpRow := container.NewHBox(
		tcpEnabledCheck,
		layout.NewSpacer(),
		tcpPortRow,
	)
	targetCard := widget.NewCard("", "", container.NewVBox(
		sectionTitle("Print Target"),
		modeSelect,
		targetDetails,
	))
	inputsCard := widget.NewCard("", "", container.NewVBox(
		sectionTitle("Server Routes"),
		apiRow,
		tcpRow,
	))
	settingsContent := container.NewVBox(
		targetCard,
		transformCard,
		inputsCard,
	)

	saveButton := widget.NewButton("Save", func() {
		mode := modeKeyFromLabel(modeSelect.Selected)
		if mode == "" {
			dialog.ShowError(fmt.Errorf("select a mode"), a.settingsWin)
			return
		}
		selectedTransformLabel := strings.TrimSpace(transformSelect.Selected)
		transform := transformKeyFromLabel(selectedTransformLabel)
		if selectedTransformLabel != "" && !isValidTransformLabel(selectedTransformLabel) {
			dialog.ShowError(fmt.Errorf("select a transform"), a.settingsWin)
			return
		}
		if mode != "system-print-queue" {
			transform = ""
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
		if transform == "" {
			prefs.RemoveValue(prefTransform)
		} else {
			prefs.SetString(prefTransform, transform)
		}
		prefs.SetBool(prefHTTPEnabled, httpEnabledCheck.Checked)
		prefs.SetString(prefHTTPPort, httpPort)
		prefs.SetBool(prefTCPEnabled, tcpEnabledCheck.Checked)
		prefs.SetString(prefTCPPort, tcpPort)

		if mode != a.targetMode || transform != a.transform {
			a.switchOutput(mode, transform)
		}

		a.settingsWin.Hide()
		if a.restartRequired(httpEnabledCheck.Checked, httpPort, tcpEnabledCheck.Checked, tcpPort) {
			dialog.ShowInformation("Restart Required", "Restart the app for API/TCP listener changes to take effect.", a.mainWindow)
		}
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
		container.NewPadded(settingsContent),
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
	if transform := strings.TrimSpace(prefs.String(prefTransform)); transform != "" {
		if isValidTransform(transform) {
			config.Transform = transform
		} else {
			prefs.RemoveValue(prefTransform)
		}
	}
	if config.TargetMode != "system-print-queue" {
		config.Transform = ""
		prefs.RemoveValue(prefTransform)
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

func transformLabel(transform string) string {
	for _, option := range transformOptions {
		if option.key == transform {
			return option.label
		}
	}
	return transform
}

func transformKeyFromLabel(label string) string {
	for _, option := range transformOptions {
		if option.label == label {
			return option.key
		}
	}
	return ""
}

func isValidTransformLabel(label string) bool {
	for _, option := range transformOptions {
		if option.label == strings.TrimSpace(label) {
			return true
		}
	}
	return false
}

func (a *App) restartRequired(httpEnabled bool, httpPort string, tcpEnabled bool, tcpPort string) bool {
	if httpEnabled != a.config.HTTPEnabled {
		return true
	}
	if strings.TrimSpace(httpPort) != portFromAddr(a.config.HTTPAddr) {
		return true
	}
	if tcpEnabled != a.config.TCPEnabled {
		return true
	}
	if strings.TrimSpace(tcpPort) != portFromAddr(a.config.TCPAddr) {
		return true
	}
	return false
}

func (a *App) targetDescription(mode string) string {
	switch mode {
	case "system-print-queue":
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		descriptor, ok := a.target.Current().(target.Descriptor)
		if !ok || mode != a.targetMode {
			descriptor, ok = any(target.NewRawSpool()).(target.Descriptor)
			if !ok {
				return "Unavailable"
			}
		}

		description, err := descriptor.Description(ctx)
		if err != nil {
			return "Unavailable"
		}

		return description
	case "http-proxy":
		return strings.TrimSpace(a.config.ProxyHTTPURL)
	default:
		return ""
	}
}

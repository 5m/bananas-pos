package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
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

	settings := loadSettings(a.fyneApp.Preferences(), a.baseConfig)

	modeLabels := make([]string, 0, len(modeOptions))
	for _, option := range modeOptions {
		modeLabels = append(modeLabels, option.label)
	}
	transformLabels := make([]string, 0, len(transformOptions))
	for _, option := range transformOptions {
		transformLabels = append(transformLabels, option.label)
	}

	modeSelect := widget.NewSelect(modeLabels, nil)
	modeSelect.SetSelected(modeLabel(a.active.TargetMode))
	printerSelect := widget.NewSelect(nil, nil)
	targetDetails := widget.NewLabel("")
	targetDetails.Wrapping = fyne.TextWrapWord
	transformSelect := widget.NewSelect(transformLabels, nil)
	transformSelect.SetSelected(transformLabel(defaultTransform(a.active.Transform)))
	printerOptionsForMode := func(mode string, selected string) []string {
		var options []string
		if mode != "system-print-queue" {
			return options
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		lister, ok := any(target.NewRawSpool(selected)).(target.PrinterLister)
		if !ok {
			return options
		}
		printers, err := lister.AvailablePrinters(ctx)
		if err != nil {
			return options
		}
		for _, name := range printers {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			options = append(options, name)
		}
		selected = strings.TrimSpace(selected)
		if selected != "" && !containsOption(options, selected) {
			options = append(options, selected)
		}
		return options
	}
	updatePrinterOptions := func(mode string, selected string) {
		options := printerOptionsForMode(mode, selected)
		printerSelect.Options = options
		value := strings.TrimSpace(selected)
		if mode != "system-print-queue" || len(options) == 0 {
			printerSelect.ClearSelected()
			return
		}
		if value != "" && containsOption(options, value) {
			printerSelect.SetSelected(value)
			return
		}

		defaultPrinter, err := resolveConfiguredPrinterName("")
		if err == nil && containsOption(options, defaultPrinter) {
			printerSelect.SetSelected(defaultPrinter)
			return
		}
		if len(options) == 1 {
			printerSelect.SetSelected(options[0])
			return
		}
		printerSelect.ClearSelected()
	}
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
	httpEnabledCheck.SetChecked(settings.HTTPEnabled)

	httpPortEntry := widget.NewEntry()
	httpPortEntry.SetText(settings.HTTPPort)
	httpPortField := container.NewGridWrap(fyne.NewSize(72, httpPortEntry.MinSize().Height), httpPortEntry)
	httpPortLabel := widget.NewLabel("Port")
	httpPortRow := container.NewHBox(httpPortLabel, httpPortField)

	tcpEnabledCheck := widget.NewCheck("TCP", nil)
	tcpEnabledCheck.SetChecked(settings.TCPEnabled)

	tcpPortEntry := widget.NewEntry()
	tcpPortEntry.SetText(settings.TCPPort)
	tcpPortField := container.NewGridWrap(fyne.NewSize(72, tcpPortEntry.MinSize().Height), tcpPortEntry)
	tcpPortLabel := widget.NewLabel("Port")
	tcpPortRow := container.NewHBox(tcpPortLabel, tcpPortField)

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

	autoStartCheck := widget.NewCheck("Start automatically when you log in", nil)
	autoStartCheck.SetChecked(settings.AutoStart)

	sectionTitle := func(text string) fyne.CanvasObject {
		return widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}
	sectionHeader := func(title string, trailing fyne.CanvasObject) fyne.CanvasObject {
		objects := []fyne.CanvasObject{sectionTitle(title)}
		if trailing != nil {
			objects = append(objects, layout.NewSpacer(), trailing)
		}
		return container.NewHBox(objects...)
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
	updatePrinterVisibility := func(mode string) {
		if mode == "system-print-queue" {
			printerSelect.Show()
			targetDetails.Hide()
			return
		}
		printerSelect.Hide()
		updateTargetDetails(mode)
	}
	modeSelect.OnChanged = func(selected string) {
		mode := modeKeyFromLabel(selected)
		if mode != "system-print-queue" {
			printerSelect.ClearSelected()
			transformSelect.SetSelected(transformLabel(""))
		}
		currentPrinter := settings.PrinterName
		if mode == a.active.TargetMode && strings.TrimSpace(a.active.PrinterName) != "" {
			currentPrinter = a.active.PrinterName
		}
		updatePrinterOptions(mode, currentPrinter)
		updateTargetDetails(mode)
		updatePrinterVisibility(mode)
		updateTransformVisibility(mode)
	}
	initialPrinter := settings.PrinterName
	if a.active.TargetMode == "system-print-queue" && strings.TrimSpace(a.active.PrinterName) != "" {
		initialPrinter = a.active.PrinterName
	}
	updatePrinterOptions(a.active.TargetMode, initialPrinter)
	updateTargetDetails(a.active.TargetMode)
	updatePrinterVisibility(a.active.TargetMode)
	updateTransformVisibility(a.active.TargetMode)

	apiRow := container.NewHBox(httpEnabledCheck, layout.NewSpacer(), httpPortRow)
	tcpRow := container.NewHBox(tcpEnabledCheck, layout.NewSpacer(), tcpPortRow)
	hostIP := hostIPAddress()
	hostIPLabel := widget.NewLabel("")
	hostIPLabel.Alignment = fyne.TextAlignTrailing
	if strings.TrimSpace(hostIP) != "" {
		hostIPLabel.SetText("Host IP: " + hostIP)
	}
	targetCard := widget.NewCard("", "", container.NewVBox(
		sectionTitle("Print Target"),
		modeSelect,
		printerSelect,
		targetDetails,
	))
	inputsCard := widget.NewCard("", "", container.NewVBox(
		sectionHeader("Server Routes", hostIPLabel),
		apiRow,
		tcpRow,
	))
	startupObjects := []fyne.CanvasObject{}
	if autoStartSupported() {
		startupObjects = append(startupObjects,
			sectionTitle("Startup"),
			autoStartCheck,
		)
	}
	startupCard := widget.NewCard("", "", container.NewVBox(startupObjects...))
	settingsObjects := []fyne.CanvasObject{targetCard, transformCard, inputsCard}
	if info, errors := a.listenerErrorDetails(); strings.TrimSpace(info) != "" {
		infoSegment := &widget.TextSegment{
			Text: info,
			Style: widget.RichTextStyle{
				ColorName: theme.ColorNameForeground,
			},
		}
		errorSegment := &widget.TextSegment{
			Text: strings.Join(errors, "\n"),
			Style: widget.RichTextStyle{
				ColorName: theme.ColorNameError,
			},
		}
		notice := widget.NewRichText(
			infoSegment,
			&widget.TextSegment{Text: "\n"},
			errorSegment,
		)
		notice.Wrapping = fyne.TextWrapWord
		settingsObjects = append([]fyne.CanvasObject{
			widget.NewCard("", "", container.NewVBox(
				sectionTitle("Startup Error"),
				notice,
			)),
		}, settingsObjects...)
	}
	if autoStartSupported() {
		settingsObjects = append(settingsObjects, startupCard)
	}
	settingsContent := container.NewVBox(settingsObjects...)

	saveButton := widget.NewButton("Save", func() {
		next, err := settingsFromForm(
			modeSelect.Selected,
			printerSelect.Selected,
			transformSelect.Selected,
			httpEnabledCheck.Checked,
			httpPortEntry.Text,
			tcpEnabledCheck.Checked,
			tcpPortEntry.Text,
			autoStartCheck.Checked,
		)
		if err != nil {
			dialog.ShowError(err, a.settingsWin)
			return
		}

		if err := setAutoStart(next.AutoStart); err != nil {
			dialog.ShowError(err, a.settingsWin)
			return
		}

		next.persist(a.fyneApp.Preferences())
		if next.TargetMode != a.active.TargetMode || next.PrinterName != a.active.PrinterName || next.Transform != a.active.Transform {
			a.switchOutput(next.TargetMode, next.PrinterName, next.Transform)
		}

		a.settingsWin.Hide()
		if a.listenerRestartRequired(next) {
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

func settingsFromForm(modeLabelValue, printerName, transformLabelValue string, httpEnabled bool, httpPort string, tcpEnabled bool, tcpPort string, autoStart bool) (settingsState, error) {
	mode := modeKeyFromLabel(modeLabelValue)
	if mode == "" {
		return settingsState{}, fmt.Errorf("select a mode")
	}
	printerName = defaultPrinterName(mode, printerName)
	if mode == "system-print-queue" && printerName == "" {
		return settingsState{}, fmt.Errorf("select a printer")
	}

	selectedTransformLabel := strings.TrimSpace(transformLabelValue)
	transform := transformKeyFromLabel(selectedTransformLabel)
	if selectedTransformLabel != "" && !isValidTransformLabel(selectedTransformLabel) {
		return settingsState{}, fmt.Errorf("select a transform")
	}
	if mode != "system-print-queue" {
		transform = ""
	}

	httpPort = strings.TrimSpace(httpPort)
	if err := validatePort(httpPort); err != nil {
		return settingsState{}, fmt.Errorf("invalid HTTP port: %w", err)
	}

	tcpPort = strings.TrimSpace(tcpPort)
	if err := validatePort(tcpPort); err != nil {
		return settingsState{}, fmt.Errorf("invalid TCP port: %w", err)
	}

	return settingsState{
		HTTPEnabled: httpEnabled,
		HTTPPort:    httpPort,
		TCPEnabled:  tcpEnabled,
		TCPPort:     tcpPort,
		AutoStart:   autoStart,
		TargetMode:  mode,
		PrinterName: printerName,
		Transform:   transform,
	}, nil
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

func (a *App) listenerRestartRequired(next settingsState) bool {
	if next.HTTPEnabled != a.active.HTTPEnabled {
		return true
	}
	if next.HTTPPort != portFromAddr(a.active.HTTPAddr) {
		return true
	}
	if next.TCPEnabled != a.active.TCPEnabled {
		return true
	}
	if next.TCPPort != portFromAddr(a.active.TCPAddr) {
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
		if !ok || mode != a.active.TargetMode {
			descriptor, ok = any(target.NewRawSpool("")).(target.Descriptor)
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
		return strings.TrimSpace(a.baseConfig.ProxyHTTPURL)
	default:
		return ""
	}
}

func containsOption(options []string, value string) bool {
	for _, option := range options {
		if option == value {
			return true
		}
	}
	return false
}

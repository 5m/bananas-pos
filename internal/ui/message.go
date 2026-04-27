package ui

import (
	"sync"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func ShowMessage(title, message string, icon fyne.Resource) {
	app := fyneapp.New()
	window := newMessageWindow(app, title, message, icon, nil)
	window.ShowAndRun()
}

func ShowWindowMessage(app fyne.App, title, message string, icon fyne.Resource, onClose func()) {
	window := newMessageWindow(app, title, message, icon, onClose)
	window.Show()
	window.RequestFocus()
}

func newMessageWindow(app fyne.App, title, message string, icon fyne.Resource, onClose func()) fyne.Window {
	if icon != nil {
		app.SetIcon(icon)
	}

	window := app.NewWindow(title)
	window.Resize(fyne.NewSize(480, 180))

	body := widget.NewLabel(message)
	body.Wrapping = fyne.TextWrapWord

	var closeOnce sync.Once
	closeWindow := func() {
		closeOnce.Do(func() {
			window.SetCloseIntercept(nil)
			window.Close()
			if onClose != nil {
				onClose()
			}
		})
	}

	window.SetContent(container.NewBorder(
		nil,
		container.NewPadded(container.NewHBox(
			layout.NewSpacer(),
			widget.NewButton("OK", closeWindow),
		)),
		nil,
		nil,
		container.NewPadded(body),
	))
	window.SetCloseIntercept(closeWindow)

	return window
}

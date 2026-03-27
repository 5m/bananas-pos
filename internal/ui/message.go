package ui

import (
	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func ShowMessage(title, message string, icon fyne.Resource) {
	app := fyneapp.New()
	if icon != nil {
		app.SetIcon(icon)
	}

	window := app.NewWindow(title)
	window.Resize(fyne.NewSize(360, 160))
	window.SetContent(container.NewBorder(
		nil,
		container.NewHBox(layout.NewSpacer(), widget.NewButton("OK", func() {
			app.Quit()
		})),
		nil,
		nil,
		container.NewPadded(widget.NewLabel(message)),
	))
	window.SetCloseIntercept(func() {
		app.Quit()
	})
	window.ShowAndRun()
}

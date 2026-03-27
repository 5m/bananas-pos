package trayicon

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed assets/icon.png
var pngIcon []byte

func Resource() fyne.Resource {
	return fyne.NewStaticResource("icon.png", pngIcon)
}

package app

import "strings"

func windowsAutoStartCommand(executable string) string {
	return `"` + strings.ReplaceAll(executable, `"`, `\"`) + `"`
}

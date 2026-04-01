//go:build !darwin && !windows

package app

func autoStartSupported() bool {
	return false
}

func autoStartDefault() bool {
	return false
}

func autoStartEnabled() (bool, error) {
	return false, nil
}

func setAutoStart(enabled bool) error {
	_ = enabled
	return nil
}

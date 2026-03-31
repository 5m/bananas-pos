//go:build !darwin && !linux && !windows

package main

func detachIfNeeded() (bool, error) {
	return false, nil
}

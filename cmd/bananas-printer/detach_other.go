//go:build !darwin && !linux

package main

func detachIfNeeded() (bool, error) {
	return false, nil
}

//go:build darwin || linux

package singleinstance

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

type Handle struct {
	file *os.File
}

func Acquire(name string) (*Handle, bool, error) {
	lockPath, err := lockFilePath(name)
	if err != nil {
		return nil, false, err
	}

	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, true, nil
		}
		return nil, false, err
	}

	return &Handle{file: file}, false, nil
}

func (h *Handle) Release() error {
	if h == nil || h.file == nil {
		return nil
	}

	err := syscall.Flock(int(h.file.Fd()), syscall.LOCK_UN)
	closeErr := h.file.Close()
	h.file = nil
	if err != nil {
		return err
	}
	return closeErr
}

func lockFilePath(name string) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		cacheDir = os.TempDir()
	}

	dir := filepath.Join(cacheDir, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	return filepath.Join(dir, name+".lock"), nil
}

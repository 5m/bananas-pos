//go:build windows

package singleinstance

import (
	"golang.org/x/sys/windows"
)

type Handle struct {
	mutex windows.Handle
}

func Acquire(name string) (*Handle, bool, error) {
	namePtr, err := windows.UTF16PtrFromString("Local\\" + name)
	if err != nil {
		return nil, false, err
	}

	mutex, err := windows.CreateMutex(nil, false, namePtr)
	if err != nil {
		if mutex != 0 {
			windows.CloseHandle(mutex)
		}
		if err == windows.ERROR_ALREADY_EXISTS {
			return nil, true, nil
		}
		return nil, false, err
	}

	return &Handle{mutex: mutex}, false, nil
}

func (h *Handle) Release() error {
	if h == nil || h.mutex == 0 {
		return nil
	}

	err := windows.CloseHandle(h.mutex)
	h.mutex = 0
	return err
}

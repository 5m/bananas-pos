//go:build darwin || linux

package main

import (
	"os"
	"os/exec"
	"syscall"
)

const noDetachEnv = "BANANAS_POS_DEBUG"

func detachIfNeeded() (bool, error) {
	if os.Getenv(noDetachEnv) != "" || !hasInteractiveTerminal() {
		return false, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return false, err
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return false, err
	}
	defer devNull.Close()

	cmd := exec.Command(executable, os.Args[1:]...)
	cmd.Env = append(os.Environ(), noDetachEnv+"=1")
	cmd.Stdin = devNull
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return false, err
	}

	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}

	return true, nil
}

func hasInteractiveTerminal() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

//go:build !windows

package main

import (
	"errors"
	"syscall"
)

// pidAlive reports whether a process with this PID exists. kill(pid, 0) sends
// no signal; EPERM means the process exists but belongs to someone else.
func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

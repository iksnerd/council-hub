//go:build windows

package main

import "os"

// pidAlive reports whether a process with this PID exists.
func pidAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	_ = p.Release()
	return true
}

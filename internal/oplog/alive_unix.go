//go:build !windows

package oplog

import (
	"errors"
	"syscall"
)

// processAlive reports whether pid names a running process (EPERM: alive, another user's).
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

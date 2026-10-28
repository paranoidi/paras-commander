//go:build windows

package cmdrun

import (
	"os"
	"os/exec"
	"syscall"
)

// detachProcessGroup is a no-op on Windows: there is no controlling-terminal grab to avoid,
// and CommandContext's default Cancel kills the process.
func detachProcessGroup(*exec.Cmd) {}

// SignalGroup kills p: Windows has no process groups to signal and no SIGTERM delivery, so every
// signal terminates the process itself.
func SignalGroup(p *os.Process, _ syscall.Signal) error {
	return p.Kill()
}

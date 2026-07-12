//go:build !linux

package subshell

import "github.com/gdamore/tcell/v2"

// Subshell is a persistent PTY-backed shell (Linux only).
type Subshell struct{}

// StartOptions configures [Start].
type StartOptions struct {
	Shell   string
	Dir     string
	Command string
}

// Start is not available on this platform.
func Start(StartOptions) (*Subshell, error) {
	return nil, ErrUnsupportedPlatform
}

// Alive always reports false on unsupported platforms.
func (s *Subshell) Alive() bool {
	return false
}

// RunVisible is not available on this platform.
func (s *Subshell) RunVisible(tcell.Screen) (bool, error) {
	return false, ErrUnsupportedPlatform
}

// Close is a no-op on unsupported platforms.
func (s *Subshell) Close() error {
	return nil
}

// PTYFd returns zero on unsupported platforms.
func (s *Subshell) PTYFd() int {
	return 0
}

// Cwd is not available on this platform.
func (s *Subshell) Cwd() (string, error) {
	return "", ErrUnsupportedPlatform
}

// Busy always reports false on unsupported platforms.
func (s *Subshell) Busy() bool {
	return false
}

// Chdir is not available on this platform.
func (s *Subshell) Chdir(string) error {
	return ErrUnsupportedPlatform
}

// SaveLaunchTerminal is a no-op on unsupported platforms.
func SaveLaunchTerminal() {}

// RestoreLaunchTerminal is a no-op on unsupported platforms.
func RestoreLaunchTerminal() {}

// EmergencyRestoreScreen resumes tcell on unsupported platforms.
func EmergencyRestoreScreen(screen tcell.Screen) {
	_ = screen.Resume()
}

// ShutdownTerminal releases tcell on unsupported platforms.
func ShutdownTerminal(screen tcell.Screen) {
	screen.Fini()
}

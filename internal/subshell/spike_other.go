//go:build !linux

package subshell

import "github.com/gdamore/tcell/v2"

// Spike is a Phase 0 PTY toggle prototype (Linux only).
type Spike struct{}

// StartOptions configures [StartSpike].
type StartOptions struct {
	Shell   string
	Dir     string
	Command string
}

// StartSpike is not available on this platform.
func StartSpike(StartOptions) (*Spike, error) {
	return nil, ErrUnsupportedPlatform
}

// Alive always reports false on unsupported platforms.
func (s *Spike) Alive() bool {
	return false
}

// RunVisible is not available on this platform.
func (s *Spike) RunVisible(tcell.Screen) (bool, error) {
	return false, ErrUnsupportedPlatform
}

// Close is a no-op on unsupported platforms.
func (s *Spike) Close() error {
	return nil
}

// PTYFd returns zero on unsupported platforms.
func (s *Spike) PTYFd() int {
	return 0
}

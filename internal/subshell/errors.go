package subshell

import "errors"

var (
	// ErrNotAlive is returned when the shell child has exited.
	ErrNotAlive = errors.New("subshell: shell not alive")
	// ErrFeedStopped is returned when the visible feed goroutines exit unexpectedly.
	ErrFeedStopped = errors.New("subshell: visible feed stopped")
	// ErrUnsupportedPlatform is returned on non-Linux builds.
	ErrUnsupportedPlatform = errors.New("subshell: unsupported platform")
)

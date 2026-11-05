//go:build !linux

// Package dirwatch is a no-op off Linux: nothing is watched and callers rely on their periodic
// refresh.
package dirwatch

// Watcher never reports anything on this platform.
type Watcher struct{}

// New returns an inert Watcher.
func New(func(dir string)) *Watcher { return &Watcher{} }

// Set does nothing.
func (*Watcher) Set([]string) {}

// Close does nothing.
func (*Watcher) Close() {}

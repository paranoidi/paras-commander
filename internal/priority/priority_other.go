//go:build !linux

package priority

// ApplyBackgroundPriority is a no-op on non-Linux platforms.
func ApplyBackgroundPriority(niceIncrement int) (restore func()) {
	return func() {}
}

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

// StartArgv is not available on this platform.
func StartArgv([]string, string) (*Subshell, error) {
	return nil, ErrUnsupportedPlatform
}

// Done returns an already-closed channel on unsupported platforms.
func (s *Subshell) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

// ExitCode always reports -1 on unsupported platforms.
func (s *Subshell) ExitCode() int { return -1 }

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

// InsertText is not available on this platform.
func (s *Subshell) InsertText(string) error {
	return ErrUnsupportedPlatform
}

// WritePTY is not available on this platform.
func (s *Subshell) WritePTY([]byte) (int, error) {
	return 0, ErrUnsupportedPlatform
}

// PanelFeed is not available on unsupported platforms.
type PanelFeed struct{}

// StartPanelFeed is not available on this platform.
func (s *Subshell) StartPanelFeed(int, int, func()) (*PanelFeed, error) {
	return nil, ErrUnsupportedPlatform
}

// Resize is a no-op on unsupported platforms.
func (f *PanelFeed) Resize(int, int) {}

// Pause is a no-op on unsupported platforms.
func (f *PanelFeed) Pause() {}

// Resume is a no-op on unsupported platforms.
func (f *PanelFeed) Resume(int, int) {}

// Close is a no-op on unsupported platforms.
func (f *PanelFeed) Close() {}

// Exited always reports false on unsupported platforms.
func (f *PanelFeed) Exited() bool { return false }

// AppCursor always reports false on unsupported platforms.
func (f *PanelFeed) AppCursor() bool { return false }

// Cursor reports no cursor on unsupported platforms.
func (f *PanelFeed) Cursor() (int, int, bool) { return 0, 0, false }

// Draw is a no-op on unsupported platforms.
func (f *PanelFeed) Draw(tcell.Style, func(x, y int, r rune, style tcell.Style)) (int, int, bool) {
	return 0, 0, false
}

// SnapshotText always reports empty text on unsupported platforms.
func (f *PanelFeed) SnapshotText() string { return "" }

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

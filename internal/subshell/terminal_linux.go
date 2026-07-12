//go:build linux

package subshell

import (
	"errors"
	"os"
	"sync"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// Kitty keyboard protocol (foot, kitty, …). Disable while proxying stdin so Ctrl+O is 0x0f.
const (
	kittyKeyboardDisable = "\x1b[>0u"
	kittyKeyboardEnable  = "\x1b[>1u"
)

var (
	launchTTYMu   sync.Mutex
	launchTTYFile *os.File
	launchTTY     *term.State
)

// SaveLaunchTerminal records termios before tcell Init (call from main). The tty handle stays
// open until [RestoreLaunchTerminal] — a closed fd could be reused and restored by mistake.
func SaveLaunchTerminal() {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return
	}
	fd := int(f.Fd())
	if !term.IsTerminal(fd) {
		_ = f.Close()
		return
	}
	state, err := term.GetState(fd)
	if err != nil {
		_ = f.Close()
		return
	}
	launchTTYMu.Lock()
	launchTTYFile = f
	launchTTY = state
	launchTTYMu.Unlock()
}

// RestoreLaunchTerminal restores the terminal to the state from [SaveLaunchTerminal].
func RestoreLaunchTerminal() {
	launchTTYMu.Lock()
	f := launchTTYFile
	state := launchTTY
	launchTTYFile = nil
	launchTTY = nil
	launchTTYMu.Unlock()
	if f == nil || state == nil {
		return
	}
	fd := int(f.Fd())
	discardPendingStdin(fd)
	_ = term.Restore(fd, state)
	enableKittyKeyboardProtocol()
	_ = f.Close()
}

// openControllingTTY returns a read/write handle to the interactive terminal.
// After [tcell.Screen.Suspend], os.Stdin may not receive keypresses; /dev/tty always does.
func openControllingTTY() (*os.File, error) {
	return os.OpenFile("/dev/tty", os.O_RDWR, 0)
}

// syncPTYSize sets the PTY window to match termSizeFrom (usually stdout). cellsChanged
// reports whether rows/cols actually changed — that Setsize already delivers a real WINCH
// with a new size, so callers can skip forceFullRedraw. Pixel-only changes don't count:
// size-comparing apps ignore them.
func syncPTYSize(ptyMaster, termSizeFrom *os.File) (cellsChanged bool, err error) {
	ws, err := pty.GetsizeFull(termSizeFrom)
	if err != nil {
		return false, err
	}
	cur, curErr := pty.GetsizeFull(ptyMaster)
	if curErr == nil && *cur == *ws {
		return false, nil
	}
	cellsChanged = curErr != nil || cur.Rows != ws.Rows || cur.Cols != ws.Cols
	return cellsChanged, pty.Setsize(ptyMaster, ws)
}

// forceFullRedraw jiggles the PTY winsize (rows-1, then the real size back) so a full-screen
// app that kept running while the TUI owned the terminal repaints from scratch — its frame is
// gone and it would otherwise only send diffs against it. A plain SIGWINCH is not enough:
// tcell/gocui apps skip repainting when the size is unchanged. The pause lets the app observe
// the shrunken size before the restore. ponytail: fixed 30ms settle; bump if some app
// coalesces both resizes into none.
func forceFullRedraw(ptyMaster *os.File) {
	ws, err := pty.GetsizeFull(ptyMaster)
	if err != nil || ws.Rows < 2 {
		return
	}
	smaller := *ws
	smaller.Rows--
	if pty.Setsize(ptyMaster, &smaller) != nil {
		return
	}
	time.Sleep(30 * time.Millisecond)
	_ = pty.Setsize(ptyMaster, ws)
}

func disableKittyKeyboardProtocol() { setKittyKeyboardProtocol(false) }

func enableKittyKeyboardProtocol() { setKittyKeyboardProtocol(true) }

func setKittyKeyboardProtocol(enable bool) {
	seq := kittyKeyboardDisable
	if enable {
		seq = kittyKeyboardEnable
	}
	if f, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0); err == nil {
		_, _ = f.WriteString(seq)
		_ = f.Close()
		return
	}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		_, _ = os.Stdout.WriteString(seq)
	}
}

// discardPendingStdin drops bytes already read into the kernel buffer (e.g. after Ctrl+O).
func discardPendingStdin(fd int) {
	_ = unix.SetNonblock(fd, true)
	defer func() { _ = unix.SetNonblock(fd, false) }()
	buf := make([]byte, 256)
	for {
		n, err := unix.Read(fd, buf)
		if n <= 0 || errors.Is(err, unix.EAGAIN) {
			return
		}
	}
}

// enterHostRawOn prepares the host terminal for shell-visible proxy I/O.
// restore leaves cooked mode for [tcell.Screen.Resume]; call [enableKittyKeyboardProtocol] after Resume.
func enterHostRawOn(fd int) (restore func(), err error) {
	if !term.IsTerminal(fd) {
		return func() {}, nil
	}
	disableKittyKeyboardProtocol()
	drainHostTTYResponses(fd)
	discardPendingStdin(fd)
	state, err := term.MakeRaw(fd)
	if err != nil {
		enableKittyKeyboardProtocol()
		return nil, err
	}
	return func() {
		discardPendingStdin(fd)
		_ = term.Restore(fd, state)
		drainHostTTYResponses(fd)
	}, nil
}

func drainHostTTYResponses(fd int) {
	_ = unix.SetNonblock(fd, true)
	defer func() { _ = unix.SetNonblock(fd, false) }()
	buf := make([]byte, 256)
	deadline := time.Now().Add(50 * time.Millisecond)
	for time.Now().Before(deadline) {
		n, err := unix.Read(fd, buf)
		if n <= 0 || errors.Is(err, unix.EAGAIN) {
			return
		}
	}
}

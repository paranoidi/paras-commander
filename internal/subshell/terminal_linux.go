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

// syncPTYSize sets the PTY window to match termSizeFrom (usually stdout).
func syncPTYSize(ptyMaster, termSizeFrom *os.File) error {
	ws, err := pty.GetsizeFull(termSizeFrom)
	if err != nil {
		return err
	}
	return pty.Setsize(ptyMaster, ws)
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

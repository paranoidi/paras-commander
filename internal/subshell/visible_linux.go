//go:build linux

package subshell

import (
	"sync"

	"github.com/gdamore/tcell/v2"
)

var visibleMu sync.Mutex
var visibleRestoreRaw func()

func registerVisibleRestore(restore func()) {
	visibleMu.Lock()
	visibleRestoreRaw = restore
	visibleMu.Unlock()
}

func clearVisibleRestore() {
	visibleMu.Lock()
	visibleRestoreRaw = nil
	visibleMu.Unlock()
}

func takeVisibleRestore() func() {
	visibleMu.Lock()
	restore := visibleRestoreRaw
	visibleRestoreRaw = nil
	visibleMu.Unlock()
	return restore
}

// EmergencyRestoreScreen leaves shell-visible mode and returns the terminal for the parent shell.
func EmergencyRestoreScreen(screen tcell.Screen) {
	if restore := takeVisibleRestore(); restore != nil {
		restore()
	}
	_ = screen.Resume()
	enableKittyKeyboardProtocol()
}

// ShutdownTerminal fully releases the sub: visible session, tcell, then launch termios.
func ShutdownTerminal(screen tcell.Screen) {
	EmergencyRestoreScreen(screen)
	screen.Fini()
	RestoreLaunchTerminal()
}

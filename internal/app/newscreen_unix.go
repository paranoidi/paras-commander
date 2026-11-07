//go:build unix

package app

import "github.com/gdamore/tcell/v2"

// newTerminalScreen is tcell.NewScreen over /dev/tty, with altShiftTty filtering the input.
func newTerminalScreen() (tcell.Screen, error) {
	tty, err := tcell.NewDevTty()
	if err != nil {
		return nil, err
	}
	return tcell.NewTerminfoScreenFromTty(&altShiftTty{Tty: tty})
}

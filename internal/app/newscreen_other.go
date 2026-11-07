//go:build !unix

package app

import "github.com/gdamore/tcell/v2"

// newTerminalScreen is tcell.NewScreen; the ESC O rewrite (altshift_tty.go) is only wired up on unix ttys.
func newTerminalScreen() (tcell.Screen, error) { return tcell.NewScreen() }

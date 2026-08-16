package subshell

import "github.com/gdamore/tcell/v2"

// PanelDrawer adapts a Subshell/PanelFeed pair to a generic terminal-panel drawer: paints live
// content and forwards input. Used by both the persistent Alt+P shell (internal/app) and
// run-for-each PTY sessions (internal/apphandler/commands) so callers need no special-casing
// between the two — both satisfy ui.TerminalDrawer structurally.
type PanelDrawer struct {
	Sub   *Subshell
	Feed  *PanelFeed
	Style tcell.Style
}

func (d *PanelDrawer) DrawTo(setCell func(x, y int, r rune, style tcell.Style)) (int, int, bool) {
	return d.Feed.Draw(d.Style, setCell)
}

func (d *PanelDrawer) WriteInput(b []byte) (int, error) { return d.Sub.WritePTY(b) }
func (d *PanelDrawer) AppCursorMode() bool              { return d.Feed.AppCursor() }
func (d *PanelDrawer) Cursor() (int, int, bool)         { return d.Feed.Cursor() }

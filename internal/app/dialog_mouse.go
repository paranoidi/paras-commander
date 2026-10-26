package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// handleDialogMouse applies a press or a wheel notch on the top-most dialog through the engine:
// dlg.Click / dlg.Wheel move the focus and return the event the matching key would produce, and
// the dialog's own event function applies it, so a click and its keyboard equivalent run the same
// code. OK / Cancel run the dialog's central buttons when it sets them. Presses that hit no widget
// (frame, labels, reserved regions) are swallowed.
func (a *App) handleDialogMouse(x, y int, buttons, pressed tcell.ButtonMask) (quit, rendered bool) {
	wheel := wheelDelta(buttons, config.MouseWheelRows)
	if wheel == 0 && pressed&tcell.Button1 == 0 {
		return false, false
	}
	d, ok := a.activeDialog(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !ok {
		return false, false
	}
	spec, fit := d.spec()
	if !fit {
		return false, false
	}
	w, h := a.screen.Size()
	var ev dlg.Event
	if wheel != 0 {
		ev, ok = dlg.Wheel(spec, w, h, d.focus, x, y, wheel)
	} else {
		// Click reads double only on a list row, so the screen row identifies the target.
		ev, ok = dlg.Click(spec, w, h, d.focus, x, y, a.mouse.doubleClick("dialog", y))
	}
	if !ok {
		return false, false
	}
	a.deferDiskIdleSortOnUserActivity()
	a.deferMetaIdleSortOnUserActivity()
	if ev.ID == dlg.OK || ev.ID == dlg.Cancel {
		fn := d.buttons.OK
		if ev.ID == dlg.Cancel {
			fn = d.buttons.Cancel
		}
		if fn != nil {
			fn()
			a.render()
			return false, true
		}
	}
	if d.apply(ev) {
		return true, false
	}
	a.render()
	return false, true
}

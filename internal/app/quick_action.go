package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// openQuickAction opens the buttonless quick-action list dialog with st, wiring
// onActivate (called with the activated item index once the dialog is already
// closed) and an optional onKey hook for consumer-specific extra keys (e.g. F9 to
// edit config). Only one quick-action dialog can be open at a time — opening a new
// one replaces any previous callbacks.
func (a *App) openQuickAction(st dialog.QuickActionState, onActivate func(int), onKey func(*tcell.EventKey) bool, footerExtra []menu.FunctionKey) {
	st.Open = true
	a.model.QuickAction = st
	a.quickActionOnActivate = onActivate
	a.quickActionOnKey = onKey
	a.quickActionFooterExtra = footerExtra

	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	vr := dialog.QuickActionViewportRows(layout, len(a.model.QuickAction.Items))
	dialog.QuickActionEnsureScroll(&a.model.QuickAction, vr)
}

// closeQuickAction closes the dialog and clears its callbacks.
func (a *App) closeQuickAction() {
	a.model.QuickAction = dialog.QuickActionState{}
	a.quickActionOnActivate = nil
	a.quickActionOnKey = nil
	a.quickActionFooterExtra = nil
}

// activateQuickAction closes the dialog, then runs the stored onActivate callback
// for item index i (close-then-act, same order as the old executeUserMenuEntry).
func (a *App) activateQuickAction(i int) {
	onActivate := a.quickActionOnActivate
	a.closeQuickAction()
	if onActivate != nil {
		onActivate(i)
	}
}

// handleQuickActionKey routes keys for the open quick-action dialog: the consumer's
// onKey hook first (e.g. F9), then Esc closes, Enter activates the selected row,
// Up/Down move the selection bar, and a plain letter (no Alt/Ctrl/Meta) jumps to and
// activates the matching row.
func (a *App) handleQuickActionKey(event *tcell.EventKey) {
	if a.quickActionOnKey != nil && a.quickActionOnKey(event) {
		return
	}
	st := &a.model.QuickAction
	n := len(st.Items)
	if n == 0 {
		a.closeQuickAction()
		return
	}

	switch event.Key() {
	case tcell.KeyEsc:
		a.closeQuickAction()
		return
	case tcell.KeyEnter:
		a.activateQuickAction(st.Selected)
		return
	case tcell.KeyUp:
		st.Selected = dialog.ListClampedSelectionDelta(st.Selected, n, -1)
	case tcell.KeyDown:
		st.Selected = dialog.ListClampedSelectionDelta(st.Selected, n, 1)
	case tcell.KeyRune:
		if event.Modifiers()&(tcell.ModAlt|tcell.ModCtrl|tcell.ModMeta) == 0 {
			if i, ok := dialog.QuickActionIndexForKey(st.Items, event.Rune()); ok {
				a.activateQuickAction(i)
			}
		}
		return
	default:
		return
	}

	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	vr := dialog.QuickActionViewportRows(layout, n)
	dialog.QuickActionEnsureScroll(st, vr)
}

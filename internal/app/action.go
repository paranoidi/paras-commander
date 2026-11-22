package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/app/helpkeys"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// lookupActionForView resolves an action from the view's overlay, then the global map.
func lookupActionForView(ev *tcell.EventKey, b *keymap.Bundle, vm ui.ViewMode) string {
	if ev == nil || b == nil || b.Global == nil {
		return ""
	}
	if overlay := b.OverlayFor(helpkeys.ViewMask(vm)); overlay != nil {
		if id, ok := overlay.Lookup(ev); ok {
			return id
		}
	}
	id, _ := b.Global.Lookup(ev)
	return id
}

// viewOverlayKeys returns the current view's overlay map.
func (a *App) viewOverlayKeys() *keymap.Map {
	return a.keys.OverlayFor(helpkeys.ViewMask(a.model.ViewMode))
}

// stripHasFocus reports whether a Selected strip (the file panel's or the find-duplicates one) has keyboard focus.
func (a *App) stripHasFocus() bool {
	return (a.model.ViewMode == ui.ViewBrowser && a.model.ActiveSubFocus == ui.SubFocusSelectedStrip) ||
		(a.model.ViewMode == ui.ViewDedup && a.model.DedupView.FocusSelected)
}

// selectedStripFooterKeys is the footer while a Selected strip has focus (dedup: the find-duplicates strip).
func (a *App) selectedStripFooterKeys(dedup bool) []menu.FunctionKey {
	return menu.FunctionKeysSelectedStripView(
		a.keys.Global.MenuBindingLabel(keymap.ActionPanelClearSelection),
		a.keys.SelectedStrip.MenuBindingLabel(keymap.ActionPanelSelectedToggleTree),
		a.model.SelectedStripTree(), dedup)
}

func (a *App) actionFromKeyEvent(ev *tcell.EventKey) string {
	if a == nil {
		return ""
	}
	if ev != nil && a.stripHasFocus() {
		if id, ok := a.keys.SelectedStrip.Lookup(ev); ok {
			return id
		}
	}
	id := lookupActionForView(ev, a.keys, a.model.ViewMode)
	if ev != nil && a.model.EmbeddedPanelFocused() {
		return a.embeddedPanelAction(ev, id)
	}
	return id
}

// embeddedPanelAction resolves a key while a view's embedded file-list panel has focus. The view
// keeps its close, pane and open-in-panel bindings; its bindings that shadow panel navigation
// become that navigation; any other binding of the view gives way to the browser keymap, so the
// panel takes every file-list key.
func (a *App) embeddedPanelAction(ev *tcell.EventKey, id string) string {
	switch id {
	case keymap.ActionDedupToggleNode, keymap.ActionUniqueToggleNode:
		return keymap.ActionNavOpen
	case keymap.ActionDedupCollapse, keymap.ActionUniqueCollapse:
		return keymap.ActionNavParent
	case keymap.ActionDedupExpandCopies:
		return keymap.ActionNavForward
	case keymap.ActionDedupCollapseCopies:
		return keymap.ActionNavBackward
	case keymap.ActionDedupClose, keymap.ActionDedupPrevPane, keymap.ActionUniqueClose, keymap.ActionUniquePrevPane, keymap.ActionCopiesClose, keymap.ActionCopiesPrevPane,
		keymap.ActionOpenInPrimary, keymap.ActionOpenInSecondary:
		return id
	}
	if overlay := a.viewOverlayKeys(); overlay != nil {
		if _, ok := overlay.Lookup(ev); ok {
			id, _ = a.keys.Global.Lookup(ev)
		}
	}
	return id
}

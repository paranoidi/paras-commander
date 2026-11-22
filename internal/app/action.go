package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// overlayForView picks the map that handles vm's keys before the global map (nil when vm has none).
func overlayForView(jobs, commands, messages, filePreview, compare, dedup, unique *keymap.Map, vm ui.ViewMode) *keymap.Map {
	switch vm {
	case ui.ViewCommands:
		return commands
	case ui.ViewCompare:
		return compare
	case ui.ViewDedup:
		return dedup
	case ui.ViewUnique:
		return unique
	case ui.ViewJobs:
		return jobs
	case ui.ViewMessages:
		return messages
	case ui.ViewFilePreview:
		return filePreview
	default:
		return nil
	}
}

// lookupActionForView resolves an action from global and optional view overlays.
func lookupActionForView(ev *tcell.EventKey, global, jobs, commands, messages, filePreview, compare, dedup, unique *keymap.Map, vm ui.ViewMode) string {
	if ev == nil || global == nil {
		return ""
	}
	if overlay := overlayForView(jobs, commands, messages, filePreview, compare, dedup, unique, vm); overlay != nil {
		if id, ok := overlay.Lookup(ev); ok {
			return id
		}
	}
	id, ok := global.Lookup(ev)
	if !ok {
		return ""
	}
	return id
}

// viewOverlayKeys returns the current view's overlay map.
func (a *App) viewOverlayKeys() *keymap.Map {
	return overlayForView(a.keys.Jobs, a.keys.Commands, a.keys.Messages, a.keys.FilePreview, a.keys.Compare, a.keys.Dedup, a.keys.Unique, a.model.ViewMode)
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
	id := lookupActionForView(ev, a.keys.Global, a.keys.Jobs, a.keys.Commands, a.keys.Messages, a.keys.FilePreview, a.keys.Compare, a.keys.Dedup, a.keys.Unique, a.model.ViewMode)
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
	case keymap.ActionDedupClose, keymap.ActionDedupPrevPane, keymap.ActionUniqueClose, keymap.ActionUniquePrevPane,
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

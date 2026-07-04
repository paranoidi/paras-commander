package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// lookupActionForView resolves an action from global and optional view overlays.
func lookupActionForView(ev *tcell.EventKey, global, jobs, commands, messages, filePreview, compare, dedup *keymap.Map, vm ui.ViewMode) string {
	if ev == nil || global == nil {
		return ""
	}
	switch vm {
	case ui.ViewCommands:
		if commands != nil {
			if id, ok := commands.Lookup(ev); ok {
				return id
			}
		}
	case ui.ViewCompare:
		if compare != nil {
			if id, ok := compare.Lookup(ev); ok {
				return id
			}
		}
	case ui.ViewDedup:
		if dedup != nil {
			if id, ok := dedup.Lookup(ev); ok {
				return id
			}
		}
	case ui.ViewJobs:
		if jobs != nil {
			if id, ok := jobs.Lookup(ev); ok {
				return id
			}
		}
	case ui.ViewMessages:
		if messages != nil {
			if id, ok := messages.Lookup(ev); ok {
				return id
			}
		}
	case ui.ViewFilePreview:
		if filePreview != nil {
			if id, ok := filePreview.Lookup(ev); ok {
				return id
			}
		}
	}
	id, ok := global.Lookup(ev)
	if !ok {
		return ""
	}
	return id
}

func (a *App) actionFromKeyEvent(ev *tcell.EventKey) string {
	if a == nil {
		return ""
	}
	return lookupActionForView(ev, a.keys, a.keysJobs, a.keysCommands, a.keysMessages, a.keysFilePreview, a.keysCompare, a.keysDedup, a.model.ViewMode)
}

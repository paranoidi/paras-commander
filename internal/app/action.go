package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
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

func (a *App) actionFromKeyEvent(ev *tcell.EventKey) string {
	if a == nil {
		return ""
	}
	return lookupActionForView(ev, a.keys.Global, a.keys.Jobs, a.keys.Commands, a.keys.Messages, a.keys.FilePreview, a.keys.Compare, a.keys.Dedup, a.keys.Unique, a.model.ViewMode)
}

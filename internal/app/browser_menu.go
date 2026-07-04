package app

import "github.com/paranoidi/paras-commander/internal/ui/menu"

func (a *App) browserMenuDefinitions() []menu.Definition {
	return menu.BrowserDefinitions(a.keys, a.devMode)
}

func (a *App) dedupMenuDefinitions() []menu.Definition {
	return menu.DedupDefinitions(a.keys, a.keysDedup)
}

package app

import (
	"context"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/foldercolor"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// folderColorsPayload wakes the main loop after the folder-colour snapshot was replaced.
type folderColorsPayload struct{ err error } // err: a failed Apply, reported on the UI thread

// runFolderColorRefresh loads the folder colours, follows the icon theme (WatchIconTheme, stopped
// with the loop), then re-checks the GVfs metadata every config.FolderColorRefreshInterval and
// reloads it when another program changed it. Started from Run, not the constructor, so tests
// keep an empty event queue. Each reload posts folderColorsPayload so the panels repaint;
// painting itself only reads the in-memory snapshot.
func (a *App) runFolderColorRefresh(stop <-chan struct{}) {
	a.reloadFolderColors(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.folderColors.WatchIconTheme(ctx, func() {
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(folderColorsPayload{}))
	})
	ticker := time.NewTicker(config.FolderColorRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if a.folderColors.Refresh() {
				_ = a.screen.PostEvent(tcell.NewEventInterrupt(folderColorsPayload{}))
			}
		}
	}
}

// openFolderColorMenu opens a strip listing the current icon-theme style's colours plus Default
// (clear). It applies to the panel's selected directories, else the directory under the cursor.
func (a *App) openFolderColorMenu(panelID int) {
	p := a.panelByID(panelID)
	if p == nil {
		return
	}
	if !p.Path.IsLocal() {
		a.setTransientMessage("Folder color: not available on remote panels", ui.MessageUrgencyWarn)
		return
	}
	dirs := p.SelectedDirectoryPaths()
	if len(dirs) == 0 {
		if e, ok := p.CurrentEntry(); ok && e.Type == localfs.EntryDirectory && e.Name != ".." {
			dirs = []string{e.Path}
		}
	}
	if len(dirs) == 0 {
		a.setTransientMessage("Folder color: no directory selected", ui.MessageUrgencyWarn)
		return
	}
	choices := a.folderColors.Choices()
	if len(choices) == 0 {
		a.setTransientMessage("Folder color: no folder-color-switcher palette found", ui.MessageUrgencyWarn)
		return
	}
	items := make([]ui.LeaderMenuItem, 0, len(choices)+1)
	for _, c := range choices {
		items = append(items, ui.LeaderMenuItem{Label: c.Name})
	}
	items = append(items, ui.LeaderMenuItem{Label: "Default"})
	// The bool return is the app-quit signal, not a success flag (see toggleGitFilterMenu).
	a.openLeaderMenuStrip(items, ui.LeaderMenuFolderColor, "Folder color", func(i int) bool {
		var choice *foldercolor.Choice // nil = the trailing Default row
		if i < len(choices) {
			choice = &choices[i]
		}
		a.applyFolderColor(dirs, choice)
		return false
	})
}

// applyFolderColor runs foldercolor.Apply off the UI thread, then reloads the snapshot and
// repaints so the change shows immediately.
func (a *App) applyFolderColor(dirs []string, c *foldercolor.Choice) {
	go func() {
		a.reloadFolderColors(a.folderColors.Apply(dirs, c))
	}()
}

// reloadFolderColors re-reads the snapshot and wakes the main loop to repaint (err: a failed Apply).
func (a *App) reloadFolderColors(err error) {
	a.folderColors.Reload()
	_ = a.screen.PostEvent(tcell.NewEventInterrupt(folderColorsPayload{err: err}))
}

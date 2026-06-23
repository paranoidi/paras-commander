package app

import (
	"context"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

type gitStatusPayload struct {
	panelID int
	gen     uint64
	listDir string
	byPath  map[string]gitstatus.Cell
	err     error
}

func (a *App) wireGitStatusLoaders() {
	a.model.Primary.ScheduleGitStatus = a.gitStatusScheduler(ui.PrimaryPanel)
	a.model.Secondary.ScheduleGitStatus = a.gitStatusScheduler(ui.SecondaryPanel)
}

func (a *App) gitStatusScheduler(panelID int) panel.GitStatusScheduler {
	return func(req panel.GitStatusRequest) bool {
		if a.gitStatusCache == nil {
			return false
		}
		gen := a.gitStatusLoadGen[panelID].Add(1)
		listDir := req.ListDir
		paths := append([]gitstatus.ListingPaths(nil), req.Paths...)
		workRoot := req.WorkRoot
		go func() {
			byPath, err := a.gitStatusCache.StatusesForListing(context.Background(), workRoot, listDir, paths)
			_ = a.screen.PostEvent(tcell.NewEventInterrupt(gitStatusPayload{
				panelID: panelID,
				gen:     gen,
				listDir: listDir,
				byPath:  byPath,
				err:     err,
			}))
		}()
		return true
	}
}

func (a *App) applyGitStatusLoad(p gitStatusPayload) bool {
	if a.gitStatusLoadGen[p.panelID].Load() != p.gen {
		return false
	}
	pan := a.panelByID(p.panelID)
	if pan == nil {
		return false
	}
	host, err := pan.Path.FilePath()
	if err != nil || filepath.Clean(host) != filepath.Clean(p.listDir) {
		return false
	}
	pan.GitPending = false
	if p.err != nil {
		pan.GitByPath = nil
		return true
	}
	pan.GitByPath = p.byPath
	return true
}

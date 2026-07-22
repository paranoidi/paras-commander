package app

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

type gitStatusPayload struct {
	panelID  int
	gen      uint64
	cwdLevel bool
	listDir  string
	byPath   map[string]gitstatus.Cell
	err      error
}

func (a *App) wireGitStatusLoaders() {
	a.model.Primary.ScheduleGitStatus = a.gitStatusScheduler(ui.PrimaryPanel)
	a.model.Secondary.ScheduleGitStatus = a.gitStatusScheduler(ui.SecondaryPanel)
}

// gitStatusScheduler dispatches an async git status fetch for req.ListDir. Two kinds of request
// share this scheduler: the cwd-level fetch from panel.State.prepareGitColumn/
// RescheduleGitStatusIfNeeded (req.ListDir == the panel's current directory), and, in tree mode,
// per-directory fetches for newly-expanded children (req.ListDir a subdirectory of it) from
// panel.State.scheduleTreeChildGitStatus. Only the cwd-level kind uses gitStatusLoadGen — a
// single shared counter per panel — to detect staleness (a chdir/refresh superseding an
// in-flight fetch for the old directory); gating tree-child fetches on that same counter would
// make an unrelated cwd/tree-child dispatch (each bumps a shared counter) spuriously invalidate
// every other in-flight fetch. Tree-child fetches don't need their own generation tracking: the
// panel layer only ever dispatches one per directory per tree "session" (setTreeNodeExpanded's
// Loading guard + Children-cached-once-loaded), so no duplicate/overlapping dispatch is possible
// for the same directory.
func (a *App) gitStatusScheduler(panelID int) panel.GitStatusScheduler {
	return func(req panel.GitStatusRequest) bool {
		if a.gitStatusCache == nil {
			return false
		}
		pan := a.panelByID(panelID)
		if pan == nil {
			return false
		}
		listDir := filepath.Clean(req.ListDir)
		host, hostErr := pan.Path.FilePath()
		cwdLevel := hostErr == nil && filepath.Clean(host) == listDir
		var gen uint64
		if cwdLevel {
			gen = a.gitStatusLoadGen[panelID].Add(1)
		}
		paths := append([]gitstatus.ListingPaths(nil), req.Paths...)
		workRoot := req.WorkRoot
		go func() {
			byPath, err := a.gitStatusCache.StatusesForListing(context.Background(), workRoot, listDir, paths)
			_ = a.screen.PostEvent(tcell.NewEventInterrupt(gitStatusPayload{
				panelID:  panelID,
				gen:      gen,
				cwdLevel: cwdLevel,
				listDir:  listDir,
				byPath:   byPath,
				err:      err,
			}))
		}()
		return true
	}
}

// applyGitStatusLoad merges an async git status result into pan.GitByPath — a global path-keyed
// cache that rendering looks up per row regardless of flat/tree source, so results for the cwd
// listing and for tree-mode expanded children accumulate rather than overwrite each other. The
// cwd-level result still fully replaces GitByPath (matching pre-tree-mode behavior: a Refresh
// should drop stale cells for files that disappeared), which is safe because prepareGitColumn
// already reset GitByPath to nil synchronously when it dispatched that fetch.
func (a *App) applyGitStatusLoad(p gitStatusPayload) bool {
	pan := a.panelByID(p.panelID)
	if pan == nil {
		return false
	}
	host, err := pan.Path.FilePath()
	if err != nil {
		return false
	}
	host = filepath.Clean(host)
	if p.cwdLevel {
		if a.gitStatusLoadGen[p.panelID].Load() != p.gen || p.listDir != host {
			return false
		}
		pan.GitPending = false
		if p.err != nil {
			pan.GitByPath = nil
			return true
		}
	} else {
		if !pan.GitColumnActive || !isWithinDir(p.listDir, host) {
			return false
		}
		if p.err != nil {
			// Leave any already-merged data (cwd + other expanded children) untouched.
			return true
		}
	}
	if pan.GitByPath == nil {
		pan.GitByPath = make(map[string]gitstatus.Cell, len(p.byPath))
	}
	for path, cell := range p.byPath {
		pan.GitByPath[path] = cell
	}
	return true
}

// quickViewGitStatusPayload is the async git status result for the QuickViewDirOverlay listing
// (populateQuickViewDirOverlay's fresh-snapshot path). It is tracked separately from
// gitStatusPayload because the overlay is not one of the two real panels panelByID resolves.
type quickViewGitStatusPayload struct {
	gen     uint64
	listDir string
	byPath  map[string]gitstatus.Cell
	err     error
}

// quickViewGitStatusScheduler dispatches an async git status fetch for the QuickViewDirOverlay.
// Every populateQuickViewDirOverlay call rebuilds the overlay from scratch (a single listing, no
// tree-mode children), so unlike gitStatusScheduler this always tracks a fresh generation.
func (a *App) quickViewGitStatusScheduler() panel.GitStatusScheduler {
	return func(req panel.GitStatusRequest) bool {
		if a.gitStatusCache == nil {
			return false
		}
		listDir := filepath.Clean(req.ListDir)
		gen := a.quickViewGitLoadGen.Add(1)
		paths := append([]gitstatus.ListingPaths(nil), req.Paths...)
		workRoot := req.WorkRoot
		go func() {
			byPath, err := a.gitStatusCache.StatusesForListing(context.Background(), workRoot, listDir, paths)
			_ = a.screen.PostEvent(tcell.NewEventInterrupt(quickViewGitStatusPayload{
				gen:     gen,
				listDir: listDir,
				byPath:  byPath,
				err:     err,
			}))
		}()
		return true
	}
}

// applyQuickViewGitStatusLoad merges an async git status result into the QuickViewDirOverlay,
// dropping it if the overlay has since been cleared or repopulated for another directory.
func (a *App) applyQuickViewGitStatusLoad(p quickViewGitStatusPayload) bool {
	if !a.model.QuickViewDirOverlayActive || a.quickViewGitLoadGen.Load() != p.gen {
		return false
	}
	ov := &a.model.QuickViewDirOverlay
	host, err := ov.Path.FilePath()
	if err != nil || filepath.Clean(host) != p.listDir {
		return false
	}
	ov.GitPending = false
	if p.err != nil {
		ov.GitByPath = nil
		return true
	}
	ov.GitByPath = p.byPath
	return true
}

// isWithinDir reports whether child is parent or a descendant of parent. Both paths must
// already be filepath.Clean'd.
func isWithinDir(child, parent string) bool {
	if parent == "" {
		return false
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

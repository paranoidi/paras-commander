package fileindex

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	fi "github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// errMixedSelection refuses a selection of both files and directories: the scope is either the
// selected directories or the panel directory, and a mix does not say which.
var errMixedSelection = errors.New("selection mixes files and directories")

const mixedSelectionMessage = dialog.FindUniqueTitle + ": select only directories or only files"

// UniquePayload carries the outcome of the one index lookup behind a Find unique files dialog;
// ScanGen says which dialog it answers (the query's Gen moves on while the lookup runs).
type UniquePayload struct {
	ScanGen int
	Result  fi.UniqueResult
	Err     error
}

// FindUnique opens Find unique files on the active panel.
func (h *Handler) FindUnique() {
	h.findUniqueOn(h.model.ActivePanel)
}

// OpenFreshUnique drops any Find in index (open or parked) and starts Find unique files on panelID.
func (h *Handler) OpenFreshUnique(panelID int) {
	h.CloseFindDialog()
	h.findUniqueOn(panelID)
}

func (h *Handler) findUniqueOn(panelID int) {
	if ui.IsAuxiliaryView(h.model.ViewMode) {
		return
	}
	p := h.host.PanelByID(panelID)
	root, scope, err := h.uniqueScope(p)
	switch {
	case errors.Is(err, errMixedSelection):
		h.host.SetTransientMessage(mixedSelectionMessage, ui.MessageUrgencyCritical)
		return
	case err != nil:
		h.host.SetErrorMessage(dialog.FindUniqueTitle, err)
		return
	case slices.Contains(scope, "/"):
		h.host.SetTransientMessage(indexRootNothingOutside, ui.MessageUrgencyWarn)
		return
	}
	if st := &h.model.IndexFindDialog; st.Parked {
		if st.UniqueMode && st.PanelID == panelID && st.Root == root && slices.Equal(st.SelectionDirRoots, scope) {
			h.RestoreParkedFind()
			return
		}
		h.model.FindRelocate = dialog.FindRelocateState{Open: true, Focus: dialog.FindRelocateRestore, Index: true, Unique: true, PanelID: panelID, Root: st.Root}
		return
	}
	h.CloseFindDialog()
	h.model.IndexFindDialog = dialog.IndexFindDialogState{
		Gen:  h.model.IndexFindDialog.Gen, // keeps counting up, so a result of an earlier dialog is stale
		Open: true, PanelID: panelID, FromIndexPanel: p.Path.Scheme() == pathloc.SchemeFileIndex, Root: root, Focus: dialog.IndexFindQuery,
		UniqueMode: true, SelectionDirRoots: scope, IncludeHidden: true, Searching: true,
	}
	h.uniqueScanning = true
	gen := h.uniqueGen
	go func() {
		res, err := h.index.UniqueFiles(scope)
		h.post(UniquePayload{ScanGen: gen, Result: res, Err: err})
	}()
}

// uniqueScope is the panel directory as the index records it and the directories to check: the
// selected directories, else the panel directory (a selection of only files does not change it).
func (h *Handler) uniqueScope(p *panel.State) (root string, scope []string, err error) {
	if root, err = fi.IndexPath(p.PathString()); err != nil {
		return "", nil, err
	}
	entries, err := p.SelectedEntries(true, h.selectedEntry)
	if err != nil {
		return "", nil, err
	}
	var dirs []string
	files := 0
	for _, e := range entries {
		if !e.ResolvesToDir() {
			files++
			continue
		}
		host, err := fi.IndexPath(e.Path)
		if err != nil {
			return "", nil, err
		}
		dirs = append(dirs, host)
	}
	switch {
	case len(dirs) > 0 && files > 0:
		return "", nil, errMixedSelection
	case len(dirs) > 0:
		return root, panel.PruneNestedPaths(dirs), nil
	}
	return root, []string{root}, nil
}

// selectedEntry resolves a selection off the panel listing: a fileindex:// path from the index,
// where it may no longer exist on disk.
func (h *Handler) selectedEntry(path string) (localfs.Entry, error) {
	loc, err := pathloc.Parse(path)
	if err != nil {
		return localfs.Entry{}, err
	}
	if loc.Scheme() != pathloc.SchemeFileIndex {
		return localfs.EntryFromPath(path)
	}
	e, err := fi.NewBackend(h.index).Stat(context.Background(), loc)
	if err != nil {
		return localfs.Entry{}, err
	}
	return localfs.Entry{Name: loc.Base(), Path: path, Type: fsbackend.LocalTypeFromBackend(e.Type)}, nil
}

// HandleUnique applies the finished lookup to the dialog it answers, then shows it through the
// current query.
func (h *Handler) HandleUnique(p UniquePayload) {
	st := &h.model.IndexFindDialog
	if p.ScanGen != h.uniqueGen || !st.UniqueMode {
		return
	}
	h.uniqueScanning = false
	if p.Err != nil {
		h.CloseFindDialog()
		h.host.SetErrorMessage(dialog.FindUniqueTitle, p.Err)
		return
	}
	h.unique = p.Result
	if len(p.Result.NotIndexed) > 0 {
		st.Note = "Not in the index: " + strings.Join(p.Result.NotIndexed, ", ")
	}
	h.scheduleFind(0)
}

// scheduleUniqueFilter narrows the loaded list by the dialog's query after delay on a goroutine;
// the lookup still running does it once it lands. The list is not written after it landed.
func (h *Handler) scheduleUniqueFilter(delay time.Duration) {
	st := &h.model.IndexFindDialog
	st.Searching = true
	if h.uniqueScanning {
		return
	}
	gen, root, limit, all := st.Gen, st.Root, h.config.UI.Find.MaxResults, h.unique
	q := fi.FindQuery{Text: st.Query, MaxDepth: st.MaxDepth, IncludeHidden: st.IncludeHidden}
	h.findTimer = time.AfterFunc(delay, func() {
		found, truncated := fi.FilterUnique(all.Files, root, q, limit)
		res := make([]dialog.IndexFindResult, len(found))
		unverified := 0
		for i, f := range found {
			flagged := all.Unverified[f.Path]
			if flagged {
				unverified++
			}
			res[i] = dialog.IndexFindResult{Path: f.Path, Rel: relTo(root, f.Path), Size: f.Size, Unverified: flagged}
		}
		h.post(FindPayload{Gen: gen, Results: res, Truncated: truncated, Unverified: unverified})
	})
}

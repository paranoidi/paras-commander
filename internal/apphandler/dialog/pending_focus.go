package dialog

import (
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// schedulePanelFocus defers select-and-center on name until the listing at listDir actually
// contains it. Used by duplicate: AddTransferJob only enqueues the copy, so the duplicated
// file doesn't exist on disk (or in the panel's Entries) at the point this is scheduled, and it
// can't be tied to one specific reload the way rename/mkdir's RefreshBothPanelsWithFocus hook can
// — it lands later, off of the job's own terminal-event refresh (jobs.Handler's ApplyRefreshes).
// applyPendingPanelFocus (called from App.reconcileAfterEvent after every event, including that
// later refresh) retries until it either succeeds or the panel has moved on to a different directory.
func (h *Handler) schedulePanelFocus(panelID int, listDir, name string) {
	h.pendingFocus = pendingPanelFocus{
		panelID: panelID,
		listDir: listDir,
		name:    name,
		center:  true,
	}
}

// scheduleTransferOtherPanelFocus defers select-and-scroll onto the first candidate that appears
// in the destination panel's visible listing order. snapCursorPath must still match CurrentEntry
// when applying; an empty snap skips that guard (empty panel at schedule time).
func (h *Handler) scheduleTransferOtherPanelFocus(panelID int, listDir string, candidates []string, snapCursorPath string) {
	h.pendingFocus = pendingPanelFocus{
		panelID:        panelID,
		listDir:        listDir,
		candidateNames: append([]string(nil), candidates...),
		snapCursorPath: snapCursorPath,
		center:         false,
	}
}

// maybeScheduleTransferOtherPanelFocus arms pending focus when a copy/move lands in the inactive
// panel's current directory and [operations].focus_other_panel_after_transfer is enabled.
// Called after jobs.AddTransferJob so optimistic listing has already updated the inactive cursor.
func (h *Handler) maybeScheduleTransferOtherPanelFocus(jobType jobs.Type, sources []string, dest string, preserve jobs.TransferPreserve) {
	if jobType != jobs.TypeCopy && jobType != jobs.TypeMove {
		return
	}
	if !h.host.Config().Operations.FocusOtherPanelAfterTransfer {
		return
	}
	destLoc, err := pathloc.Parse(dest)
	if err != nil {
		return
	}
	destDir := destLoc
	if !ops.DestinationIsDirAtEnqueue(destLoc) {
		destDir = destLoc.Parent()
	}
	inactive := h.host.InactivePanel()
	if inactive == nil || inactive.Path.IsZero() || !destDir.Equal(inactive.Path) {
		return
	}
	candidates := transferTopLevelDestNames(sources, dest, preserve.FlattenIntoDest)
	if len(candidates) == 0 {
		return
	}
	snap := ""
	if e, ok := inactive.CurrentEntry(); ok {
		snap = e.Path
	}
	h.scheduleTransferOtherPanelFocus(h.host.InactivePanelID(), inactive.PathString(), candidates, snap)
}

// applyPendingPanelFocus retries the deferred select scheduled by schedulePanelFocus /
// scheduleTransferOtherPanelFocus. A no-op once nothing is pending, once the target panel has
// navigated elsewhere, once the cursor snap no longer matches, or once the entry is selected.
func (h *Handler) applyPendingPanelFocus() {
	f := h.pendingFocus
	if f.name == "" && len(f.candidateNames) == 0 {
		return
	}
	p := h.host.PanelByID(f.panelID)
	if p == nil {
		h.pendingFocus = pendingPanelFocus{}
		return
	}
	if filepath.Clean(p.PathString()) != filepath.Clean(f.listDir) {
		h.pendingFocus = pendingPanelFocus{}
		return
	}
	if f.snapCursorPath != "" {
		cur, ok := p.CurrentEntry()
		if !ok || filepath.Clean(cur.Path) != filepath.Clean(f.snapCursorPath) {
			h.pendingFocus = pendingPanelFocus{}
			return
		}
	}
	name := f.name
	if name == "" {
		name = firstVisibleCandidate(p, f.candidateNames)
		if name == "" {
			return
		}
	}
	vr := h.host.PanelViewportRows(f.panelID)
	var ok bool
	if f.center {
		ok = p.SelectVisibleEntryCentered(name, vr)
	} else {
		ok = p.SelectVisibleEntryInViewport(name, vr)
	}
	if ok {
		h.pendingFocus = pendingPanelFocus{}
	}
}

// firstVisibleCandidate returns the first name in p's visible listing order that is in candidates.
func firstVisibleCandidate(p *panel.State, candidates []string) string {
	want := make(map[string]bool, len(candidates))
	for _, n := range candidates {
		if n != "" {
			want[n] = true
		}
	}
	if len(want) == 0 {
		return ""
	}
	for i := 0; i < p.VisibleEntryCount(); i++ {
		e, _, ok := p.VisibleEntry(i)
		if !ok {
			continue
		}
		if want[e.Name] {
			return e.Name
		}
	}
	return ""
}

// transferTopLevelDestNames returns the basenames (or first path segments for structured
// transfers) that will appear as rows in the destination directory listing.
func transferTopLevelDestNames(sources []string, dest string, flatten bool) []string {
	srcLocs, err := pathloc.ParseAll(sources)
	if err != nil || len(srcLocs) == 0 {
		return nil
	}
	destLoc, err := pathloc.Parse(dest)
	if err != nil {
		return nil
	}
	if !ops.DestinationIsDirAtEnqueue(destLoc) {
		base := destLoc.Base()
		if base == "" {
			return nil
		}
		return []string{base}
	}
	nameRoot := ops.TransferNameRoot(srcLocs)
	if flatten {
		nameRoot = pathloc.Path{}
	}
	seen := make(map[string]bool, len(srcLocs))
	names := make([]string, 0, len(srcLocs))
	for _, src := range srcLocs {
		top := topLevelPathSegment(ops.TransferDestName(src, nameRoot))
		if top == "" || seen[top] {
			continue
		}
		seen[top] = true
		names = append(names, top)
	}
	return names
}

func topLevelPathSegment(rel string) string {
	rel = strings.Trim(rel, `/\`)
	if rel == "" {
		return ""
	}
	if i := strings.IndexAny(rel, `/\`); i >= 0 {
		return rel[:i]
	}
	return rel
}

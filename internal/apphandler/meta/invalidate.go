package meta

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/fschange"
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// goneListing is the listing state a panel's gone set was last checked against.
type goneListing struct {
	epoch, treeGen uint64
}

// fsReport is the last thing a batch of filesystem changes said about one path.
type fsReport struct {
	seq  int
	gone bool
	dir  bool
}

// fsReports holds a batch's reports by host path. A later report of a path replaces an earlier
// one, so a delete followed by a recreate reads as changed.
type fsReports map[string]fsReport

func newFSReports(batch []fschange.Change) fsReports {
	r := make(fsReports, len(batch))
	seq := 0
	add := func(path string, gone, dir bool) {
		seq++
		r[path] = fsReport{seq: seq, gone: gone, dir: dir}
	}
	for _, c := range batch {
		switch c.Kind {
		case fschange.Removed:
			add(c.Path, true, c.IsDir)
		case fschange.Renamed:
			add(c.From, true, c.IsDir)
			add(c.Path, false, c.IsDir)
		case fschange.Created, fschange.Dirty:
			add(c.Path, false, c.IsDir)
		}
	}
	return r
}

// lookup returns the latest report covering a row path: its own, or that of a directory above it.
// A row under a reported directory counts as gone: only a new listing can say it still exists.
// Rows of fileindex:// panels are matched by their host path, which is what changes report.
func (r fsReports) lookup(rowPath string) (rep fsReport, ok bool) {
	host := strings.TrimPrefix(rowPath, pathloc.FileIndexPrefix)
	rep, ok = r[host]
	for dir := filepath.Dir(host); ; dir = filepath.Dir(dir) {
		if up, found := r[dir]; found && up.dir && (!ok || up.seq > rep.seq) {
			rep, ok = up, true
			rep.gone = true
		}
		if dir == filepath.Dir(dir) {
			return rep, ok
		}
	}
}

// ApplyFSChanges invalidates the meta values of the paths a batch of filesystem changes touched,
// and nothing else (ADR 0023). A changed path's row is recomputed by the next ReconcileForPanel. A
// gone path's row keeps its cells until the listing loses it (pruneGone), because a row still
// listed with no cell would run its commands on a path that no longer exists.
func (h *Handler) ApplyFSChanges(batch []fschange.Change) {
	if len(batch) == 0 {
		return
	}
	reports := newFSReports(batch)
	h.cacheMu.Lock()
	for _, values := range h.cache {
		for path := range values {
			if _, ok := reports.lookup(path); ok {
				delete(values, path)
			}
		}
	}
	h.cacheMu.Unlock()
	for panelID := range h.model.MetaResults {
		h.invalidateRows(panelID, reports)
	}
}

// invalidateRows applies reports to panelID's cells. A changed row with a command still running is
// left alone: its result is on the way, and running it again would count the row pending twice
// against one decrement in applyWakeResult.
func (h *Handler) invalidateRows(panelID int, reports fsReports) {
	cols := h.model.MetaResults[panelID]
	if len(cols) == 0 || cols[0].Results == nil {
		return
	}
	touched := false
	for path := range cols[0].Results {
		rep, ok := reports.lookup(path)
		if !ok {
			continue
		}
		if rep.gone {
			if h.gone[panelID] == nil {
				h.gone[panelID] = make(map[string]struct{})
			}
			h.gone[panelID][path] = struct{}{}
			continue
		}
		delete(h.gone[panelID], path)
		if rowPending(h, panelID, path) {
			continue
		}
		dropRow(h, panelID, path)
		touched = true
	}
	h.goneDirty[panelID] = true
	if touched {
		h.afterRowsDropped(panelID)
	}
}

// pruneGone drops the cells of gone rows the listing no longer holds, so a path reappearing later
// is computed fresh. It looks at the listing only after the listing changed or new rows were
// reported, so it stays cheap while a gone row waits for its reload.
func (h *Handler) pruneGone(panelID int, p *panel.State) {
	if len(h.gone[panelID]) == 0 {
		return
	}
	listing := goneListing{epoch: p.ListingEpoch, treeGen: p.TreeChildrenGen}
	if !h.goneDirty[panelID] && listing == h.goneChecked[panelID] {
		return
	}
	h.goneDirty[panelID] = false
	h.goneChecked[panelID] = listing

	listed := make(map[string]struct{}, len(p.Entries))
	for _, g := range panelGroups(p) {
		for _, e := range g.entries {
			listed[e.Path] = struct{}{}
		}
	}
	touched := false
	for path := range h.gone[panelID] {
		if _, ok := listed[path]; ok {
			continue
		}
		delete(h.gone[panelID], path)
		dropRow(h, panelID, path)
		touched = true
	}
	if touched {
		h.afterRowsDropped(panelID)
	}
}

// rowPending reports whether any column of the row still has a command running.
func rowPending(h *Handler, panelID int, path string) bool {
	for _, c := range h.model.MetaResults[panelID] {
		if c.Pending != "" && c.Results[path] == c.Pending {
			return true
		}
	}
	return false
}

// dropRow removes the row's cells from every column. A pending cell gives up its count here, since
// its result finds no marker to replace when it arrives.
func dropRow(h *Handler, panelID int, path string) {
	cols := h.model.MetaResults[panelID]
	for i := range cols {
		if cols[i].Pending != "" && cols[i].Results[path] == cols[i].Pending {
			cols[i].PendingCount--
		}
		delete(cols[i].Results, path)
	}
}

// afterRowsDropped gets dropped rows' values out of the column layout.
func (h *Handler) afterRowsDropped(panelID int) {
	cols := h.model.MetaResults[panelID]
	for i := range cols {
		cols[i].ResultsGen++
	}
	if pendingCount(cols) == 0 {
		h.pendingReveal[panelID].Stop()
	}
	h.scheduleRenderFlush(panelID)
}

// RefreshProviders re-runs panelID's Go column providers after the index changed. Their values can
// change on any row (removing one copy changes the count of the others), but one provider call is
// a query per directory, and shell columns do not read the index, so those keep their values. Cells
// hold their value until the new one lands, without pending markers. A panel with no live run has
// nothing to refresh: a run being set up reads the index when it starts.
func (h *Handler) RefreshProviders(panelID int) {
	r := h.run[panelID]
	if r == nil || r.ctx.Err() != nil || h.loadPending[panelID] {
		return
	}
	var providers []*metacmds.Provider
	for _, def := range r.defs {
		if def.Provider != nil {
			providers = append(providers, def.Provider)
		}
	}
	p := h.host.PanelByID(panelID)
	if len(providers) == 0 || p == nil {
		return
	}
	h.stopProviderRefresh(panelID)
	ctx, cancel := context.WithCancel(r.ctx)
	h.providerCancel[panelID] = cancel
	groups := panelGroups(p)
	gen := h.runGen[panelID]
	for _, provider := range providers {
		go h.runProvider(ctx, panelID, gen, provider, groups)
	}
}

// stopProviderRefresh cancels panelID's RefreshProviders still in flight, if any. Only a refresh
// runs under this cancel, so a run's own provider calls still resolve their pending cells.
func (h *Handler) stopProviderRefresh(panelID int) {
	if cancel := h.providerCancel[panelID]; cancel != nil {
		cancel()
		h.providerCancel[panelID] = nil
	}
}

package meta

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdmacro"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobbridge"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func (h *Handler) postResult(panelID int, entryName, path, value string, gen uint64) {
	_ = h.screen.PostEvent(tcell.NewEventInterrupt(WakePayload{
		PanelID:   panelID,
		EntryName: entryName,
		Path:      path,
		Value:     value,
		Gen:       gen,
	}))
}

func (h *Handler) postExecFailed(panelID int, gen uint64, exitCode int, stderr, confCmd, expandedCmd string) {
	_ = h.screen.PostEvent(tcell.NewEventInterrupt(ExecFailedPayload{
		PanelID:     panelID,
		Gen:         gen,
		ExitCode:    exitCode,
		Stderr:      stderr,
		ConfCmd:     confCmd,
		ExpandedCmd: expandedCmd,
	}))
}

// HandleWake applies one async command result (or one provider batch) and schedules a paced repaint.
// Called from Run's interrupt switch for WakePayload.
func (h *Handler) HandleWake(d WakePayload) {
	if d.Gen == h.runGen[d.PanelID] {
		if d.Values != nil {
			for path, v := range d.Values {
				// A provider works from a snapshot of the listing: a row dropped since then stays dropped.
				if !h.rowExists(d.PanelID, d.EntryName, path) {
					continue
				}
				d.Path, d.Value = path, v
				h.applyWakeResult(d)
			}
		} else {
			h.applyWakeResult(d)
		}
		if p := h.host.PanelByID(d.PanelID); p != nil && p.Sort.Mode == panel.SortMeta && p.Sort.MetaColumn == d.EntryName {
			h.resortPending[d.PanelID] = true
		}
		if pendingCount(h.model.MetaResults[d.PanelID]) == 0 {
			h.pendingReveal[d.PanelID].Stop()
		}
		h.scheduleRenderFlush(d.PanelID)
	}
}

// rowExists reports whether the named column holds a cell for path.
func (h *Handler) rowExists(panelID int, entryName, path string) bool {
	for _, c := range h.model.MetaResults[panelID] {
		if c.EntryName == entryName {
			_, ok := c.Results[path]
			return ok
		}
	}
	return false
}

func (h *Handler) applyWakeResult(d WakePayload) {
	cols := h.model.MetaResults[d.PanelID]
	for i := range cols {
		if cols[i].EntryName != d.EntryName {
			continue
		}
		if cols[i].Results == nil {
			cols[i].Results = make(map[string]string)
		}
		old, hadOld := cols[i].Results[d.Path]
		cols[i].Results[d.Path] = d.Value
		cols[i].ResultsGen++
		if hadOld && cols[i].Pending != "" && old == cols[i].Pending && d.Value != cols[i].Pending {
			cols[i].PendingCount--
		}
		return
	}
}

// HandleExecFailed reports a shell command failure to the messages log. Called from Run's
// interrupt switch for ExecFailedPayload.
func (h *Handler) HandleExecFailed(d ExecFailedPayload) {
	if d.Gen != h.runGen[d.PanelID] {
		return
	}
	const urgency = ui.MessageUrgencyWarn
	banner := "meta: command failed to execute"
	if d.Provider != "" {
		banner = "meta: " + d.Provider + " failed"
	}

	wrapCols := h.host.MessageLogWrapCols()

	lines := []string{banner}

	if d.ExpandedCmd != "" {
		lines = append(lines, fmt.Sprintf("  exit: %d", d.ExitCode))
	}

	if d.Stderr != "" {
		wrapped := ui.WrapTextLines(d.Stderr, wrapCols-10)
		for i, l := range wrapped {
			if i == 0 {
				lines = append(lines, "  stderr: "+l)
			} else {
				lines = append(lines, "          "+l)
			}
		}
	}

	if d.ConfCmd != "" {
		wrapped := ui.WrapTextLines(d.ConfCmd, wrapCols-8)
		for i, l := range wrapped {
			if i == 0 {
				lines = append(lines, "  conf: "+l)
			} else {
				lines = append(lines, "        "+l)
			}
		}
	}

	if d.ExpandedCmd != "" {
		wrapped := ui.WrapTextLines(d.ExpandedCmd, wrapCols-7)
		for i, l := range wrapped {
			if i == 0 {
				lines = append(lines, "  cmd: "+l)
			} else {
				lines = append(lines, "       "+l)
			}
		}
	}

	h.host.AppendTransientMessageLines(banner, lines, urgency)
}

type loadedMetaFile struct {
	MF      *metacmds.MetaFile
	Warns   []string
	LoadErr string
}

// loadMetaPath reads and decodes a resolved meta.toml (a seam for tests that block the read).
var loadMetaPath = metacmds.LoadFile

// fetchMetaFile loads the panel's meta.toml and adds the Go column providers, which stay
// available when there is no meta.toml or it failed to load (MF then holds only them).
func (h *Handler) fetchMetaFile(panelPath string) loadedMetaFile {
	res := h.fetchMetaTOML(panelPath)
	res.MF = metacmds.WithProviders(res.MF, h.providers)
	return res
}

func (h *Handler) fetchMetaTOML(panelPath string) loadedMetaFile {
	path, warns := metacmds.ResolveMetaTOML(h.config, h.model.UserHomeDir, h.configDir, panelPath)
	if path == "" {
		return loadedMetaFile{Warns: warns}
	}
	mf, err := loadMetaPath(path)
	if err != nil {
		return loadedMetaFile{Warns: warns, LoadErr: "meta: " + err.Error()}
	}
	return loadedMetaFile{MF: mf, Warns: warns}
}

// loadMetaFile resolves and loads the meta.toml for the given panel path on the caller
// goroutine. Used by dialog/editor paths that already released the terminal.
// MF holds only the Go column providers (nil without any) when no file is found (not an error)
// or it failed to load. Warnings and the load error are shown as transient messages.
func (h *Handler) loadMetaFile(panelID int) loadedMetaFile {
	res := h.fetchMetaFile(h.host.PanelByID(panelID).PathString())
	for _, w := range res.Warns {
		h.host.SetTransientMessage(w, ui.MessageUrgencyWarn)
	}
	if res.LoadErr != "" {
		h.host.SetTransientMessage(res.LoadErr, ui.MessageUrgencyCritical)
	}
	return res
}

func (h *Handler) startAsyncLoad(panelID int, activeNames []string) {
	h.loadGen[panelID]++
	loadGen := h.loadGen[panelID]
	h.loadPending[panelID] = true
	names := append([]string(nil), activeNames...)
	cur := filepath.Clean(h.host.PanelByID(panelID).PathString())
	go func() {
		res := h.fetchMetaFile(cur)
		if h.screen == nil {
			return
		}
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(LoadPayload{
			PanelID:     panelID,
			LoadGen:     loadGen,
			Path:        cur,
			MF:          res.MF,
			ActiveNames: names,
			Warns:       res.Warns,
			LoadErr:     res.LoadErr,
		}))
	}()
}

func (h *Handler) invalidatePendingLoads(panelID int) {
	h.loadGen[panelID]++
	h.loadPending[panelID] = false
}

func (h *Handler) ensureGlobalStub() (path string, err error) {
	path = metacmds.ResolveMetaGlobalPath(h.config, h.model.UserHomeDir, h.configDir)
	if path == "" {
		return "", fmt.Errorf("meta: no global meta path configured")
	}
	if _, err := metacmds.WriteMetaStub(path); err != nil {
		return "", err
	}
	return path, nil
}

func (h *Handler) clearCache() {
	h.cacheMu.Lock()
	h.cache = nil
	h.cacheMu.Unlock()
}

func (h *Handler) rerunSinglePanel(panelID int) {
	if len(h.activeEntries[panelID]) == 0 {
		return
	}
	h.invalidatePendingLoads(panelID)
	mf := h.loadMetaFile(panelID).MF
	if mf == nil {
		return
	}
	sorted := metacmds.SortEntriesForDisplay(h.activeEntries[panelID], mf)
	if len(sorted) == 0 {
		return
	}
	cols := make([]ui.MetaColumnState, len(sorted))
	for i, e := range sorted {
		cols[i] = newColumn(e)
	}
	h.runForPanel(panelID, sorted, cols)
}

// newColumn is the empty panel column of an entry.
func newColumn(e metacmds.MetaEntry) ui.MetaColumnState {
	return ui.MetaColumnState{EntryName: e.Name, ColumnTitle: e.Column, Order: e.Order, Alerts: e.Alerts}
}

func (h *Handler) rerunActivePanels() {
	for panelID := range h.activeEntries {
		h.rerunSinglePanel(panelID)
	}
}

// ReconcileForPanel detects entries with no cell after a same-directory refresh (e.g. rename,
// flatten, periodic scan) or ApplyFSChanges, and dispatches meta for just those entries into the
// current run, leaving every other row's value alone. It also prunes gone rows (pruneGone). Called
// from reconcileAfterEvent; must be cheap when nothing is missing.
func (h *Handler) ReconcileForPanel(panelID int) {
	cols := h.model.MetaResults[panelID]
	if len(cols) == 0 {
		return
	}
	// Results being nil means a run is being set up — nothing to reconcile yet.
	if cols[0].Results == nil {
		return
	}
	if h.loadPending[panelID] {
		return
	}
	p := h.host.PanelByID(panelID)
	if p == nil {
		return
	}
	h.pruneGone(panelID, p)
	groups := []dirEntries{{dir: p.PathString(), entries: p.Entries}}
	// LoadedTreeChildren allocates, so the tree is walked only when children were attached or the
	// tree children were attached since the last walk (a run's walk is stamped by runForPanel).
	if h.treeWalked[panelID] != p.TreeChildrenGen {
		h.treeWalked[panelID] = p.TreeChildrenGen
		groups = append(groups, treeDirEntries(p)...)
	}
	var missing []dirEntries
	for _, g := range groups {
		var es []localfs.Entry
		for _, e := range g.entries {
			if _, ok := cols[0].Results[e.Path]; !ok {
				es = append(es, e)
			}
		}
		if len(es) > 0 {
			missing = append(missing, dirEntries{dir: g.dir, entries: es})
		}
	}
	if len(missing) == 0 {
		return
	}
	r := h.run[panelID]
	if r == nil || r.ctx.Err() != nil || len(r.defs) != len(cols) {
		h.startAsyncLoad(panelID, h.activeEntries[panelID])
		return
	}
	h.dispatch(r.ctx, panelID, r.defs, cols, missing)
}

// panelGroups snapshots every listing the panel shows: its own and each loaded tree node's. The
// result is safe to hand to a background goroutine.
func panelGroups(p *panel.State) []dirEntries {
	root := dirEntries{dir: p.PathString(), entries: append([]localfs.Entry(nil), p.Entries...)}
	return append([]dirEntries{root}, treeDirEntries(p)...)
}

// treeDirEntries returns one group per loaded tree node (dir is the node ID, the child listing's
// directory); nil outside tree layout.
func treeDirEntries(p *panel.State) []dirEntries {
	children := p.LoadedTreeChildren()
	var out []dirEntries
	for _, id := range slices.Sorted(maps.Keys(children)) {
		out = append(out, dirEntries{dir: id, entries: children[id]})
	}
	return out
}

// OpenFileEditor opens the meta.toml at path in an external editor, clears the session
// result cache, and re-runs active meta commands on both panels. Returns false on error
// (an error message has already been set).
func (h *Handler) OpenFileEditor(path string) bool {
	changed, err := metacmds.RefreshDocumentation(path)
	if err != nil {
		h.host.SetErrorMessage("Meta commands", err)
		return false
	}
	if err := h.host.OpenFileInExternalEditor(path); err != nil {
		h.host.SetErrorMessage("Meta commands", err)
		return false
	}
	h.clearCache()
	h.rerunActivePanels()
	if changed {
		h.host.SetTransientMessage("Meta commands: updated documentation in "+path, ui.MessageUrgencyInfo)
	} else {
		h.host.SetTransientMessage("Meta commands: edited "+path, ui.MessageUrgencyInfo)
	}
	return true
}

// EditMetaFile opens the active panel's meta.toml in an external editor, creating a stub
// if it does not exist.
func (h *Handler) EditMetaFile() {
	if h.model.ViewMode != ui.ViewBrowser {
		return
	}
	path, err := h.resolveEditPath(h.model.ActivePanel)
	if err != nil {
		h.host.SetErrorMessage("Meta commands", err)
		return
	}
	h.OpenFileEditor(path)
}

func (h *Handler) CloseDialog() {
	h.model.ColumnsDialog = dialog.ColumnsDialogState{}
}

// entriesFromFile returns MetaEntry slice sorted by section, then order+name, from a MetaFile
// (nil-safe).
func entriesFromFile(mf *metacmds.MetaFile) []dialog.MetaEntry {
	if mf == nil || len(mf.Entries) == 0 {
		return nil
	}
	sorted := metacmds.SortedEntries(mf)
	out := make([]dialog.MetaEntry, len(sorted))
	for i, e := range sorted {
		out[i] = dialog.MetaEntry{
			Name:             e.Name,
			Description:      e.Description,
			SortOnActivation: e.SortOnActivation,
			SortReverse:      e.SortReverse,
		}
		if e.Provider != nil {
			out[i].Section = e.Provider.Section
			if out[i].Section != "" {
				out[i].Label = e.Column
			}
		}
	}
	// Sectioned providers go below the meta.toml entries, grouped by section.
	slices.SortStableFunc(out, func(a, b dialog.MetaEntry) int { return strings.Compare(a.Section, b.Section) })
	return out
}

// limitActiveNames returns the names a panel may have active at once (the first
// config.DefaultMetaMaxActiveColumns), warning when it drops any.
func (h *Handler) limitActiveNames(names []string) []string {
	maxCols := config.DefaultMetaMaxActiveColumns
	if len(names) <= maxCols {
		return names
	}
	h.host.SetTransientMessage(fmt.Sprintf("meta: showing first %d of %d selected columns", maxCols, len(names)), ui.MessageUrgencyWarn)
	return names[:maxCols]
}

// deactivate clears the panel's meta columns, falling back from a meta sort or filter.
func (h *Handler) deactivate(panelID int) {
	h.invalidatePendingLoads(panelID)
	h.stopRun(panelID)
	h.gone[panelID] = nil
	h.model.MetaResults[panelID] = nil
	h.activeEntries[panelID] = nil
	h.navPath[panelID] = ""
	h.resortPending[panelID] = false
	if p := h.host.PanelByID(panelID); p != nil {
		if p.Sort.Mode == panel.SortMeta {
			h.dropMetaSort(panelID, p)
		}
		if p.ActiveEntryFilter != nil && p.ActiveEntryFilter.ID == panel.PatternMetaFilterID {
			// Meta columns just went away: the filter's closure now reads an empty
			// MetaResults, so re-run it immediately rather than waiting for the next flush.
			p.RefreshEntryFilter()
		}
	}
}

// activate switches the panel's meta columns to names and starts loading them.
func (h *Handler) activate(panelID int, activeNames []string) {
	h.stopRun(panelID)
	h.runGen[panelID]++

	names := append([]string(nil), activeNames...)
	cols := make([]ui.MetaColumnState, len(names))
	for i, name := range names {
		cols[i] = ui.MetaColumnState{
			EntryName:   name,
			ColumnTitle: name,
			Results:     nil,
		}
	}
	h.activeEntries[panelID] = names
	h.navPath[panelID] = filepath.Clean(h.host.PanelByID(panelID).PathString())
	h.model.MetaResults[panelID] = cols
	h.startAsyncLoad(panelID, names)
}

// HandlePanelDirChanged re-runs active meta commands when the panel navigates to a new directory.
// The meta file is resolved and loaded off the UI goroutine to avoid blocking on disk I/O.
// A panel with no active columns entering an index view (fileindex:// or Browse with fileindex)
// applies [fileindex].default_profile, and leaving the index views restores the panel's columns
// unless the user changed them meanwhile. Moving between the two views is not an edge.
func (h *Handler) HandlePanelDirChanged(panelID int) {
	p := h.host.PanelByID(panelID)
	if p == nil {
		return
	}
	inIndex := p.ShowsIndex()
	entered := inIndex && !h.inIndexView[panelID]
	left := !inIndex && h.inIndexView[panelID]
	h.inIndexView[panelID] = inIndex
	if entered && len(h.activeEntries[panelID]) == 0 && h.applyIndexProfile(panelID, p) {
		return
	}
	if left && h.autoColumns[panelID] != nil {
		h.restoreAutoColumns(panelID, p)
		return
	}
	if len(h.activeEntries[panelID]) == 0 {
		return
	}
	cur := filepath.Clean(h.host.PanelByID(panelID).PathString())
	if h.navPath[panelID] == cur {
		return
	}
	h.navPath[panelID] = cur
	h.stopRun(panelID)
	h.runGen[panelID]++
	h.startAsyncLoad(panelID, h.activeEntries[panelID])
}

// RefreshPanel handles a user-requested panel refresh: drops the session `cache = true`
// results and re-runs the panel's active meta commands so every value is recalculated. Changes
// pc makes itself go through ApplyFSChanges and RefreshProviders instead.
func (h *Handler) RefreshPanel(panelID int) {
	h.clearCache()
	if len(h.model.MetaResults[panelID]) == 0 || len(h.activeEntries[panelID]) == 0 {
		return
	}
	h.stopRun(panelID)
	h.runGen[panelID]++
	h.startAsyncLoad(panelID, h.activeEntries[panelID])
}

// HandleLoad is called on the UI goroutine when an async meta file load completes.
// Called from Run's interrupt switch for LoadPayload.
func (h *Handler) HandleLoad(d LoadPayload) {
	if d.LoadGen != h.loadGen[d.PanelID] {
		return
	}
	h.loadPending[d.PanelID] = false
	for _, w := range d.Warns {
		h.host.SetTransientMessage(w, ui.MessageUrgencyWarn)
	}
	if d.LoadErr != "" {
		// A broken meta.toml still leaves the Go column providers to run.
		h.host.SetTransientMessage(d.LoadErr, ui.MessageUrgencyCritical)
	}
	// Without a file, columns the user picked stay until a load works again; default columns
	// fall through and are dropped (an index view with no index database and no meta.toml).
	if d.MF == nil && h.autoColumns[d.PanelID] == nil {
		return
	}
	panel := h.host.PanelByID(d.PanelID)
	if panel == nil {
		return
	}
	if d.Path != "" && filepath.Clean(d.Path) != filepath.Clean(panel.PathString()) {
		return
	}
	if !slices.Equal(d.ActiveNames, h.activeEntries[d.PanelID]) {
		return
	}
	sorted := metacmds.SortEntriesForDisplay(d.ActiveNames, d.MF)
	if len(sorted) == 0 {
		// None of the names exist here: drop the placeholder columns rather than leaving empty
		// headers. The names stay active, so a directory that offers them (a local meta.toml)
		// shows them again; default columns have no such pull and go.
		h.model.MetaResults[d.PanelID] = nil
		if h.autoColumns[d.PanelID] != nil {
			h.activeEntries[d.PanelID] = nil
		}
		return
	}
	h.sortOnActivation(panel, d.PanelID, sorted)
	cols := make([]ui.MetaColumnState, len(sorted))
	for i, e := range sorted {
		cols[i] = newColumn(e)
	}
	h.runForPanel(d.PanelID, sorted, cols)
}

// HandleRenderFlush consumes a coalesced repaint. Called from the event loop
// (or tests simulating RenderFlushPayload) so timer state is not written from AfterFunc.
// For any panel whose active SortMeta column received new values since the last flush, this
// only *signals* the host once the column is fully resolved (ColumnResolved) — it never sorts
// directly. The host (App) decides when to actually apply the re-sort, deferred behind the same
// idle-delay/user-activity hold as the disk-usage idle sort, so the listing does not reshuffle
// under the user while they are still scrolling/typing.
func (h *Handler) HandleRenderFlush(d RenderFlushPayload) {
	h.renderDebounce[d.PanelID].Stop()
	h.publishLayout(d.PanelID)
	if h.resortPending[d.PanelID] {
		h.resortPending[d.PanelID] = false
		if h.ColumnResolved(d.PanelID) {
			h.host.NoteMetaColumnResolved(d.PanelID)
		}
	}
	h.refreshMetaPatternFilter(d.PanelID)
	h.reportLostColumns(d.PanelID)
}

// publishLayout makes panelID's latest results the drawn layout. Once nothing is pending the
// values are final, so the width guess of the run goes too.
func (h *Handler) publishLayout(panelID int) {
	cols := h.model.MetaResults[panelID]
	resolved := pendingCount(cols) == 0
	for i := range cols {
		cols[i].LayoutGen = cols[i].ResultsGen
		if resolved {
			cols[i].Guess = ui.MetaLayoutGuess{}
		}
	}
}

// seedLayoutGuess starts cols at the layouts panelID last measured, its outgoing columns'
// included: a refresh, or a sibling directory of the same kind, then moves nothing as its values
// arrive.
func (h *Handler) seedLayoutGuess(panelID int, cols []ui.MetaColumnState) {
	if h.layoutGuess[panelID] == nil {
		h.layoutGuess[panelID] = make(map[string]ui.MetaLayoutGuess)
	}
	outgoing := h.model.MetaResults[panelID]
	for i := range outgoing {
		if guess, ok := ui.MeasuredMetaLayout(&outgoing[i]); ok {
			h.layoutGuess[panelID][outgoing[i].EntryName] = guess
		}
	}
	for i := range cols {
		cols[i].Guess = h.layoutGuess[panelID][cols[i].EntryName]
	}
}

// MinColumnShownBeforeToast is how long a run must have been showing a column for
// reportLostColumns to report losing it: a column gone sooner was barely on screen, and a toast
// would follow every navigation into a directory with wide values.
const MinColumnShownBeforeToast = 500 * time.Millisecond

// columnFitWatch is one panel's baseline for reportLostColumns; the zero value has none.
type columnFitWatch struct {
	gen      uint64 // run the baseline belongs to
	seededAt time.Time
	fit      ui.PanelMetaFit
	warned   map[string]bool // columns already reported in this run, by EntryName
}

// seedColumnFit records what panelID draws as the baseline of its current run.
func (h *Handler) seedColumnFit(panelID int, fit ui.PanelMetaFit) {
	h.fitWatch[panelID] = columnFitWatch{gen: h.runGen[panelID], seededAt: time.Now(), fit: fit, warned: map[string]bool{}}
}

// reportLostColumns toasts columns that incoming results pushed out of panelID's listing. The
// baseline is seeded when a run starts. A column lost within MinColumnShownBeforeToast of that is
// not reported, nor is one that never fit, and a change of available space (resize, zoom, listing
// format) moves the baseline silently: the user caused that one and can see it. Each column is
// reported once per run.
func (h *Handler) reportLostColumns(panelID int) {
	w := &h.fitWatch[panelID]
	cols := h.model.MetaResults[panelID]
	if len(cols) == 0 {
		*w = columnFitWatch{}
		return
	}
	seeded := !w.seededAt.IsZero() && w.gen == h.runGen[panelID]
	fit, ok := h.host.PanelMetaFit(panelID)
	if !ok {
		*w = columnFitWatch{}
		return
	}
	if !seeded {
		h.seedColumnFit(panelID, fit)
		return
	}
	prev := w.fit
	w.fit = fit
	if prev.Width != fit.Width || prev.NameRoom != fit.NameRoom || time.Since(w.seededAt) < MinColumnShownBeforeToast {
		return
	}
	var lost []string
	for _, lay := range prev.Drawn {
		drawn := slices.ContainsFunc(fit.Drawn, func(l ui.MetaColumnLayout) bool { return l.EntryName == lay.EntryName })
		if !w.warned[lay.EntryName] && !drawn {
			w.warned[lay.EntryName] = true
			lost = append(lost, lay.Title)
		}
	}
	if n := len(lost); n > 0 {
		h.host.SetTransientMessage(fmt.Sprintf("%s %s no longer %s on available space",
			jobbridge.Plural(n, "Column", "Columns"), strings.Join(lost, ", "), jobbridge.Plural(n, "fits", "fit")), ui.MessageUrgencyInfo)
	}
}

// refreshMetaPatternFilter re-runs panelID's active entry filter when it matches live meta values
// (panel.PatternMetaFilterID), so rows appear/disappear as meta results resolve.
func (h *Handler) refreshMetaPatternFilter(panelID int) {
	p := h.host.PanelByID(panelID)
	if p != nil && p.ActiveEntryFilter != nil && p.ActiveEntryFilter.ID == panel.PatternMetaFilterID {
		p.RefreshEntryFilter()
	}
}

// ColumnResolved reports whether panelID currently sorts by a meta column (Sort.Mode ==
// SortMeta) whose command has finished for every dispatched entry (PendingCount == 0), and no
// meta.toml reload is pending for the panel (a pending reload can still replace the active
// columns, so it never counts as resolved). Used both when this handler signals a column just
// finished (HandleRenderFlush) and by the App-side idle re-sort scheduler re-arming its timer on
// user activity (deferMetaIdleSortOnUserActivity), so it can recheck without waiting for another
// wake event. O(1): PendingCount is kept in sync by runForPanel/applyWakeResult rather than
// scanning Results here.
func (h *Handler) ColumnResolved(panelID int) bool {
	if h.loadPending[panelID] {
		return false
	}
	p := h.host.PanelByID(panelID)
	if p == nil || p.Sort.Mode != panel.SortMeta || p.Sort.MetaColumn == "" {
		return false
	}
	col, ok := ui.MetaColumnByName(h.model.MetaResults[panelID], p.Sort.MetaColumn)
	if !ok {
		return false
	}
	return col.PendingCount == 0
}

// pendingCount is the number of dispatched cells still running across cols.
func pendingCount(cols []ui.MetaColumnState) int {
	n := 0
	for _, c := range cols {
		n += c.PendingCount
	}
	return n
}

// HandlePendingReveal shows the pending markers of panelID once the reveal delay elapsed with
// commands still running. Called from Run's interrupt switch for PendingRevealPayload.
func (h *Handler) HandlePendingReveal(d PendingRevealPayload) {
	if d.Gen != h.runGen[d.PanelID] {
		return
	}
	cols := h.model.MetaResults[d.PanelID]
	for i := range cols {
		cols[i].PendingHidden = false
	}
}

// scheduleRenderFlush coalesces panelID's WakePayload events (one per entry in large directories)
// into a flush at most every config.MetaLayoutInterval. It is first-arm-wins, so a steady stream
// of results cannot postpone the flush; the result that resolves the last cell flushes at once.
func (h *Handler) scheduleRenderFlush(panelID int) {
	if pendingCount(h.model.MetaResults[panelID]) == 0 {
		h.armRenderFlush(panelID, 0)
		return
	}
	if !h.renderDebounce[panelID].Armed() {
		h.armRenderFlush(panelID, config.MetaLayoutInterval)
	}
}

func (h *Handler) armRenderFlush(panelID int, delay time.Duration) {
	h.renderDebounce[panelID].Arm(delay, func() {
		if h.screen == nil {
			return
		}
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(RenderFlushPayload{PanelID: panelID}))
	})
}

// runForPanel runs meta commands for every active entry on panelID using worker pools.
func (h *Handler) runForPanel(panelID int, cmdDefs []metacmds.MetaEntry, cols []ui.MetaColumnState) {
	panel := h.host.PanelByID(panelID)
	groups := panelGroups(panel)

	h.stopRun(panelID)
	h.gone[panelID] = nil
	ctx, cancel := context.WithCancel(context.Background())
	h.run[panelID] = &metaRun{ctx: ctx, cancel: cancel, defs: cmdDefs}
	h.runGen[panelID]++
	// This run covers every loaded tree child, so ReconcileForPanel need not walk the tree again.
	h.treeWalked[panelID] = panel.TreeChildrenGen

	h.seedLayoutGuess(panelID, cols)
	for i := range cols {
		cols[i].Results = make(map[string]string, len(groups[0].entries))
		cols[i].PendingCount = 0
	}
	h.model.MetaResults[panelID] = cols
	h.dispatch(ctx, panelID, cmdDefs, cols, groups)
	if fit, ok := h.host.PanelMetaFit(panelID); ok {
		h.seedColumnFit(panelID, fit)
	}
}

// dispatch marks the groups' entries pending in cols (pre-filling cached values) and runs cmdDefs
// for them in the background under ctx and the panel's current run generation.
func (h *Handler) dispatch(ctx context.Context, panelID int, cmdDefs []metacmds.MetaEntry, cols []ui.MetaColumnState, groups []dirEntries) {
	gen := h.runGen[panelID]
	runningMarker := h.host.IconMetaRunning()

	// Only a dispatch into an idle panel starts the hide/reveal cycle. A later dispatch (e.g.
	// ReconcileForPanel adding rows) must neither postpone the reveal nor re-hide revealed columns.
	wasIdle := pendingCount(cols) == 0

	for i, cmdDef := range cmdDefs {
		results := cols[i].Results
		for e := range entriesIn(groups) {
			results[e.Path] = ""
		}
		if cmdDef.Cache {
			h.cacheMu.RLock()
			cmdCache := h.cache[cmdDef.Name] // nil when nothing is cached yet
			for e := range entriesIn(groups) {
				if v, ok := cmdCache[e.Path]; ok {
					results[e.Path] = v
				}
			}
			h.cacheMu.RUnlock()
		}

		for e, dir := range entriesIn(groups) {
			if cmdDef.Provider == nil {
				if _, ok := h.entryCmd(cmdDef, e, dir); !ok {
					continue
				}
			}
			results[e.Path] = runningMarker
			cols[i].PendingCount++
		}
		cols[i].Pending = runningMarker
		cols[i].ResultsGen++
	}

	if delay := h.config.Panels.PendingMarkerDelayMS; wasIdle && delay != 0 && pendingCount(cols) > 0 {
		for i := range cols {
			cols[i].PendingHidden = true
		}
		if delay > 0 {
			h.pendingReveal[panelID].Arm(time.Duration(delay)*time.Millisecond, func() {
				if h.screen == nil {
					return
				}
				_ = h.screen.PostEvent(tcell.NewEventInterrupt(PendingRevealPayload{PanelID: panelID, Gen: gen}))
			})
		}
	}

	// A dispatch into an idle panel shows its pending and cached cells right away, then holds the
	// layout so the first update has a sample of values to size the columns from.
	if wasIdle {
		h.publishLayout(panelID)
		if pendingCount(cols) > 0 {
			h.armRenderFlush(panelID, config.MetaLayoutInterval)
		}
	}

	// A run that dispatches nothing for the sorted column (every entry cached or filtered out
	// by `when`) never posts a wake, so HandleWake/HandleRenderFlush never get a chance to
	// signal resolution — check it here too.
	if h.ColumnResolved(panelID) {
		h.host.NoteMetaColumnResolved(panelID)
	}

	defaultWorkers := h.config.Meta.DefaultEntryWorkers // read here: SetConfig runs on the UI goroutine
	go func() {
		var wg sync.WaitGroup
		var notifyExecFailed sync.Once

	dispatch:
		for _, cmdDef := range cmdDefs {
			cmdDef := cmdDef
			if cmdDef.Provider != nil {
				wg.Add(1)
				go func() {
					defer wg.Done()
					h.runProvider(ctx, panelID, gen, cmdDef.Provider, groups)
				}()
				continue
			}
			workers := cmdDef.Workers
			if workers < 1 {
				workers = defaultWorkers
			}

			var items []dispatchItem
			for e, dir := range entriesIn(groups) {
				cmd, ok := h.entryCmd(cmdDef, e, dir)
				if !ok {
					continue
				}
				items = append(items, dispatchItem{entry: e, dir: dir, cmd: cmd})
			}

			sem := make(chan struct{}, workers)
			for _, item := range items {
				item := item
				// Stop queueing once cancelled (navigated away / rerun) so a large backlog is
				// dropped instead of spinning up a goroutine per remaining row.
				select {
				case sem <- struct{}{}:
				case <-ctx.Done():
					break dispatch
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					out, err := runCommand(ctx, item.cmd, item.entry.Path, item.dir)
					if err != nil {
						if ctx.Err() != nil {
							return
						}
						var failure *runFailure
						if errors.As(err, &failure) {
							notifyExecFailed.Do(func() {
								h.postExecFailed(panelID, gen, failure.ExitCode, failure.Stderr, failure.ConfCmd, failure.ExpandedCmd)
							})
						} else {
							notifyExecFailed.Do(func() { h.postExecFailed(panelID, gen, -1, "", "", "") })
						}
						h.postResult(panelID, cmdDef.Name, item.entry.Path, "", gen)
						return
					}
					if cmdDef.Cache {
						h.cacheMu.Lock()
						if h.cache == nil {
							h.cache = make(map[string]map[string]string)
						}
						if h.cache[cmdDef.Name] == nil {
							h.cache[cmdDef.Name] = make(map[string]string)
						}
						h.cache[cmdDef.Name][item.entry.Path] = out
						h.cacheMu.Unlock()
					}
					h.postResult(panelID, cmdDef.Name, item.entry.Path, out, gen)
				}()
			}
		}
		wg.Wait()
	}()
}

// entriesIn yields every entry of groups with the directory it is listed in.
func entriesIn(groups []dirEntries) iter.Seq2[localfs.Entry, string] {
	return func(yield func(localfs.Entry, string) bool) {
		for _, g := range groups {
			for _, e := range g.entries {
				if !yield(e, g.dir) {
					return
				}
			}
		}
	}
}

// runProvider calls a Go column provider once per group directory and posts every entry's value in
// ONE WakePayload (Values; missing entries get ""), so a large listing or a wide tree expand
// cannot flood the event queue and drop wakes that would leave pending markers stuck.
// A provider error is reported once (ExecFailedPayload) and leaves that directory's cells empty.
func (h *Handler) runProvider(ctx context.Context, panelID int, gen uint64, p *metacmds.Provider, groups []dirEntries) {
	n := 0
	for _, g := range groups {
		n += len(g.entries)
	}
	batch := make(map[string]string, n)
	reported := false
	for _, g := range groups {
		vals, err := p.Values(ctx, g.dir, g.entries)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			if !reported {
				reported = true
				_ = h.screen.PostEvent(tcell.NewEventInterrupt(ExecFailedPayload{PanelID: panelID, Gen: gen, Provider: p.Name, Stderr: err.Error()}))
			}
			vals = nil
		}
		for _, e := range g.entries {
			batch[e.Path] = vals[e.Path]
		}
	}
	_ = h.screen.PostEvent(tcell.NewEventInterrupt(WakePayload{PanelID: panelID, EntryName: p.Name, Values: batch, Gen: gen}))
}

// entryCmd returns the shell command to run for entry e under cmdDef.
// when filters every row; files use file, directories use dirs.
// Returns ("", false) when the entry should not be dispatched (a ghost row, which has no file to
// run on; filtered, no command, or cached).
func (h *Handler) entryCmd(cmdDef metacmds.MetaEntry, e localfs.Entry, dir string) (cmd string, ok bool) {
	if e.IndexStatus == localfs.IndexStatusMissing {
		return "", false
	}
	matched, err := cmdDef.MatchesRow(e, dir)
	if err != nil || !matched {
		return "", false
	}
	if e.Type == localfs.EntryDirectory {
		cmd = cmdDef.Dirs
	} else {
		cmd = cmdDef.File
	}
	if cmd == "" {
		return "", false
	}
	if cmdDef.Cache {
		h.cacheMu.RLock()
		var hit bool
		if h.cache != nil {
			if cc := h.cache[cmdDef.Name]; cc != nil {
				_, hit = cc[e.Path]
			}
		}
		h.cacheMu.RUnlock()
		if hit {
			return "", false
		}
	}
	return cmd, true
}

// runCommand runs a shell command template with %f expanded to path.
// Returns trimmed stdout on success. On failure returns *runFailure (or a plain error for context cancellation).
func runCommand(ctx context.Context, cmd, path, dir string) (string, error) {
	built, err := cmdrun.BuildInvocation(cmdrun.InvocationSpec{
		Template: cmd,
		Mode:     cmdrun.ModeShellScript,
		Ctx:      cmdmacro.Context{RowPath: path},
	})
	if err != nil {
		return "", &runFailure{ExitCode: -1, Stderr: err.Error(), err: err}
	}
	res := cmdrun.Run(ctx, built.Argv, dir, cmdrun.MaxStreamBytes)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if res.LaunchErr != nil {
		return "", &runFailure{
			ExitCode:    res.ExitCode,
			Stderr:      strings.TrimSpace(string(res.Stderr)),
			ConfCmd:     cmd,
			ExpandedCmd: built.Expanded,
			err:         res.LaunchErr,
		}
	}
	if res.ExitCode != 0 {
		return "", &runFailure{
			ExitCode:    res.ExitCode,
			Stderr:      strings.TrimSpace(string(res.Stderr)),
			ConfCmd:     cmd,
			ExpandedCmd: built.Expanded,
			err:         fmt.Errorf("exit status %d", res.ExitCode),
		}
	}
	return strings.TrimRight(string(res.Stdout), "\r\n"), nil
}

// CancelAll cancels every in-flight per-panel meta run. Called on quit.
func (h *Handler) CancelAll() {
	for i := range h.run {
		h.stopRun(i)
	}
}

// stopRun cancels the in-flight meta run for panelID, if any, and clears it.
func (h *Handler) stopRun(panelID int) {
	h.pendingReveal[panelID].Invalidate()
	h.stopProviderRefresh(panelID)
	if r := h.run[panelID]; r != nil {
		r.cancel()
		h.run[panelID] = nil
	}
}

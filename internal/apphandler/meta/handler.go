package meta

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdmacro"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
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

// HandleWake applies one async command result and schedules a debounced repaint.
// Called from Run's interrupt switch for WakePayload.
func (h *Handler) HandleWake(d WakePayload) {
	if d.Gen == h.runGen[d.PanelID] {
		h.applyWakeResult(d)
		if p := h.host.PanelByID(d.PanelID); p != nil && p.Sort.Mode == panel.SortMeta && p.Sort.MetaColumn == d.EntryName {
			h.resortPending[d.PanelID] = true
		}
	}
	h.scheduleRenderDebounced()
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
	const urgency = ui.MessageUrgencyCritical
	banner := "meta: command failed to execute"

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

func (h *Handler) fetchMetaFile(panelPath string) loadedMetaFile {
	path, warns := metacmds.ResolveMetaTOML(h.config, h.model.UserHomeDir, h.configDir, panelPath)
	if path == "" {
		return loadedMetaFile{Warns: warns}
	}
	mf, err := metacmds.LoadFile(path)
	if err != nil {
		return loadedMetaFile{Warns: warns, LoadErr: "meta: " + err.Error()}
	}
	return loadedMetaFile{MF: mf, Warns: warns}
}

// loadMetaFile resolves and loads the meta.toml for the given panel path on the caller
// goroutine. Used by dialog/editor paths that already released the terminal.
// Returns nil when no file is found (not an error). Warnings are shown as transient messages.
func (h *Handler) loadMetaFile(panelID int) *metacmds.MetaFile {
	res := h.fetchMetaFile(h.host.PanelByID(panelID).PathString())
	for _, w := range res.Warns {
		h.host.SetTransientMessage(w, ui.MessageUrgencyWarn)
	}
	if res.LoadErr != "" {
		h.host.SetTransientMessage(res.LoadErr, ui.MessageUrgencyCritical)
		return nil
	}
	return res.MF
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
	mf := h.loadMetaFile(panelID)
	if mf == nil {
		return
	}
	sorted := metacmds.SortEntriesForDisplay(h.activeEntries[panelID], mf)
	if len(sorted) == 0 {
		return
	}
	cols := make([]ui.MetaColumnState, len(sorted))
	for i, e := range sorted {
		cols[i] = ui.MetaColumnState{
			EntryName:   e.Name,
			ColumnTitle: e.Column,
			Order:       e.Order,
			Results:     nil,
		}
	}
	h.runForPanel(panelID, sorted, cols)
}

func (h *Handler) rerunActivePanels() {
	for panelID := range h.activeEntries {
		h.rerunSinglePanel(panelID)
	}
}

// ReconcileForPanel detects entries that appeared in the panel listing after a same-directory
// refresh (e.g. flatten, periodic scan) and re-runs meta for the panel so the new entries get
// their meta column values populated. Called from reconcileAfterEvent; must be cheap when
// nothing is missing.
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
	for _, e := range p.Entries {
		if _, ok := cols[0].Results[e.Path]; !ok {
			h.startAsyncLoad(panelID, h.activeEntries[panelID])
			return
		}
	}
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

// OpenDialog opens the checkbox meta command picker for the given panel.
func (h *Handler) OpenDialog(panelID int) {
	if h.model.ViewMode != ui.ViewBrowser {
		return
	}
	mf := h.loadMetaFile(panelID)
	entries := entriesFromFile(mf)

	activeSet := make(map[string]struct{}, len(h.activeEntries[panelID]))
	for _, n := range h.activeEntries[panelID] {
		activeSet[n] = struct{}{}
	}
	checked := make([]bool, len(entries))
	for i, e := range entries {
		_, checked[i] = activeSet[e.Name]
	}

	h.model.MetaDialog = dialog.MetaDialogState{
		Open:    true,
		PanelID: panelID,
		Entries: entries,
		Checked: checked,
		Focus:   0,
	}
	h.host.ClearTransientMessage()
}

func (h *Handler) closeDialog() {
	h.model.MetaDialog = dialog.MetaDialogState{}
}

// entriesFromFile returns MetaEntry slice sorted by order+name from a MetaFile (nil-safe).
func entriesFromFile(mf *metacmds.MetaFile) []dialog.MetaEntry {
	if mf == nil || len(mf.Entries) == 0 {
		return nil
	}
	sorted := metacmds.SortedEntries(mf)
	out := make([]dialog.MetaEntry, len(sorted))
	for i, e := range sorted {
		out[i] = dialog.MetaEntry{Name: e.Name, Description: e.Description}
	}
	return out
}

// ActivateSelection applies the checked set from the open meta dialog: (re)runs the
// selected commands for the dialog's panel, or clears the panel's meta columns when
// nothing is checked.
func (h *Handler) ActivateSelection() {
	st := h.model.MetaDialog
	panelID := st.PanelID
	checked := append([]bool(nil), st.Checked...)
	entries := append([]dialog.MetaEntry(nil), st.Entries...)
	h.closeDialog()

	var activeNames []string
	for i, on := range checked {
		if on && i < len(entries) {
			activeNames = append(activeNames, entries[i].Name)
		}
	}

	if len(activeNames) == 0 {
		h.invalidatePendingLoads(panelID)
		if h.cancel[panelID] != nil {
			h.cancel[panelID]()
			h.cancel[panelID] = nil
		}
		h.model.MetaResults[panelID] = nil
		h.activeEntries[panelID] = nil
		h.navPath[panelID] = ""
		h.resortPending[panelID] = false
		if p := h.host.PanelByID(panelID); p != nil && p.Sort.Mode == panel.SortMeta {
			// The sorted-on column no longer exists — fall back to Name immediately (like an
			// explicit Sort dialog apply), rather than leaving the panel sorted by a column
			// that just vanished from the header.
			p.Sort.Mode = panel.SortName
			p.Sort.MetaColumn = ""
			h.host.ResortPanel(panelID)
		}
		return
	}

	maxCols := config.DefaultMetaMaxActiveColumns
	if len(activeNames) > maxCols {
		h.host.SetTransientMessage(fmt.Sprintf("meta: showing first %d of %d selected columns", maxCols, len(activeNames)), ui.MessageUrgencyWarn)
		activeNames = activeNames[:maxCols]
	}

	if h.cancel[panelID] != nil {
		h.cancel[panelID]()
		h.cancel[panelID] = nil
	}
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
func (h *Handler) HandlePanelDirChanged(panelID int) {
	if len(h.model.MetaResults[panelID]) == 0 {
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
		h.host.SetTransientMessage(d.LoadErr, ui.MessageUrgencyCritical)
		return
	}
	if d.MF == nil {
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
		return
	}
	cols := make([]ui.MetaColumnState, len(sorted))
	for i, e := range sorted {
		cols[i] = ui.MetaColumnState{
			EntryName:   e.Name,
			ColumnTitle: e.Column,
			Order:       e.Order,
			Results:     nil,
		}
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
func (h *Handler) HandleRenderFlush() {
	h.renderDebounce.Stop()
	for panelID := range h.resortPending {
		if !h.resortPending[panelID] {
			continue
		}
		h.resortPending[panelID] = false
		if h.ColumnResolved(panelID) {
			h.host.NoteMetaColumnResolved(panelID)
		}
	}
}

// ColumnResolved reports whether panelID currently sorts by a meta column (Sort.Mode ==
// SortMeta) whose command has finished for every dispatched entry (PendingCount == 0). Used both
// when this handler signals a column just finished (HandleRenderFlush) and by the App-side idle
// re-sort scheduler re-arming its timer on user activity (deferMetaIdleSortOnUserActivity), so it
// can recheck without waiting for another wake event. O(1): PendingCount is kept in sync by
// runForPanel/applyWakeResult rather than scanning Results here.
func (h *Handler) ColumnResolved(panelID int) bool {
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

// scheduleRenderDebounced arms a short timer to coalesce rapid WakePayload events
// (one per entry in large directories) into a single screen repaint at ~60 fps.
func (h *Handler) scheduleRenderDebounced() {
	if h.renderDebounce.Armed() {
		return
	}
	const debounce = 16 * time.Millisecond
	h.renderDebounce.Arm(debounce, func() {
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(RenderFlushPayload{}))
	})
}

// runForPanel runs meta commands for every active entry on panelID using worker pools.
func (h *Handler) runForPanel(panelID int, cmdDefs []metacmds.MetaEntry, cols []ui.MetaColumnState) {
	panel := h.host.PanelByID(panelID)
	entries := append([]localfs.Entry(nil), panel.Entries...)
	dir := panel.PathString()

	if h.cancel[panelID] != nil {
		h.cancel[panelID]()
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel[panelID] = cancel
	h.runGen[panelID]++
	gen := h.runGen[panelID]

	runningMarker := h.host.IconMetaRunning()

	for i, cmdDef := range cmdDefs {
		results := make(map[string]string, len(entries))
		for _, e := range entries {
			results[e.Path] = ""
		}
		if cmdDef.Cache {
			h.cacheMu.RLock()
			if h.cache != nil {
				if cmdCache := h.cache[cmdDef.Name]; cmdCache != nil {
					for _, e := range entries {
						if v, ok := cmdCache[e.Path]; ok {
							results[e.Path] = v
						}
					}
				}
			}
			h.cacheMu.RUnlock()
		}

		pendingCount := 0
		for _, e := range entries {
			if _, ok := h.entryCmd(cmdDef, e, dir); !ok {
				continue
			}
			results[e.Path] = runningMarker
			pendingCount++
		}
		cols[i].Results = results
		cols[i].Pending = runningMarker
		cols[i].PendingCount = pendingCount
	}

	h.model.MetaResults[panelID] = cols

	go func() {
		var wg sync.WaitGroup
		var notifyExecFailed sync.Once

		for _, cmdDef := range cmdDefs {
			cmdDef := cmdDef
			workers := cmdDef.Workers
			if workers < 1 {
				workers = h.config.Meta.DefaultEntryWorkers
			}

			var items []dispatchItem
			for _, e := range entries {
				cmd, ok := h.entryCmd(cmdDef, e, dir)
				if !ok {
					continue
				}
				items = append(items, dispatchItem{entry: e, cmd: cmd})
			}

			sem := make(chan struct{}, workers)
			for _, item := range items {
				item := item
				wg.Add(1)
				sem <- struct{}{}
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					out, err := runCommand(ctx, item.cmd, item.entry.Path, dir)
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

// entryCmd returns the shell command to run for entry e under cmdDef.
// File rows are filtered via when only; directories use dirs.
// Returns ("", false) when the entry should not be dispatched (filtered, no command, or cached).
func (h *Handler) entryCmd(cmdDef metacmds.MetaEntry, e localfs.Entry, dir string) (cmd string, ok bool) {
	if e.Type == localfs.EntryDirectory {
		cmd = cmdDef.Dirs
	} else {
		ok, err := cmdDef.MatchesRow(e, dir)
		if err != nil || !ok {
			return "", false
		}
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

// HandleDialogKey routes a key event to the open meta checkbox dialog.
func (h *Handler) HandleDialogKey(event *tcell.EventKey) {
	st := &h.model.MetaDialog
	n := len(st.Entries)
	form := dialog.NewDialogLinearForm(n)

	if dialog.AltDialogOK(event) {
		h.ActivateSelection()
		return
	}
	if dialog.AltDialogCancel(event) {
		h.closeDialog()
		return
	}

	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		if i, ok := dialog.MetaEntryIndexForAltShortcut(st.Entries, event.Rune()); ok {
			st.Checked[i] = !st.Checked[i]
			st.Focus = i
			return
		}
	}

	switch event.Key() {
	case tcell.KeyF9:
		h.editConfigFromDialog()
		return
	case tcell.KeyEsc:
		h.closeDialog()
	case tcell.KeyEnter:
		switch st.Focus {
		case form.CancelIndex():
			h.closeDialog()
		case form.OKIndex():
			h.ActivateSelection()
		default:
			if st.Focus < n {
				st.Checked[st.Focus] = !st.Checked[st.Focus]
			}
		}
	case tcell.KeyRune:
		if event.Modifiers() != tcell.ModNone {
			break
		}
		switch event.Rune() {
		case 'o', 'O':
			h.ActivateSelection()
			return
		case 'c', 'C':
			h.closeDialog()
			return
		case ' ':
			switch {
			case st.Focus < n:
				st.Checked[st.Focus] = !st.Checked[st.Focus]
			case st.Focus == form.OKIndex():
				h.ActivateSelection()
			case st.Focus == form.CancelIndex():
				h.closeDialog()
			}
			return
		}
	}
	if focus, ok := form.MoveFocus(st.Focus, event.Key()); ok {
		st.Focus = focus
	}
}

// CancelAll cancels every in-flight per-panel meta run. Called on quit.
func (h *Handler) CancelAll() {
	for i := range h.cancel {
		if h.cancel[i] != nil {
			h.cancel[i]()
		}
	}
}

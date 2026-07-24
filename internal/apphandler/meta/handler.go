package meta

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
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
		cols[i].Results[d.Path] = d.Value
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

// loadMetaFile resolves and loads the meta.toml for the given panel path.
// Returns nil when no file is found (not an error). Warnings are shown as transient messages.
func (h *Handler) loadMetaFile(panelID int) *metacmds.MetaFile {
	path, warns := metacmds.ResolveMetaTOML(h.config, h.model.UserHomeDir, h.configDir, h.host.PanelByID(panelID).PathString())
	for _, w := range warns {
		h.host.SetTransientMessage(w, ui.MessageUrgencyWarn)
	}
	if path == "" {
		return nil
	}
	mf, err := metacmds.LoadFile(path)
	if err != nil {
		h.host.SetTransientMessage("meta: "+err.Error(), ui.MessageUrgencyCritical)
		return nil
	}
	return mf
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
	p := h.host.PanelByID(panelID)
	if p == nil {
		return
	}
	for _, e := range p.Entries {
		if _, ok := cols[0].Results[e.Path]; !ok {
			h.rerunSinglePanel(panelID)
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
	h.host.Render()
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
		if h.cancel[panelID] != nil {
			h.cancel[panelID]()
			h.cancel[panelID] = nil
		}
		h.model.MetaResults[panelID] = nil
		h.activeEntries[panelID] = nil
		h.navPath[panelID] = ""
		return
	}

	mf := h.loadMetaFile(panelID)
	if mf == nil {
		return
	}

	maxCols := config.DefaultMetaMaxActiveColumns
	if len(activeNames) > maxCols {
		h.host.SetTransientMessage(fmt.Sprintf("meta: showing first %d of %d selected columns", maxCols, len(activeNames)), ui.MessageUrgencyWarn)
		activeNames = activeNames[:maxCols]
	}

	sorted := metacmds.SortEntriesForDisplay(activeNames, mf)
	names := make([]string, len(sorted))
	cols := make([]ui.MetaColumnState, len(sorted))
	for i, e := range sorted {
		names[i] = e.Name
		cols[i] = ui.MetaColumnState{
			EntryName:   e.Name,
			ColumnTitle: e.Column,
			Order:       e.Order,
			Results:     nil,
		}
	}
	h.activeEntries[panelID] = names
	h.navPath[panelID] = filepath.Clean(h.host.PanelByID(panelID).PathString())
	h.runForPanel(panelID, sorted, cols)
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

	h.loadGen[panelID]++
	loadGen := h.loadGen[panelID]
	activeNames := append([]string(nil), h.activeEntries[panelID]...)
	cfg := h.config
	homeDir := h.model.UserHomeDir
	configDir := h.configDir

	go func() {
		path, warns := metacmds.ResolveMetaTOML(cfg, homeDir, configDir, cur)
		p := LoadPayload{
			PanelID:     panelID,
			LoadGen:     loadGen,
			ActiveNames: activeNames,
			Warns:       warns,
		}
		if path != "" {
			mf, err := metacmds.LoadFile(path)
			if err != nil {
				p.LoadErr = "meta: " + err.Error()
			} else {
				p.MF = mf
			}
		}
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(p))
	}()
}

// HandleLoad is called on the UI goroutine when an async meta file load completes.
// Called from Run's interrupt switch for LoadPayload.
func (h *Handler) HandleLoad(d LoadPayload) {
	if d.LoadGen != h.loadGen[d.PanelID] {
		return
	}
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

// scheduleRenderDebounced arms a short timer to coalesce rapid WakePayload events
// (one per entry in large directories) into a single screen repaint at ~60 fps.
func (h *Handler) scheduleRenderDebounced() {
	if h.renderTimer != nil {
		return
	}
	const debounce = 16 * time.Millisecond
	h.renderTimer = time.AfterFunc(debounce, func() {
		h.renderTimer = nil
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

	runningMarker := h.host.SymbolMetaRunning()

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

		for _, e := range entries {
			if _, ok := h.entryCmd(cmdDef, e, dir); !ok {
				continue
			}
			results[e.Path] = runningMarker
		}
		cols[i].Results = results
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
	c := exec.CommandContext(ctx, built.Argv[0], built.Argv[1:]...)
	c.Dir = dir
	out, err := c.Output()
	if err != nil {
		var exitErr *exec.ExitError
		exitCode := -1
		var stderr string
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
			stderr = strings.TrimSpace(string(exitErr.Stderr))
		}
		return "", &runFailure{ExitCode: exitCode, Stderr: stderr, ConfCmd: cmd, ExpandedCmd: built.Expanded, err: err}
	}
	return strings.TrimRight(string(out), "\r\n"), nil
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

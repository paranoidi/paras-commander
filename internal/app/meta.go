package app

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdmacro"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// metaWakePayload wakes PollEvent after a meta background worker completes one entry.
// It carries the result so the event handler can apply it on the main goroutine, preventing
// concurrent map access between background workers and the render path.
type metaWakePayload struct {
	panelID   int
	entryName string
	path      string
	value     string
	gen       uint64
}

// metaExecFailedPayload notifies the main goroutine once per meta run when a shell command fails.
type metaExecFailedPayload struct {
	panelID int
	gen     uint64
}

func (a *App) postMetaResult(panelID int, entryName, path, value string, gen uint64) {
	_ = a.screen.PostEvent(tcell.NewEventInterrupt(metaWakePayload{
		panelID:   panelID,
		entryName: entryName,
		path:      path,
		value:     value,
		gen:       gen,
	}))
}

func (a *App) postMetaExecFailed(panelID int, gen uint64) {
	_ = a.screen.PostEvent(tcell.NewEventInterrupt(metaExecFailedPayload{
		panelID: panelID,
		gen:     gen,
	}))
}

func (a *App) applyMetaWakeResult(d metaWakePayload) {
	cols := a.model.MetaResults[d.panelID]
	for i := range cols {
		if cols[i].EntryName != d.entryName {
			continue
		}
		if cols[i].Results == nil {
			cols[i].Results = make(map[string]string)
		}
		cols[i].Results[d.path] = d.value
		return
	}
}

func (a *App) metaConfigDir() string {
	return strings.TrimSpace(a.paths.ConfigDir)
}

// loadMetaFile resolves and loads the meta.toml for the given panel path.
// Returns nil when no file is found (not an error). Warnings are shown as transient messages.
func (a *App) loadMetaFile(panelID int) *metacmds.MetaFile {
	path, warns := metacmds.ResolveMetaTOML(a.config, a.model.UserHomeDir, a.metaConfigDir(), a.panelByID(panelID).PathString())
	for _, w := range warns {
		a.setTransientMessage(w, ui.MessageUrgencyWarn)
	}
	if path == "" {
		return nil
	}
	mf, err := metacmds.LoadFile(path)
	if err != nil {
		a.setTransientMessage("meta: "+err.Error(), ui.MessageUrgencyCritical)
		return nil
	}
	return mf
}

func (a *App) ensureGlobalMetaStub() (path string, err error) {
	path = metacmds.ResolveMetaGlobalPath(a.config, a.model.UserHomeDir, a.metaConfigDir())
	if path == "" {
		return "", fmt.Errorf("meta: no global meta path configured")
	}
	if _, err := metacmds.WriteMetaStub(path); err != nil {
		return "", err
	}
	return path, nil
}

func (a *App) clearMetaCache() {
	a.metaCacheMu.Lock()
	a.metaCache = nil
	a.metaCacheMu.Unlock()
}

func (a *App) rerunActiveMetaPanels() {
	for panelID := range a.metaActiveEntries {
		if len(a.metaActiveEntries[panelID]) == 0 {
			continue
		}
		mf := a.loadMetaFile(panelID)
		if mf == nil {
			continue
		}
		sorted := metacmds.SortEntriesForDisplay(a.metaActiveEntries[panelID], mf)
		if len(sorted) == 0 {
			continue
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
		a.runMetaForPanel(panelID, sorted, cols)
	}
}

func (a *App) openMetaFileEditor(path string) bool {
	changed, err := metacmds.RefreshDocumentation(path)
	if err != nil {
		a.setErrorMessage("Meta commands", err)
		return false
	}
	if err := a.openFileInExternalEditor(path); err != nil {
		a.setErrorMessage("Meta commands", err)
		return false
	}
	a.clearMetaCache()
	a.rerunActiveMetaPanels()
	if changed {
		a.setTransientMessage("Meta commands: updated documentation in "+path, ui.MessageUrgencyInfo)
	} else {
		a.setTransientMessage("Meta commands: edited "+path, ui.MessageUrgencyInfo)
	}
	a.render()
	return true
}

// editMetaFile opens the meta.toml in an external editor, creating a stub if it does not exist.
func (a *App) editMetaFile() {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	path, err := a.resolveMetaEditPath(a.model.ActivePanel)
	if err != nil {
		a.setErrorMessage("Meta commands", err)
		return
	}
	a.openMetaFileEditor(path)
}

// openMetaDialog opens the checkbox meta command picker for the given panel.
func (a *App) openMetaDialog(panelID int) {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	mf := a.loadMetaFile(panelID)
	entries := metaDialogEntries(mf)

	activeSet := make(map[string]struct{}, len(a.metaActiveEntries[panelID]))
	for _, n := range a.metaActiveEntries[panelID] {
		activeSet[n] = struct{}{}
	}
	checked := make([]bool, len(entries))
	for i, e := range entries {
		_, checked[i] = activeSet[e.Name]
	}

	a.model.MetaDialog = ui.MetaDialogState{
		Open:    true,
		PanelID: panelID,
		Entries: entries,
		Checked: checked,
		Focus:   0,
	}
	a.clearTransientMessage()
}

func (a *App) closeMetaDialog() {
	a.model.MetaDialog = ui.MetaDialogState{}
}

// metaEntries returns MetaEntry slice sorted by order+name from a MetaFile (nil-safe).
func metaEntries(mf *metacmds.MetaFile) []ui.MetaEntry {
	if mf == nil || len(mf.Entries) == 0 {
		return nil
	}
	sorted := metacmds.SortedEntries(mf)
	out := make([]ui.MetaEntry, len(sorted))
	for i, e := range sorted {
		out[i] = ui.MetaEntry{Name: e.Name, Description: e.Description}
	}
	return out
}

func (a *App) activateMetaSelection() {
	st := a.model.MetaDialog
	panelID := st.PanelID
	checked := append([]bool(nil), st.Checked...)
	entries := append([]ui.MetaEntry(nil), st.Entries...)
	a.closeMetaDialog()

	var activeNames []string
	for i, on := range checked {
		if on && i < len(entries) {
			activeNames = append(activeNames, entries[i].Name)
		}
	}

	if len(activeNames) == 0 {
		if a.metaCancel[panelID] != nil {
			a.metaCancel[panelID]()
			a.metaCancel[panelID] = nil
		}
		a.model.MetaResults[panelID] = nil
		a.metaActiveEntries[panelID] = nil
		a.metaNavPath[panelID] = ""
		return
	}

	mf := a.loadMetaFile(panelID)
	if mf == nil {
		return
	}

	maxCols := config.DefaultMetaMaxActiveColumns
	if len(activeNames) > maxCols {
		a.setTransientMessage(fmt.Sprintf("meta: showing first %d of %d selected columns", maxCols, len(activeNames)), ui.MessageUrgencyWarn)
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
	a.metaActiveEntries[panelID] = names
	a.metaNavPath[panelID] = filepath.Clean(a.panelByID(panelID).PathString())
	a.runMetaForPanel(panelID, sorted, cols)
}

// handleMetaPanelDirChanged re-runs active meta commands when the panel navigates to a new directory.
func (a *App) handleMetaPanelDirChanged(panelID int) {
	if len(a.model.MetaResults[panelID]) == 0 {
		return
	}
	if len(a.metaActiveEntries[panelID]) == 0 {
		return
	}
	cur := filepath.Clean(a.panelByID(panelID).PathString())
	if a.metaNavPath[panelID] == cur {
		return
	}
	a.metaNavPath[panelID] = cur

	mf := a.loadMetaFile(panelID)
	if mf == nil {
		return
	}
	sorted := metacmds.SortEntriesForDisplay(a.metaActiveEntries[panelID], mf)
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
	a.runMetaForPanel(panelID, sorted, cols)
}

// metaDispatchItem holds a single entry selected for meta command execution.
type metaDispatchItem struct {
	entry localfs.Entry
	cmd   string
}

// runMetaForPanel runs meta commands for every active entry on panelID using worker pools.
func (a *App) runMetaForPanel(panelID int, cmdDefs []metacmds.MetaEntry, cols []ui.MetaColumnState) {
	panel := a.panelByID(panelID)
	entries := append([]localfs.Entry(nil), panel.Entries...)
	dir := panel.PathString()

	if a.metaCancel[panelID] != nil {
		a.metaCancel[panelID]()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.metaCancel[panelID] = cancel
	a.metaRunGen[panelID]++
	gen := a.metaRunGen[panelID]

	runningMarker := a.styles.SymbolMetaRunning()

	for i, cmdDef := range cmdDefs {
		results := make(map[string]string, len(entries))
		for _, e := range entries {
			results[e.Path] = ""
		}
		if cmdDef.Cache {
			a.metaCacheMu.RLock()
			if a.metaCache != nil {
				if cmdCache := a.metaCache[cmdDef.Name]; cmdCache != nil {
					for _, e := range entries {
						if v, ok := cmdCache[e.Path]; ok {
							results[e.Path] = v
						}
					}
				}
			}
			a.metaCacheMu.RUnlock()
		}

		for _, e := range entries {
			if _, ok := a.metaEntryCmd(cmdDef, e, dir); !ok {
				continue
			}
			results[e.Path] = runningMarker
		}
		cols[i].Results = results
	}

	a.model.MetaResults[panelID] = cols

	go func() {
		var wg sync.WaitGroup
		var notifyExecFailed sync.Once

		for _, cmdDef := range cmdDefs {
			cmdDef := cmdDef
			workers := cmdDef.Workers
			if workers < 1 {
				workers = a.config.Meta.DefaultEntryWorkers
			}

			var items []metaDispatchItem
			for _, e := range entries {
				cmd, ok := a.metaEntryCmd(cmdDef, e, dir)
				if !ok {
					continue
				}
				items = append(items, metaDispatchItem{entry: e, cmd: cmd})
			}

			sem := make(chan struct{}, workers)
			for _, item := range items {
				item := item
				wg.Add(1)
				sem <- struct{}{}
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					out, err := runMetaCommand(ctx, item.cmd, item.entry.Path, dir)
					if err != nil {
						if ctx.Err() != nil {
							return
						}
						notifyExecFailed.Do(func() { a.postMetaExecFailed(panelID, gen) })
						a.postMetaResult(panelID, cmdDef.Name, item.entry.Path, "", gen)
						return
					}
					if cmdDef.Cache {
						a.metaCacheMu.Lock()
						if a.metaCache == nil {
							a.metaCache = make(map[string]map[string]string)
						}
						if a.metaCache[cmdDef.Name] == nil {
							a.metaCache[cmdDef.Name] = make(map[string]string)
						}
						a.metaCache[cmdDef.Name][item.entry.Path] = out
						a.metaCacheMu.Unlock()
					}
					a.postMetaResult(panelID, cmdDef.Name, item.entry.Path, out, gen)
				}()
			}
		}
		wg.Wait()
	}()
}

// metaEntryCmd returns the shell command to run for entry e under cmdDef.
// File rows are filtered via when only; directories use dirs.
// Returns ("", false) when the entry should not be dispatched (filtered, no command, or cached).
func (a *App) metaEntryCmd(cmdDef metacmds.MetaEntry, e localfs.Entry, dir string) (cmd string, ok bool) {
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
		a.metaCacheMu.RLock()
		var hit bool
		if a.metaCache != nil {
			if cc := a.metaCache[cmdDef.Name]; cc != nil {
				_, hit = cc[e.Path]
			}
		}
		a.metaCacheMu.RUnlock()
		if hit {
			return "", false
		}
	}
	return cmd, true
}

// runMetaCommand runs a shell command template with %f expanded to path.
// Returns trimmed stdout on success. On failure returns a non-nil error (including context cancellation).
func runMetaCommand(ctx context.Context, cmd, path, dir string) (string, error) {
	built, err := cmdrun.BuildInvocation(cmdrun.InvocationSpec{
		Template: cmd,
		Mode:     cmdrun.ModeShellScript,
		Ctx:      cmdmacro.Context{RowPath: path},
	})
	if err != nil {
		return "", err
	}
	c := exec.CommandContext(ctx, built.Argv[0], built.Argv[1:]...)
	c.Dir = dir
	out, err := c.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

func (a *App) handleMetaDialogKey(event *tcell.EventKey) {
	st := &a.model.MetaDialog
	n := len(st.Entries)
	form := ui.NewDialogLinearForm(n)

	if ui.AltDialogOK(event) {
		a.activateMetaSelection()
		return
	}
	if ui.AltDialogCancel(event) {
		a.closeMetaDialog()
		return
	}

	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		if i, ok := ui.MetaEntryIndexForAltShortcut(st.Entries, event.Rune()); ok {
			st.Checked[i] = !st.Checked[i]
			st.Focus = i
			return
		}
	}

	switch event.Key() {
	case tcell.KeyF4:
		a.editMetaConfigFromDialog()
		return
	case tcell.KeyEsc:
		a.closeMetaDialog()
	case tcell.KeyEnter:
		switch st.Focus {
		case form.CancelIndex():
			a.closeMetaDialog()
		case form.OKIndex():
			a.activateMetaSelection()
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
			a.activateMetaSelection()
			return
		case 'c', 'C':
			a.closeMetaDialog()
			return
		case ' ':
			switch {
			case st.Focus < n:
				st.Checked[st.Focus] = !st.Checked[st.Focus]
			case st.Focus == form.OKIndex():
				a.activateMetaSelection()
			case st.Focus == form.CancelIndex():
				a.closeMetaDialog()
			}
			return
		}
	}
	if focus, ok := form.MoveFocus(st.Focus, event.Key()); ok {
		st.Focus = focus
	}
}

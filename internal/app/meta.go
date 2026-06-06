package app

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// metaWakePayload wakes PollEvent after a meta background worker completes one entry.
// It carries the result so the event handler can apply it on the main goroutine, preventing
// concurrent map access between background workers and the render path.
type metaWakePayload struct {
	panelID int
	path    string
	value   string
	gen     uint64
}

func (a *App) postMetaResult(panelID int, path, value string, gen uint64) {
	_ = a.screen.PostEvent(tcell.NewEventInterrupt(metaWakePayload{
		panelID: panelID,
		path:    path,
		value:   value,
		gen:     gen,
	}))
}

func (a *App) metaConfigDir() string {
	return strings.TrimSpace(a.paths.ConfigDir)
}

// loadMetaFile resolves and loads the meta.toml for the active panel path.
// Returns nil when no file is found (not an error). Warnings are shown as transient messages.
func (a *App) loadMetaFile() *metacmds.MetaFile {
	path, warns := metacmds.ResolveMetaTOML(a.config, a.model.UserHomeDir, a.metaConfigDir(), a.activePanel().PathString())
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
	path, err := a.resolveMetaEditPath()
	if err != nil {
		a.setErrorMessage("Meta commands", err)
		return
	}
	a.openMetaFileEditor(path)
}

// openMetaDialog opens the radio-button meta command picker for the given panel.
func (a *App) openMetaDialog(panelID int) {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	mf := a.loadMetaFile()
	entries := metaDialogEntries(mf)

	// Pre-select the currently active command.
	selected := 0
	for i, e := range entries {
		if e.Name == a.metaActiveCmd[panelID] {
			selected = i
			break
		}
	}

	a.model.MetaDialog = ui.MetaDialogState{
		Open:     true,
		PanelID:  panelID,
		Entries:  entries,
		Selected: selected,
		Focus:    selected,
	}
	a.clearTransientMessage()
}

func (a *App) closeMetaDialog() {
	a.model.MetaDialog = ui.MetaDialogState{}
}

// metaEntries returns MetaEntry slice sorted by name from a MetaFile (nil-safe).
func metaEntries(mf *metacmds.MetaFile) []ui.MetaEntry {
	if mf == nil || len(mf.Entries) == 0 {
		return nil
	}
	sorted := append([]metacmds.MetaEntry(nil), mf.Entries...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	out := make([]ui.MetaEntry, len(sorted))
	for i, e := range sorted {
		out[i] = ui.MetaEntry{Name: e.Name, Description: e.Description}
	}
	return out
}

func (a *App) activateMetaSelection() {
	st := &a.model.MetaDialog
	if st.Selected < 0 || st.Selected >= len(st.Entries) {
		return
	}
	panelID := st.PanelID
	entry := st.Entries[st.Selected]
	a.closeMetaDialog()

	if entry.Name == "none" {
		// Clear meta for this panel and cancel any in-flight run.
		if a.metaCancel[panelID] != nil {
			a.metaCancel[panelID]()
			a.metaCancel[panelID] = nil
		}
		a.model.MetaResults[panelID] = nil
		a.metaActiveCmd[panelID] = ""
		a.metaNavPath[panelID] = ""
		return
	}

	mf := a.loadMetaFile()
	if mf == nil {
		return
	}
	cmdDef, ok := mf.EntryByName(entry.Name)
	if !ok {
		return
	}
	a.metaActiveCmd[panelID] = entry.Name
	a.metaNavPath[panelID] = filepath.Clean(a.panelByID(panelID).PathString())
	a.runMetaForPanel(panelID, cmdDef)
}

// handleMetaPanelDirChanged re-runs the active meta command when the panel navigates to a new directory.
func (a *App) handleMetaPanelDirChanged(panelID int) {
	if a.model.MetaResults[panelID] == nil {
		return
	}
	cmdName := a.metaActiveCmd[panelID]
	if cmdName == "" {
		return
	}
	cur := filepath.Clean(a.panelByID(panelID).PathString())
	if a.metaNavPath[panelID] == cur {
		return
	}
	a.metaNavPath[panelID] = cur

	mf := a.loadMetaFile()
	if mf == nil {
		return
	}
	cmdDef, ok := mf.EntryByName(cmdName)
	if !ok {
		return
	}
	a.runMetaForPanel(panelID, cmdDef)
}

// metaDispatchItem holds a single entry selected for meta command execution.
type metaDispatchItem struct {
	entry localfs.Entry
	cmd   string
}

// runMetaForPanel runs the meta command for every entry in panelID using a worker pool.
// When cmdDef.Cache is true, cached results from previous runs are reused and only
// uncached entries are dispatched to workers.
// Any in-flight run for the same panel is cancelled before the new one starts.
func (a *App) runMetaForPanel(panelID int, cmdDef metacmds.MetaEntry) {
	panel := a.panelByID(panelID)
	entries := append([]localfs.Entry(nil), panel.Entries...)
	dir := panel.PathString()
	workers := cmdDef.Workers
	if workers < 1 {
		workers = a.config.Meta.DefaultEntryWorkers
	}

	// Cancel any in-flight run for this panel and start a new generation.
	if a.metaCancel[panelID] != nil {
		a.metaCancel[panelID]()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.metaCancel[panelID] = cancel
	a.metaRunGen[panelID]++
	gen := a.metaRunGen[panelID]

	// Initialize results map. Pre-populate with cached values when caching is on.
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

	// Determine which entries will be dispatched and mark them with the running indicator.
	// This pre-pass runs on the main goroutine so map writes are safe (no concurrent readers yet).
	runningMarker := a.styles.SymbolMetaRunning()
	var items []metaDispatchItem
	for _, e := range entries {
		cmd, ok := a.metaEntryCmd(cmdDef, e, dir)
		if !ok {
			continue
		}
		results[e.Path] = runningMarker
		items = append(items, metaDispatchItem{entry: e, cmd: cmd})
	}

	a.model.MetaResults[panelID] = results

	go func() {
		sem := make(chan struct{}, workers)
		var wg sync.WaitGroup

		for _, item := range items {
			item := item
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				out, apply := runMetaCommand(ctx, item.cmd, item.entry.Path, dir)
				if !apply {
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
				a.postMetaResult(panelID, item.entry.Path, out, gen)
			}()
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

// runMetaCommand runs a single shell command with path as $1.
// Returns (output, true) on success or real error; (_, false) when the context was cancelled.
func runMetaCommand(ctx context.Context, cmd, path, dir string) (string, bool) {
	c := exec.CommandContext(ctx, "sh", "-c", cmd, "sh", path)
	c.Dir = dir
	out, err := c.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", false
		}
		return "", true
	}
	return strings.TrimRight(string(out), "\r\n"), true
}

func (a *App) handleMetaDialogKey(event *tcell.EventKey) {
	st := &a.model.MetaDialog
	n := len(st.Entries)
	form := ui.NewDialogLinearForm(n)

	// Alt+O = OK, Alt+C = Cancel
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
			st.Selected = i
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
		default:
			// Enter on radio item selects it; Enter on OK confirms.
			if st.Focus < n {
				st.Selected = st.Focus
			}
			a.activateMetaSelection()
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
			// Space on a radio item selects it; on buttons activates.
			switch {
			case st.Focus < n:
				st.Selected = st.Focus
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

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
	"github.com/paranoidi/paras-commander/internal/cmdmacro"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// metaWakePayload wakes PollEvent after meta background run mutations.
type metaWakePayload struct{}

func (a *App) postMetaWake() {
	_ = a.screen.PostEvent(tcell.NewEventInterrupt(metaWakePayload{}))
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

func (a *App) openMetaFileEditor(path string) {
	if err := a.openFileInExternalEditor(path); err != nil {
		a.setErrorMessage("Meta commands", err)
		return
	}
	a.setTransientMessage("Meta commands: edited "+path, ui.MessageUrgencyInfo)
	a.render()
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
		// Clear meta for this panel.
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

// runMetaForPanel runs the meta command for every entry in panelID using a worker pool.
// When cmdDef.Cache is true, cached results from previous runs are reused and only
// uncached entries are dispatched to workers.
func (a *App) runMetaForPanel(panelID int, cmdDef metacmds.MetaEntry) {
	panel := a.panelByID(panelID)
	entries := append([]localfs.Entry(nil), panel.Entries...)
	dir := panel.PathString()
	workers := a.config.Meta.Workers
	if workers < 1 {
		workers = 1
	}

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
	a.model.MetaResults[panelID] = results

	go func() {
		var mu sync.Mutex
		sem := make(chan struct{}, workers)
		var wg sync.WaitGroup

		for _, e := range entries {
			ok, err := cmdDef.MatchesRow(e, dir)
			if err != nil {
				continue
			}
			if !ok {
				continue
			}
			// Skip entries already resolved from cache.
			if cmdDef.Cache {
				a.metaCacheMu.RLock()
				var hit bool
				if a.metaCache != nil {
					if cmdCache := a.metaCache[cmdDef.Name]; cmdCache != nil {
						_, hit = cmdCache[e.Path]
					}
				}
				a.metaCacheMu.RUnlock()
				if hit {
					continue
				}
			}
			var cmd string
			if e.Type == localfs.EntryDirectory {
				cmd = cmdDef.Dirs
			} else {
				cmd = cmdDef.File
			}
			if cmd == "" {
				continue
			}
			e := e
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				out := runMetaCommand(cmd, e.Path, dir)
				mu.Lock()
				results[e.Path] = out
				mu.Unlock()
				if cmdDef.Cache {
					a.metaCacheMu.Lock()
					if a.metaCache == nil {
						a.metaCache = make(map[string]map[string]string)
					}
					if a.metaCache[cmdDef.Name] == nil {
						a.metaCache[cmdDef.Name] = make(map[string]string)
					}
					a.metaCache[cmdDef.Name][e.Path] = out
					a.metaCacheMu.Unlock()
				}
				a.postMetaWake()
			}()
		}
		wg.Wait()
	}()
}

// runMetaCommand runs a shell command template with %f expanded to path.
func runMetaCommand(cmd, path, dir string) string {
	built, err := cmdrun.BuildInvocation(cmdrun.InvocationSpec{
		Template: cmd,
		Mode:     cmdrun.ModeShellScript,
		Ctx:      cmdmacro.Context{RowPath: path},
	})
	if err != nil {
		return ""
	}
	c := exec.CommandContext(context.Background(), built.Argv[0], built.Argv[1:]...)
	c.Dir = dir
	out, err := c.Output()
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(out), "\r\n")
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

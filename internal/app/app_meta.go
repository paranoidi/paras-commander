package app

import (
	"context"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// metaWakePayload wakes PollEvent after meta background run mutations.
type metaWakePayload struct{}

func (a *App) postMetaWake() {
	_ = a.screen.PostEvent(tcell.NewEventInterrupt(metaWakePayload{}))
}

// openMetaDialog opens the radio-button meta command picker for the given panel.
func (a *App) openMetaDialog(panelID int) {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	cmds := sortedMetaEntries(a.config.Meta)
	// "None" is always the first choice.
	entries := make([]ui.MetaEntry, 0, 1+len(cmds))
	entries = append(entries, ui.MetaEntry{Name: "none", Description: "None (clear)"})
	entries = append(entries, cmds...)

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

// sortedMetaEntries returns MetaEntry slice sorted by name from the config map.
func sortedMetaEntries(meta map[string]config.MetaCommandDef) []ui.MetaEntry {
	if len(meta) == 0 {
		return nil
	}
	names := make([]string, 0, len(meta))
	for k := range meta {
		names = append(names, k)
	}
	sort.Strings(names)
	out := make([]ui.MetaEntry, len(names))
	for i, n := range names {
		out[i] = ui.MetaEntry{Name: n, Description: meta[n].Description}
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

	cmdDef, ok := a.config.Meta[entry.Name]
	if !ok {
		return
	}
	a.metaActiveCmd[panelID] = entry.Name
	a.metaNavPath[panelID] = filepath.Clean(a.panelByID(panelID).Path)
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
	cur := filepath.Clean(a.panelByID(panelID).Path)
	if a.metaNavPath[panelID] == cur {
		return
	}
	a.metaNavPath[panelID] = cur
	cmdDef, ok := a.config.Meta[cmdName]
	if !ok {
		return
	}
	a.runMetaForPanel(panelID, cmdDef)
}

// runMetaForPanel runs the meta command for every entry in panelID in background.
func (a *App) runMetaForPanel(panelID int, cmdDef config.MetaCommandDef) {
	panel := a.panelByID(panelID)
	entries := append([]localfs.Entry(nil), panel.Entries...)
	dir := panel.Path

	// Initialize results map (empty string = not yet computed).
	results := make(map[string]string, len(entries))
	for _, e := range entries {
		results[e.Path] = ""
	}
	a.model.MetaResults[panelID] = results

	go func() {
		var mu sync.Mutex
		for _, e := range entries {
			var cmd string
			if e.Type == localfs.EntryDirectory {
				cmd = cmdDef.Dirs
			} else {
				cmd = cmdDef.File
			}
			if cmd == "" {
				continue
			}
			out := runMetaCommand(cmd, e.Path, dir)
			mu.Lock()
			results[e.Path] = out
			mu.Unlock()
			a.postMetaWake()
		}
	}()
}

// runMetaCommand runs a single shell command with path as $1.
func runMetaCommand(cmd, path, dir string) string {
	c := exec.CommandContext(context.Background(), "sh", "-c", cmd, "sh", path)
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
	if event.Key() == tcell.KeyRune && event.Modifiers() == tcell.ModAlt {
		switch event.Rune() {
		case 'o', 'O':
			a.activateMetaSelection()
			return
		case 'c', 'C':
			a.closeMetaDialog()
			return
		}
	}

	switch event.Key() {
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


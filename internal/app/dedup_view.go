package app

import (
	"github.com/gdamore/tcell/v2"
	dedupctrl "github.com/paranoidi/paras-commander/internal/apphandler/dedup"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// dedupHost adapts *App to the dedup handler's Host interface.
type dedupHost struct {
	appShellHost
}

func (h dedupHost) NavigatePanelToPath(panelID int, path string, selectName string) error {
	return h.app.navigatePanelToDirectory(panelID, path, selectName)
}

func (h dedupHost) EnqueueDeleteJob(paths []string) { h.app.enqueueDeleteJob(paths) }

func (h dedupHost) DedupMenuDefinitions() []menu.Definition { return h.app.dedupMenuDefinitions() }

func (h dedupHost) BrowserMenuDefinitions() []menu.Definition { return h.app.browserMenuDefinitions() }

func (a *App) openFindDuplicates() { a.dedupCtrl.Open() }

// openDedupDeleteDialog opens the standard delete confirmation for the marked
// duplicate files, reusing the browser dialog (list + impact summary + Yes/No).
func (a *App) openDedupDeleteDialog() {
	st := a.model.DedupView
	var entries []dialog.DeleteListEntry
	for _, e := range a.model.DedupList {
		if st.Marked[e.AbsKey] {
			entries = append(entries, dialog.DeleteListEntry{
				Name: e.File.Rel,
				Path: e.AbsKey,
				Type: localfs.EntryFile,
			})
		}
	}
	if len(entries) == 0 {
		return
	}
	fd := dialog.FileDialogState{
		Open:          true,
		DialogType:    dialog.FileDialogDelete,
		DeleteSummary: ui.FormatDeleteImpactSummary(int64(st.MarkedCount), st.MarkedReclaimBytes, false, a.styles.SymbolWorking()),
		DeleteEntries: entries,
		FocusedField:  1, // No (safe default); Yes stays index 0.
	}
	fd.DeleteLayoutMinWidth = dialog.ComputeDeleteDialogLayoutMinWidth(fd, ui.DialogListIconLeadingWidth(a.model.ShowFileIcons))
	a.model.FileDialog = fd
}

func (a *App) closeDedupView() { a.dedupCtrl.Close() }

func (a *App) pollDedupUpdates(payload dedupctrl.WakePayload) bool {
	return a.dedupCtrl.PollUpdates(payload)
}

// tryDispatchDedup handles dedup-view actions from keys or the F9 menu.
func (a *App) tryDispatchDedup(actionID string) bool {
	if a.model.ViewMode != ui.ViewDedup {
		return false
	}
	switch actionID {
	case keymap.ActionDedupClose:
		a.closeDedupView()
		return true
	case keymap.ActionDedupRefresh:
		a.dedupCtrl.Refresh()
		return true
	case keymap.ActionDedupToggleSort:
		a.dedupCtrl.ToggleSortOrder()
		return true
	case keymap.ActionDedupToggleEmpty:
		a.dedupCtrl.ToggleIgnoreEmpty()
		return true
	case keymap.ActionDedupMarkRedundant:
		a.dedupCtrl.MarkRedundantUnderSelected()
		return true
	case keymap.ActionFileDelete:
		if len(a.dedupCtrl.MarkedPaths()) > 0 {
			a.openDedupDeleteDialog()
		} else {
			a.setTransientMessage("Mark files with Space first", ui.MessageUrgencyInfo)
		}
		return true
	default:
		return false
	}
}

func dedupViewFooterKeys(global, dedup *keymap.Map) []menu.FunctionKey {
	var out []menu.FunctionKey
	if dedup != nil {
		if lbl := dedup.MenuBindingLabel(keymap.ActionDedupRefresh); lbl != "" {
			out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Refresh"})
		}
		if lbl := dedup.MenuBindingLabel(keymap.ActionDedupToggleSort); lbl != "" {
			out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Sort"})
		}
		if lbl := dedup.MenuBindingLabel(keymap.ActionDedupMarkRedundant); lbl != "" {
			out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Keep uniques"})
		}
	}
	if global != nil {
		if lbl := global.MenuBindingLabel(keymap.ActionFileDelete); lbl != "" {
			out = append(out, menu.FunctionKey{Key: tcell.KeyF8, KeyLabel: lbl, Hint: "Delete"})
		}
	}
	return out
}

// dedupVisibleRows is the number of file rows the results list can show (title border,
// root header, and bottom border consume three rows of chrome — same as jobs/commands).
func (a *App) dedupVisibleRows() int {
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	rect := ui.MergeTwinPanelRects(layout.Primary, layout.Secondary, a.model.SplitOrientation)
	return ui.PanelListRows(rect)
}

func (a *App) handleDedupViewKey(event *tcell.EventKey) bool {
	snap := a.model.DedupSnapshot
	st := &a.model.DedupView

	// Confirmation gate before the (expensive) hashing phase.
	if snap.Phase == comparepkg.DedupAwaitConfirm {
		switch event.Key() {
		case tcell.KeyEnter:
			a.dedupCtrl.Confirm()
		case tcell.KeyEsc, tcell.KeyLeft:
			a.closeDedupView()
		}
		return false
	}

	nextAction := a.actionFromKeyEvent(event)
	if nextAction == keymap.ActionAppQuit {
		return a.handleQuit()
	}
	if nextAction == keymap.ActionAppQuitImmediate {
		return a.handleQuitImmediate()
	}
	if nextAction == keymap.ActionAppOpenMenu {
		a.openMenu()
		return false
	}
	if a.tryOpenMenuByShortcut(event) {
		return false
	}

	visible := a.dedupVisibleRows()
	n := a.dedupCtrl.ListLen()

	if nextAction != "" && a.tryDispatchDedup(nextAction) {
		return false
	}
	if nextAction != "" && a.tryDispatchAuxiliaryScreens(nextAction) {
		return false
	}

	switch nextAction {
	case keymap.ActionPanelSelectToggle:
		a.dedupCtrl.ToggleMark()
		if st.Selected < n-1 {
			st.Selected++
		}
		a.dedupCtrl.EnsureSelectionVisible(visible)
		return false
	}

	switch event.Key() {
	case tcell.KeyEsc:
		a.closeDedupView()
	case tcell.KeyUp:
		if st.Selected > 0 {
			st.Selected--
		}
		a.dedupCtrl.EnsureSelectionVisible(visible)
	case tcell.KeyDown:
		if st.Selected < n-1 {
			st.Selected++
		}
		a.dedupCtrl.EnsureSelectionVisible(visible)
	case tcell.KeyPgUp:
		st.Selected = max(0, st.Selected-visible)
		a.dedupCtrl.EnsureSelectionVisible(visible)
	case tcell.KeyPgDn:
		if n > 0 {
			st.Selected = min(n-1, st.Selected+visible)
		}
		a.dedupCtrl.EnsureSelectionVisible(visible)
	case tcell.KeyHome:
		st.Selected = 0
		a.dedupCtrl.EnsureSelectionVisible(visible)
	case tcell.KeyEnd:
		if n > 0 {
			st.Selected = n - 1
		}
		a.dedupCtrl.EnsureSelectionVisible(visible)
	case tcell.KeyEnter:
		a.dedupCtrl.NavigateFromSelection()
		a.closeDedupView()
	}
	return false
}

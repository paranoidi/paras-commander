package app

import (
	"github.com/gdamore/tcell/v2"
	dedupctrl "github.com/paranoidi/paras-commander/internal/apphandler/dedup"
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

func (h dedupHost) EnqueueDeleteJob(paths []string, removeEmptyDirs bool) {
	// Dedup has its own empty-dirs confirm (apphandler/dialog's ExecuteDelete opens it via
	// Deps.Dedup for the dedup-view branch); never double-prompt with the generic
	// dangling-dirs cleanup.
	h.app.jobsCtrl.EnqueueDeleteJob(paths, removeEmptyDirs, false)
}

func (h dedupHost) DedupMenuDefinitions() []menu.Definition { return h.app.dedupMenuDefinitions() }

func (h dedupHost) BrowserMenuDefinitions() []menu.Definition { return h.app.browserMenuDefinitions() }

func (a *App) openFindDuplicates() { a.dedupCtrl.Open() }

// openDedupDeleteDialog opens the standard delete confirmation for the marked
// duplicate files, reusing the browser dialog (list + impact summary + Yes/No).
func (a *App) openDedupDeleteDialog() {
	st := a.model.DedupView
	var entries []dialog.DeleteListEntry
	// Marked files come from the handler (group-based), not the visible rows, so
	// copies hidden inside collapsed tree nodes are listed too.
	for _, f := range a.dedupCtrl.MarkedFiles() {
		entries = append(entries, dialog.DeleteListEntry{
			Name: f.Rel,
			Path: f.Abs.String(),
			Type: localfs.EntryFile,
		})
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

func (a *App) handleDedupEmptyDirsConfirmKey(event *tcell.EventKey) bool {
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 'y', 'Y':
			a.model.DedupEmptyDirsConfirm = dialog.DedupEmptyDirsConfirmState{}
			a.dedupCtrl.DeleteMarked(true)
			return false
		case 'n', 'N':
			a.model.DedupEmptyDirsConfirm = dialog.DedupEmptyDirsConfirmState{}
			a.dedupCtrl.DeleteMarked(false)
			return false
		}
	}
	switch event.Key() {
	case tcell.KeyEsc:
		a.model.DedupEmptyDirsConfirm = dialog.DedupEmptyDirsConfirmState{}
		a.dedupCtrl.DeleteMarked(false)
	case tcell.KeyLeft:
		a.model.DedupEmptyDirsConfirm.Focus = dialog.DialogPairLeftRight(a.model.DedupEmptyDirsConfirm.Focus, false)
	case tcell.KeyRight:
		a.model.DedupEmptyDirsConfirm.Focus = dialog.DialogPairLeftRight(a.model.DedupEmptyDirsConfirm.Focus, true)
	case tcell.KeyEnter:
		removeEmpty := a.model.DedupEmptyDirsConfirm.Focus == 0
		a.model.DedupEmptyDirsConfirm = dialog.DedupEmptyDirsConfirmState{}
		a.dedupCtrl.DeleteMarked(removeEmpty)
	}
	return false
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
	case keymap.ActionPanelRefresh:
		a.dedupCtrl.Refresh()
		return true
	case keymap.ActionDedupToggleSort:
		a.dedupCtrl.ToggleSortOrder()
		return true
	case keymap.ActionDedupToggleEmpty:
		a.dedupCtrl.ToggleIgnoreEmpty()
		return true
	case keymap.ActionDedupToggleNode:
		a.dedupCtrl.DescendFromSelection()
		a.dedupCtrl.EnsureSelectionVisible(a.dedupVisibleRows())
		return true
	case keymap.ActionDedupCollapse:
		a.dedupCtrl.CollapseOrParent()
		a.dedupCtrl.EnsureSelectionVisible(a.dedupVisibleRows())
		return true
	case keymap.ActionDedupToggleTree:
		a.dedupCtrl.ToggleTreeMode()
		a.dedupCtrl.EnsureSelectionVisible(a.dedupVisibleRows())
		return true
	case keymap.ActionDedupCollapseAll:
		a.dedupCtrl.CollapseAll()
		a.dedupCtrl.EnsureSelectionVisible(a.dedupVisibleRows())
		return true
	case keymap.ActionDedupExpandAll:
		a.dedupCtrl.ExpandAll()
		a.dedupCtrl.EnsureSelectionVisible(a.dedupVisibleRows())
		return true
	case keymap.ActionDedupPrevDir:
		a.dedupCtrl.MoveToAdjacentDir(-1)
		a.dedupCtrl.EnsureSelectionVisible(a.dedupVisibleRows())
		return true
	case keymap.ActionDedupNextDir:
		a.dedupCtrl.MoveToAdjacentDir(1)
		a.dedupCtrl.EnsureSelectionVisible(a.dedupVisibleRows())
		return true
	case keymap.ActionDedupMarkKeep:
		a.dedupCtrl.KeepSelection()
		a.dedupCtrl.EnsureSelectionVisible(a.dedupVisibleRows())
		return true
	case keymap.ActionDedupCompare:
		if p, s, ok := a.dedupCtrl.CompareDirsFromSelection(); ok {
			a.compareCtrl.OpenPaths(p, s, a.activePanel().ShowHidden,
				func() { a.dedupCtrl.ReopenPreservingState() })
		}
		return true
	case keymap.ActionPanelClearSelection:
		a.dedupCtrl.ClearMarks()
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

func dedupViewFooterKeys(global, dedup *keymap.Map, treeDirs bool) []menu.FunctionKey {
	var out []menu.FunctionKey
	if global != nil {
		if lbl := global.MenuBindingLabel(keymap.ActionPanelRefresh); lbl != "" {
			out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Refresh"})
		}
	}
	if dedup != nil {
		if lbl := dedup.MenuBindingLabel(keymap.ActionDedupToggleTree); lbl != "" {
			out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Dirs/Groups"})
		}
		if !treeDirs {
			if lbl := dedup.MenuBindingLabel(keymap.ActionDedupToggleSort); lbl != "" {
				out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Sort"})
			}
		}
	}
	if global != nil {
		if lbl := global.MenuBindingLabel(keymap.ActionPanelClearSelection); lbl != "" {
			out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Unselect all"})
		}
	}
	if dedup != nil {
		if lbl := dedup.MenuBindingLabel(keymap.ActionDedupMarkKeep); lbl != "" {
			out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Keep"})
		}
		if lbl := dedup.MenuBindingLabel(keymap.ActionDedupCompare); lbl != "" {
			out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Compare"})
		}
	}
	if global != nil {
		if lbl := global.MenuBindingLabel(keymap.ActionFileDelete); lbl != "" {
			out = append(out, menu.FunctionKey{Key: tcell.KeyF8, KeyLabel: lbl, Hint: "Delete"})
		}
	}
	return out
}

// dedupVisibleRows is the number of file rows one tree pane can show (title
// border, header line, and bottom border consume three rows of chrome — same as
// jobs/commands). Both panes share the twin-panel split, so the primary rect's
// row count serves for clamping either pane.
func (a *App) dedupVisibleRows() int {
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	return ui.PanelListRows(layout.Primary)
}

func (a *App) handleDedupViewKey(event *tcell.EventKey) bool {
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

	if nextAction != "" && a.tryDispatchDedup(nextAction) {
		return false
	}
	if nextAction != "" && a.tryDispatchAuxiliaryScreens(nextAction) {
		return false
	}

	switch nextAction {
	case keymap.ActionPanelSelectToggle:
		a.dedupCtrl.SelectToggleAndAdvance()
		a.dedupCtrl.EnsureSelectionVisible(visible)
		return false
	case keymap.ActionPanelInvertSelection:
		if a.model.DedupView.FocusCopies {
			a.dedupCtrl.ToggleCopiesPaneSelectAll()
		}
		return false
	case keymap.ActionPanelSwitch:
		a.dedupCtrl.SwitchPane()
		return false
	}

	switch event.Key() {
	case tcell.KeyEsc:
		a.closeDedupView()
	case tcell.KeyUp:
		a.dedupCtrl.MoveSelection(-1)
		a.dedupCtrl.EnsureSelectionVisible(visible)
	case tcell.KeyDown:
		a.dedupCtrl.MoveSelection(1)
		a.dedupCtrl.EnsureSelectionVisible(visible)
	case tcell.KeyPgUp:
		a.dedupCtrl.MoveSelection(-visible)
		a.dedupCtrl.EnsureSelectionVisible(visible)
	case tcell.KeyPgDn:
		a.dedupCtrl.MoveSelection(visible)
		a.dedupCtrl.EnsureSelectionVisible(visible)
	case tcell.KeyHome:
		a.dedupCtrl.SelectEdge(false)
		a.dedupCtrl.EnsureSelectionVisible(visible)
	case tcell.KeyEnd:
		a.dedupCtrl.SelectEdge(true)
		a.dedupCtrl.EnsureSelectionVisible(visible)
	case tcell.KeyEnter:
		a.dedupCtrl.NavigateFromSelection()
		a.closeDedupView()
	}
	return false
}

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
		DeleteSummary: ui.FormatDeleteImpactSummary(int64(st.MarkedCount), st.MarkedReclaimBytes, false, a.styles.IconWorking()),
		DeleteEntries: entries,
		FocusedField:  1, // No (safe default); Yes stays index 0.
	}
	fd.DeleteLayoutMinWidth = dialog.ComputeDeleteDialogLayoutMinWidth(fd, ui.DialogListIconLeadingWidth(a.model.UseNerdfontIcons))
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

// closeDedupView leaves the view but keeps its results for ActionDedupOpen.
func (a *App) closeDedupView() {
	a.dedupCtrl.Leave()
	if lbl := a.keys.Global.MenuBindingLabel(keymap.ActionDedupOpen); lbl != "" {
		a.setTransientMessage("Duplicates kept \u2014 "+lbl+" returns", ui.MessageUrgencyInfo)
	}
}

// showKeptDuplicates brings the kept results back (ActionDedupOpen).
func (a *App) showKeptDuplicates() {
	if !a.dedupCtrl.HasResults() {
		a.setTransientMessage("No duplicates results \u2014 run Find duplicates first", ui.MessageUrgencyInfo)
		return
	}
	a.dedupCtrl.VerifyAndShowKept()
}

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
	case keymap.ActionPanelTreeCollapseAll:
		a.dedupCtrl.CollapseLevel()
		a.dedupCtrl.EnsureSelectionVisible(a.dedupVisibleRows())
		return true
	case keymap.ActionPanelTreeExpandAllShallow:
		a.dedupCtrl.ExpandLevel()
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
	case keymap.ActionOpenInPrimary, keymap.ActionOpenInSecondary:
		if path, isDir, ok := a.dedupCtrl.PaneTarget(a.model.DedupView.FocusCopies); ok {
			a.dedupCtrl.OpenInPanel(openInPanelID(actionID), path, isDir)
		}
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
			msg := "Mark files to keep first"
			if lbl := a.keys.Dedup.MenuBindingLabel(keymap.ActionDedupMarkKeep); lbl != "" {
				msg = "Mark files to keep with " + lbl + " first"
			}
			a.setTransientMessage(msg, ui.MessageUrgencyInfo)
		}
		return true
	default:
		return false
	}
}

func openInPanelID(actionID string) int {
	if actionID == keymap.ActionOpenInSecondary {
		return ui.SecondaryPanel
	}
	return ui.PrimaryPanel
}

func dedupViewFooterKeys(global, dedup *keymap.Map, treeDirs bool) []menu.FunctionKey {
	var out []menu.FunctionKey
	if global != nil {
		if lbl := global.MenuBindingLabel(keymap.ActionPanelRefresh); lbl != "" {
			out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Refresh", ActionID: keymap.ActionPanelRefresh})
		}
	}
	if dedup != nil {
		if lbl := dedup.MenuBindingLabel(keymap.ActionDedupToggleTree); lbl != "" {
			out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Dirs/Groups", ActionID: keymap.ActionDedupToggleTree})
		}
		if !treeDirs {
			if lbl := dedup.MenuBindingLabel(keymap.ActionDedupToggleSort); lbl != "" {
				out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Sort", ActionID: keymap.ActionDedupToggleSort})
			}
		}
	}
	if global != nil {
		if lbl := global.MenuBindingLabel(keymap.ActionPanelClearSelection); lbl != "" {
			out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Unselect all", ActionID: keymap.ActionPanelClearSelection})
		}
	}
	if dedup != nil {
		if lbl := dedup.MenuBindingLabel(keymap.ActionDedupMarkKeep); lbl != "" {
			out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Keep", ActionID: keymap.ActionDedupMarkKeep})
		}
		if lbl := dedup.MenuBindingLabel(keymap.ActionDedupCompare); lbl != "" {
			out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Compare", ActionID: keymap.ActionDedupCompare})
		}
		if lbl := dedup.MenuBindingLabel(keymap.ActionOpenInPrimary); lbl != "" {
			out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Open ◄", ActionID: keymap.ActionOpenInPrimary})
		}
		if lbl := dedup.MenuBindingLabel(keymap.ActionOpenInSecondary); lbl != "" {
			out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: "Open ►", ActionID: keymap.ActionOpenInSecondary})
		}
	}
	if global != nil {
		if lbl := global.MenuBindingLabel(keymap.ActionFileDelete); lbl != "" {
			out = append(out, menu.FunctionKey{Key: tcell.KeyF8, KeyLabel: lbl, Hint: "Delete", ActionID: keymap.ActionFileDelete})
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
	if a.model.DedupView.FocusCopies {
		copies, _ := ui.DedupSecondaryRects(layout.Secondary, a.model.DedupView.TreeDirs)
		return ui.PanelListRows(copies)
	}
	return ui.PanelListRows(layout.Primary)
}

func (a *App) handleDedupViewKey(event *tcell.EventKey) bool {
	if a.model.ViMotionMode {
		event = keymap.RemapViMotionKey(event)
	}
	nextAction := a.actionFromKeyEvent(event)
	if result, handled := a.dispatchAuxiliaryViewCommonKeys(event, nextAction); handled {
		return result
	}

	if nextAction != "" && a.tryDispatchAuxiliaryScreens(nextAction) {
		return false
	}
	if a.model.DedupView.FocusPanel {
		return a.handleDedupPanelKey(nextAction, event)
	}
	visible := a.dedupVisibleRows()

	if nextAction != "" && a.tryDispatchDedup(nextAction) {
		return false
	}

	switch nextAction {
	case keymap.ActionPanelSelectToggle:
		a.dedupCtrl.SelectToggleAndAdvance()
		a.dedupCtrl.EnsureSelectionVisible(visible)
		return false
	case keymap.ActionPanelPinToggle:
		a.pinCtrl.ToggleDedupSelection()
		return false
	case keymap.ActionPanelInvertSelection:
		if a.model.DedupView.FocusCopies {
			a.dedupCtrl.ToggleCopiesPaneSelectAll()
		}
		return false
	case keymap.ActionPanelSwitch:
		a.dedupCtrl.SwitchPane()
		return false
	case keymap.ActionNavUp:
		a.dedupCtrl.MoveSelection(-1)
		a.dedupCtrl.EnsureSelectionVisible(visible)
	case keymap.ActionNavDown:
		a.dedupCtrl.MoveSelection(1)
		a.dedupCtrl.EnsureSelectionVisible(visible)
	case keymap.ActionNavPageUp:
		a.dedupCtrl.MoveSelection(-visible)
		a.dedupCtrl.EnsureSelectionVisible(visible)
	case keymap.ActionNavPageDown:
		a.dedupCtrl.MoveSelection(visible)
		a.dedupCtrl.EnsureSelectionVisible(visible)
	case keymap.ActionNavTop:
		a.dedupCtrl.SelectEdge(false)
		a.dedupCtrl.EnsureSelectionVisible(visible)
	case keymap.ActionNavBottom:
		a.dedupCtrl.SelectEdge(true)
		a.dedupCtrl.EnsureSelectionVisible(visible)
	case keymap.ActionNavOpen:
		a.dedupCtrl.NavigateFromSelection()
		a.closeDedupView()
	}
	return false
}

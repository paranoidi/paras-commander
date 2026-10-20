package app

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	dialogctrl "github.com/paranoidi/paras-commander/internal/apphandler/dialog"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
	"github.com/paranoidi/paras-commander/internal/zoxide"
)

func (a *App) mergedPanelHistories() []string {
	return panel.MergeNavigationHistories(a.inactivePanel().History, a.activePanel().History)
}

func (a *App) openHistoryDialog(panelID int) {
	if ui.IsAuxiliaryView(a.model.ViewMode) {
		return
	}
	if a.inQuickFilterUI() {
		a.cancelActiveQuickFilter()
	}
	p := a.panelByID(panelID)
	panelPaths := append([]string(nil), p.History...)
	if len(panelPaths) == 0 {
		a.setTransientMessage("No directory history yet", ui.MessageUrgencyInfo)
		return
	}
	curIdx := p.HistoryIndex
	if curIdx < 0 || curIdx >= len(panelPaths) {
		curIdx = 0
	}
	a.model.HistoryDialog = dialog.HistoryDialogState{
		Open:              true,
		PanelID:           panelID,
		Paths:             panelPaths,
		CurrentIndex:      curIdx,
		BothPanels:        false,
		PanelPaths:        panelPaths,
		PanelCurrentIndex: curIdx,
		DisplayLines:      panelPaths,
		PathMissing:       make([]bool, len(panelPaths)),
		IncludeZoxide:     a.config.History.Zoxide,
		ZoxideFrom:        len(panelPaths),
		Query:             "",
		Focus:             0,
		Selected:          0,
		ListScroll:        0,
	}
	a.syncHistoryDialogRanks()
	selected := 0
	for i, idx := range a.model.HistoryDialog.Ranked {
		if idx == curIdx {
			selected = i
			break
		}
	}
	a.model.HistoryDialog.Selected = selected
	dialog.EnsureHistoryListScroll(&a.model.HistoryDialog, a.historyDialogListRows())
	a.startHistoryMissingScan()
	// Always query: the checkbox is shown only when zoxide is installed, and toggling it on
	// should not wait for another subprocess.
	if a.screen != nil {
		screen := a.screen
		go func() {
			paths, installed := zoxide.List()
			_ = screen.PostEvent(tcell.NewEventInterrupt(historyZoxidePayload{Paths: paths, Installed: installed}))
		}()
	}
}

// historyZoxidePayload carries `zoxide query --list` results back to the UI goroutine.
type historyZoxidePayload struct {
	Paths     []string
	Installed bool
}

// applyHistoryZoxide stores zoxide's directories on the open history dialog and shows its
// "Include zoxide" checkbox.
// ponytail: no generation guard; a late result for a since-reopened dialog carries the same data.
func (a *App) applyHistoryZoxide(p historyZoxidePayload) {
	st := &a.model.HistoryDialog
	if !st.Open || !p.Installed {
		return
	}
	if !st.ZoxideAvailable && st.Focus > 0 {
		st.Focus++ // the checkbox is inserted before OK/Cancel; keep the same button focused
	}
	st.ZoxideAvailable = true
	st.Zoxide = p.Paths
	a.rebuildHistoryDialogPaths()
}

// toggleHistoryIncludeZoxide flips the "Include zoxide" checkbox (Alt+I).
func (a *App) toggleHistoryIncludeZoxide() {
	st := &a.model.HistoryDialog
	if !st.ZoxideAvailable {
		return
	}
	st.IncludeZoxide = !st.IncludeZoxide
	a.rebuildHistoryDialogPaths()
}

// rebuildHistoryDialogPaths sets Paths to the target-only or merged history followed, when
// IncludeZoxide, by the zoxide directories not already in it, then re-ranks and rescans for
// missing paths.
func (a *App) rebuildHistoryDialogPaths() {
	st := &a.model.HistoryDialog
	base := append([]string(nil), st.PanelPaths...)
	if st.BothPanels {
		base = a.mergedPanelHistories()
	}
	st.ZoxideFrom = len(base)
	seen := make(map[string]struct{}, len(base))
	for _, p := range base {
		seen[panel.CleanPathString(p)] = struct{}{}
	}
	for _, p := range st.Zoxide {
		if !st.IncludeZoxide {
			break
		}
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			base = append(base, p)
		}
	}
	st.Paths = base
	st.PathMissing = make([]bool, len(st.Paths))
	a.reloadHistoryDialogList()
	a.startHistoryMissingScan()
}

func (a *App) closeHistoryDialog() {
	a.model.HistoryDialog = dialog.HistoryDialogState{}
	a.historyMissingGen++
}

// startHistoryMissingScan bumps the history dialog's missing-scan generation and starts a
// background scan of its current path list, so applyHistoryDialogMissing can later fill in
// PathMissing without blocking dialog open/toggle on stats (see StartPathsMissingScan).
func (a *App) startHistoryMissingScan() {
	a.historyMissingGen++
	st := &a.model.HistoryDialog
	dialogctrl.StartPathsMissingScan(a.screen, "history", a.historyMissingGen, "", "", st.Paths)
}

// applyPathsMissing routes a background missing-path scan's result (dialogctrl.PathsMissingPayload)
// to whichever dialog started it. Split out of handleInterruptPayload to keep that switch's
// cyclomatic complexity within golangci-lint's gocyclo threshold (same reasoning as
// handlePreviewInterruptPayload).
func (a *App) applyPathsMissing(p dialogctrl.PathsMissingPayload) {
	switch p.Target {
	case "picker":
		a.dialogCtrl.ApplyPathPickerMissing(p)
	case "history":
		a.applyHistoryDialogMissing(p)
	case "pin":
		a.pinCtrl.ApplyMissing(p)
	}
}

// applyHistoryDialogMissing applies a background missing-path scan's result to the open
// history dialog's paths. Ignored if the dialog has since closed or a newer scan has been
// started (e.g. by toggleHistoryDialogBothPanels).
func (a *App) applyHistoryDialogMissing(p dialogctrl.PathsMissingPayload) {
	st := &a.model.HistoryDialog
	if !st.Open || p.Gen != a.historyMissingGen {
		return
	}
	for i := range st.PathMissing {
		if i >= len(st.Paths) {
			break
		}
		st.PathMissing[i] = p.Missing[st.Paths[i]]
	}
}

func (a *App) toggleHistoryDialogBothPanels() {
	st := &a.model.HistoryDialog
	if !st.Open {
		return
	}
	st.BothPanels = !st.BothPanels
	a.rebuildHistoryDialogPaths()
}

func (a *App) reloadHistoryDialogList() {
	st := &a.model.HistoryDialog
	if !st.Open {
		return
	}
	var selectedPath string
	if len(st.Ranked) > 0 && st.Selected >= 0 && st.Selected < len(st.Ranked) {
		idx := st.Ranked[st.Selected]
		if idx >= 0 && idx < len(st.Paths) {
			selectedPath = st.Paths[idx]
		}
	}
	st.DisplayLines = st.Paths
	a.syncHistoryDialogRanks()
	if selectedPath != "" {
		want := panel.CleanPathString(selectedPath)
		for i, idx := range st.Ranked {
			if idx >= 0 && idx < len(st.Paths) && panel.CleanPathString(st.Paths[idx]) == want {
				st.Selected = i
				break
			}
		}
	}
	dialog.EnsureHistoryListScroll(st, a.historyDialogListRows())
}

func historyDialogOverlayFooterKeys(keys *keymap.Map, bothPanels bool) []menu.FunctionKey {
	if keys == nil {
		return nil
	}
	hint := "Both Panels"
	if bothPanels {
		hint = "Active panel"
	}
	var out []menu.FunctionKey
	if lbl := keys.MenuBindingLabel(keymap.ActionPanelHistoryBothPanels); lbl != "" {
		out = append(out, menu.FunctionKey{KeyLabel: lbl, Hint: hint})
	}
	return append(out, openInPanelFooterKeys(keys)...)
}

// openHistorySelectionInPanel navigates panelID to the selected history directory; the dialog
// stays open (like the Pin dialog's open-in-panel).
func (a *App) openHistorySelectionInPanel(panelID int) {
	st := &a.model.HistoryDialog
	if len(st.Ranked) == 0 || st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return
	}
	entIdx := st.Ranked[st.Selected]
	if entIdx < 0 || entIdx >= len(st.Paths) {
		return
	}
	path := filepath.Clean(st.Paths[entIdx])
	if err := a.navigatePanelToDirectory(panelID, path, ""); err != nil {
		a.setErrorMessage("History", err)
		return
	}
	a.setTransientMessage("Opened: "+path, ui.MessageUrgencyInfo)
}

func (a *App) syncHistoryDialogRanks() {
	st := &a.model.HistoryDialog
	if !st.Open {
		return
	}
	lines := make([]string, len(st.DisplayLines))
	copy(lines, st.DisplayLines)
	st.Ranked, st.MatchRanges = syncFilteredListRanks(lines, st.Query, len(st.Paths), a.config.Filter.CaseInsensitive)
	clampFilteredListSelection(&st.Selected, len(st.Ranked))
	dialog.EnsureHistoryListScroll(st, a.historyDialogListRows())
}

func (a *App) historyDialogListRows() int {
	termW, termH := a.screen.Size()
	layout := a.layoutForTerminalSize(termW, termH)
	return max(dialog.HistoryDialogListRows(layout.Height, a.model.HistoryDialog.ZoxideAvailable), 1)
}

func (a *App) activateHistorySelection() {
	st := &a.model.HistoryDialog
	if len(st.Ranked) == 0 || st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return
	}
	entIdx := st.Ranked[st.Selected]
	if entIdx < 0 || entIdx >= len(st.Paths) {
		return
	}
	path := filepath.Clean(st.Paths[entIdx])
	if err := a.navigatePanelToDirectory(st.PanelID, path, ""); err != nil {
		a.setErrorMessage("History", err)
		return
	}
	a.model.ActivePanel = st.PanelID
	a.model.ActiveSubFocus = ui.SubFocusFileList
	a.closeHistoryDialog()
	a.setTransientMessage(path, ui.MessageUrgencyInfo)
}

// pinHistorySelection toggles the selected history directory in the pin list.
func (a *App) pinHistorySelection() {
	st := &a.model.HistoryDialog
	if st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return
	}
	entIdx := st.Ranked[st.Selected]
	if entIdx < 0 || entIdx >= len(st.Paths) {
		return
	}
	path := filepath.Clean(st.Paths[entIdx])
	a.pinCtrl.TogglePath(filepath.Base(path), path, true)
}

func (a *App) handleHistoryDialogKey(event *tcell.EventKey) {
	if id, ok := a.keys.Global.Lookup(event); ok && id == keymap.ActionPanelPinToggle {
		a.pinHistorySelection()
		return
	}
	if a.keys.HistoryDialog != nil {
		if id, ok := a.keys.HistoryDialog.Lookup(event); ok {
			switch id {
			case keymap.ActionPanelHistoryBothPanels:
				a.toggleHistoryDialogBothPanels()
				return
			case keymap.ActionOpenInPrimary:
				a.openHistorySelectionInPanel(ui.PrimaryPanel)
				return
			case keymap.ActionOpenInSecondary:
				a.openHistorySelectionInPanel(ui.SecondaryPanel)
				return
			}
		}
	}
	if dialog.TryDialogMnemonics(event, []dialog.ExtraMnemonic{{Rune: dialog.HistoryZoxideMnemonic, Fn: a.toggleHistoryIncludeZoxide}}) {
		return
	}

	st := &a.model.HistoryDialog
	if st.Focus == 0 {
		onChange := func() {
			a.syncHistoryDialogRanks()
			st.Selected = 0
			dialog.EnsureHistoryListScroll(st, a.historyDialogListRows())
		}
		edit := scrollquery.NewEdit(&st.Query, &st.QueryCursor, &st.QueryScroll, a.historyDialogQueryWidth(), onChange)
		if a.handleScrollingQueryKey(event, true, edit) {
			return
		}
	}

	switch event.Key() {
	case tcell.KeyEnter:
		switch st.Focus {
		case st.HistoryCancelFocus():
			a.closeHistoryDialog()
		case st.HistoryZoxideFocus():
			a.toggleHistoryIncludeZoxide()
		default:
			a.activateHistorySelection()
		}
	case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown:
		// Up/Down on the list move the selection, not focus.
		listMotion := st.Focus == 0 && (event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown)
		if nf, ok := st.HistoryForm().MoveFocus(st.Focus, event.Key()); ok && !listMotion {
			st.Focus = nf
			if st.Focus == 0 && event.Key() == tcell.KeyUp {
				dialog.EnsureHistoryListScroll(st, a.historyDialogListRows())
			}
			break
		}
		if handleFilteredListSelectionKey(event, st.Focus, &st.Selected, len(st.Ranked), a.historyDialogListRows, func() {
			dialog.EnsureHistoryListScroll(st, a.historyDialogListRows())
		}) {
			break
		}
	case tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn:
		if handleFilteredListSelectionKey(event, st.Focus, &st.Selected, len(st.Ranked), a.historyDialogListRows, func() {
			dialog.EnsureHistoryListScroll(st, a.historyDialogListRows())
		}) {
			break
		}
	case tcell.KeyRune:
		if event.Modifiers() != tcell.ModNone {
			break
		}
		if st.Focus == 0 {
			break
		}
		switch dialog.DialogButtonRune(event.Rune()) {
		case dialog.ButtonRuneOK:
			a.activateHistorySelection()
		case dialog.ButtonRuneCancel:
			a.closeHistoryDialog()
		case dialog.ButtonRuneToggle:
			switch st.Focus {
			case st.HistoryZoxideFocus():
				a.toggleHistoryIncludeZoxide()
			case st.HistoryOKFocus():
				a.activateHistorySelection()
			case st.HistoryCancelFocus():
				a.closeHistoryDialog()
			}
		}
	}
}

package app

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/app/helpkeys"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// helpContext describes what the F1 help dialog shows for a given ui.ViewMode.
// Entry visibility itself comes from ActionSpec.Views (internal/keymap/specs.go).
type helpContext struct {
	title    string                             // dialog title
	activate func(a *App, actionID string) bool // nil = dispatchActionLikeKeyboardShortcut
}

// helpContextFor resolves the help content for the current view. This is the single place
// that knows what "contextual help" means for each ui.ViewMode — new views plug in here
// instead of growing another branch in openHelpDialog/activateHelpAction.
func (a *App) helpContextFor(vm ui.ViewMode) helpContext {
	switch vm {
	case ui.ViewDedup:
		return helpContext{title: "Help — Dedup", activate: (*App).activateDedupHelpAction}
	case ui.ViewJobs:
		return helpContext{title: "Help — Jobs", activate: (*App).activateJobsHelpAction}
	case ui.ViewCommands:
		return helpContext{title: "Help — Commands", activate: (*App).activateCommandsHelpAction}
	case ui.ViewMessages:
		return helpContext{title: "Help — Messages", activate: (*App).activateMessagesHelpAction}
	case ui.ViewCompare:
		return helpContext{title: "Help — Compare", activate: (*App).activateCompareHelpAction}
	case ui.ViewFilePreview:
		return helpContext{title: "Help — Preview"}
	default:
		return helpContext{title: "Help"}
	}
}

func (a *App) openHelpDialog() {
	hc := a.helpContextFor(a.model.ViewMode)
	entries := a.buildHelpEntriesForView(a.model.ViewMode)
	if entries == nil {
		entries = []dialog.HelpEntry{}
	}
	a.model.HelpView = dialog.HelpViewState{
		Open:       true,
		Title:      hc.title,
		Query:      "",
		Entries:    entries,
		Selected:   0,
		ListScroll: 0,
		Focus:      0,
	}
	a.syncHelpRanks()
}

func (a *App) closeHelpDialog() {
	a.model.HelpView = dialog.HelpViewState{}
}

func (a *App) syncHelpRanks() {
	st := &a.model.HelpView
	if !st.Open {
		return
	}
	rankTexts := make([]string, len(st.Entries))
	for i, e := range st.Entries {
		rankTexts[i] = helpkeys.CanonicalRankText(e)
	}
	q := search.Parse(st.Query)
	opts := search.Options{CaseInsensitive: a.config.Filter.CaseInsensitive}
	ranked := q.Rank(rankTexts, opts)
	st.Ranked = make([]int, len(ranked))
	st.MatchRanges = make([][]search.Range, len(st.Entries))
	for i := range st.MatchRanges {
		st.MatchRanges[i] = nil
	}
	w, h := a.screen.Size()
	metrics, metricsOK := dialog.ComputeHelpDialogListMetrics(dialog.Layout{Width: w, Height: h})
	for i, r := range ranked {
		st.Ranked[i] = r.Index
		idx := r.Index
		if idx < 0 || idx >= len(st.Entries) {
			continue
		}
		if !metricsOK {
			continue
		}
		painted := dialog.FormatHelpRow(st.Entries[idx], 0, metrics.KeyPad, metrics.KeyPad+metrics.SecPad, metrics.InputWidth)
		st.MatchRanges[idx] = q.Match(painted, opts).Ranges
	}
	if st.Selected >= len(st.Ranked) {
		if len(st.Ranked) == 0 {
			st.Selected = 0
		} else {
			st.Selected = len(st.Ranked) - 1
		}
	}
	if st.Selected < 0 {
		st.Selected = 0
	}
	ensureHelpListScroll(st, a.helpListRows())
}

func ensureHelpListScroll(st *dialog.HelpViewState, listRows int) {
	n := len(st.Ranked)
	if n == 0 || listRows <= 0 {
		st.ListScroll = 0
		return
	}
	if st.Selected < 0 {
		st.Selected = 0
	}
	if st.Selected >= n {
		st.Selected = n - 1
	}
	if st.ListScroll > st.Selected {
		st.ListScroll = st.Selected
	}
	if st.Selected >= st.ListScroll+listRows {
		st.ListScroll = st.Selected - listRows + 1
	}
}

// applyHelpListNav applies PgUp/PgDn/Ctrl+Home/Ctrl+End to the ranked help
// list when it has focus, updating scroll. Returns false (no-op) when the
// list isn't focused, is empty, or key isn't a recognized list-navigation key.
func (a *App) applyHelpListNav(key tcell.Key, mods tcell.ModMask) bool {
	st := &a.model.HelpView
	if st.Focus != 0 || len(st.Ranked) == 0 {
		return false
	}
	sel, ok := dialog.ListNavKeySelection(key, mods, st.Selected, len(st.Ranked), max(1, a.helpListRows()-1))
	if !ok {
		return false
	}
	st.Selected = sel
	ensureHelpListScroll(st, a.helpListRows())
	return true
}

func (a *App) helpListRows() int {
	_, termH := a.screen.Size()
	// Centered dialog: 7 rows margin top/bottom → max height = termH - 14.
	// Total dialog height = 9 + listH (title, sep, filter label, blank, input, sep, header, sep after list, button row).
	maxHeight := termH - 14
	if maxHeight < 12 {
		maxHeight = 12
	}
	listH := maxHeight - 9
	if listH < 4 {
		listH = 4
	}
	if listH > 27 {
		listH = 27 // cap at 36 total height - 9 chrome
	}
	return listH
}

// buildHelpEntries constructs help entries for the browser view.
func (a *App) buildHelpEntries() []dialog.HelpEntry {
	return a.buildHelpEntriesForView(ui.ViewBrowser)
}

// buildDedupHelpEntries constructs contextual help for the find-duplicates view.
func (a *App) buildDedupHelpEntries() []dialog.HelpEntry {
	return a.buildHelpEntriesForView(ui.ViewDedup)
}

func (a *App) buildHelpEntriesForView(vm ui.ViewMode) []dialog.HelpEntry {
	var entries []dialog.HelpEntry

	specs := keymap.DefaultActionSpecs()
	for _, spec := range specs {
		if !helpkeys.ActionRunnableInView(vm, spec.ID) {
			continue
		}
		keys := a.effectiveKeyStringsForView(spec.ID, spec.DefaultKeys, vm)
		if len(keys) == 0 {
			continue // unbound
		}
		displayKeys := helpkeys.JoinDisplay(keys, spec.PreferredKey)
		entries = append(entries, dialog.HelpEntry{
			ActionID:   spec.ID,
			Title:      spec.Title,
			Keys:       displayKeys,
			Section:    helpSectionForView(vm, spec),
			FuzzyExtra: strings.TrimSpace(spec.ID + helpkeys.ConcatKeywords(spec.Keywords)),
		})
	}
	return entries
}

// helpSectionForView returns the help grouping for an action in vm. Preview-only and
// preview-shared bindings use the Preview section while the fullscreen file view is active.
func helpSectionForView(vm ui.ViewMode, spec keymap.ActionSpec) string {
	if vm != ui.ViewFilePreview {
		return spec.Section
	}
	switch spec.ID {
	case keymap.ActionFileEdit,
		keymap.ActionFileQuickViewPreviewPageUp,
		keymap.ActionFileQuickViewPreviewPageDown:
		return "Preview"
	}
	if spec.Views == keymap.HelpFilePreview {
		return "Preview"
	}
	return spec.Section
}

// effectiveKeyStringsForView returns bound key strings for an action in the given view context.
func (a *App) effectiveKeyStringsForView(actionID string, defaults []string, vm ui.ViewMode) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(xs []string) {
		for _, k := range xs {
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			out = append(out, k)
		}
	}
	if a.keys != nil {
		add(a.keys.BindingsForAction(actionID))
	}
	if a.keysJobs != nil {
		add(a.keysJobs.BindingsForAction(actionID))
	}
	if a.keysCommands != nil {
		add(a.keysCommands.BindingsForAction(actionID))
	}
	if a.keysMessages != nil {
		add(a.keysMessages.BindingsForAction(actionID))
	}
	if a.keysBookmarkDialog != nil {
		add(a.keysBookmarkDialog.BindingsForAction(actionID))
	}
	if a.keysFindDialog != nil {
		add(a.keysFindDialog.BindingsForAction(actionID))
	}
	if a.keysHistoryDialog != nil {
		add(a.keysHistoryDialog.BindingsForAction(actionID))
	}
	if a.keysFlattenDialog != nil {
		add(a.keysFlattenDialog.BindingsForAction(actionID))
	}
	if vm == ui.ViewDedup && a.keysDedup != nil {
		add(a.keysDedup.BindingsForAction(actionID))
	}
	if vm == ui.ViewCompare && a.keysCompare != nil {
		add(a.keysCompare.BindingsForAction(actionID))
	}
	if vm == ui.ViewFilePreview && a.keysFilePreview != nil {
		add(a.keysFilePreview.BindingsForAction(actionID))
	}
	if len(out) > 0 {
		return out
	}
	add(defaults)
	if len(out) > 0 {
		return out
	}
	if od := keymap.DefaultJobsOverlayKeys()[actionID]; len(od) > 0 {
		add(od)
	}
	if od := keymap.DefaultBookmarkDialogOverlayKeys()[actionID]; len(od) > 0 {
		add(od)
	}
	if od := keymap.DefaultFindDialogOverlayKeys()[actionID]; len(od) > 0 {
		add(od)
	}
	if od := keymap.DefaultHistoryDialogOverlayKeys()[actionID]; len(od) > 0 {
		add(od)
	}
	if od := keymap.DefaultFlattenDialogOverlayKeys()[actionID]; len(od) > 0 {
		add(od)
	}
	if od := keymap.DefaultCommandsOverlayKeys()[actionID]; len(od) > 0 {
		add(od)
	}
	if od := keymap.DefaultMessagesOverlayKeys()[actionID]; len(od) > 0 {
		add(od)
	}
	if vm == ui.ViewDedup {
		if od := keymap.DefaultDedupOverlayKeys()[actionID]; len(od) > 0 {
			add(od)
		}
	}
	if vm == ui.ViewCompare {
		if od := keymap.DefaultCompareOverlayKeys()[actionID]; len(od) > 0 {
			add(od)
		}
	}
	if vm == ui.ViewFilePreview {
		if od := keymap.DefaultFilePreviewOverlayKeys()[actionID]; len(od) > 0 {
			add(od)
		}
	}
	return out
}

// activateHelpAction runs the action chosen from the help dialog.
func (a *App) activateHelpAction(actionID string) bool {
	hc := a.helpContextFor(a.model.ViewMode)
	if hc.activate != nil {
		return hc.activate(a, actionID)
	}
	return a.dispatchActionLikeKeyboardShortcut(actionID)
}

// activateJobsHelpAction runs a help-dialog action while the jobs view is active. Nav
// actions bypass dispatch() (which assumes file-browser panel navigation) and move the
// jobs-list cursor directly, mirroring what the raw arrow-key handler does.
func (a *App) activateJobsHelpAction(actionID string) bool {
	switch actionID {
	case keymap.ActionAppQuit:
		return a.handleQuit()
	case keymap.ActionAppQuitImmediate:
		return a.handleQuitImmediate()
	case keymap.ActionAppOpenMenu:
		a.openMenu()
		return false
	}
	if a.tryDispatchJobs(actionID) {
		return false
	}
	if a.tryDispatchAuxiliaryScreens(actionID) {
		return false
	}
	switch actionID {
	case keymap.ActionNavUp:
		a.jobsCtrl.MoveSelection(-1)
	case keymap.ActionNavDown:
		a.jobsCtrl.MoveSelection(1)
	case keymap.ActionNavPageUp:
		a.jobsCtrl.MoveSelection(-5)
	case keymap.ActionNavPageDown:
		a.jobsCtrl.MoveSelection(5)
	case keymap.ActionNavTop:
		a.jobsCtrl.SelectEdge(false)
	case keymap.ActionNavBottom:
		a.jobsCtrl.SelectEdge(true)
	}
	return false
}

// activateCommandsHelpAction runs a help-dialog action while the Commands view is active.
func (a *App) activateCommandsHelpAction(actionID string) bool {
	switch actionID {
	case keymap.ActionAppQuit:
		return a.handleQuit()
	case keymap.ActionAppQuitImmediate:
		return a.handleQuitImmediate()
	case keymap.ActionAppOpenMenu:
		a.openMenu()
		return false
	}
	if a.tryDispatchCommands(actionID) {
		return false
	}
	if a.tryDispatchAuxiliaryScreens(actionID) {
		return false
	}
	if actionID == keymap.ActionPanelExternalBrowser {
		a.dispatch(actionID)
		return false
	}
	switch actionID {
	case keymap.ActionNavUp:
		a.moveCommandsSelection(-1)
	case keymap.ActionNavDown:
		a.moveCommandsSelection(1)
	case keymap.ActionNavPageUp:
		a.moveCommandsSelection(-5)
	case keymap.ActionNavPageDown:
		a.moveCommandsSelection(5)
	case keymap.ActionNavTop:
		a.selectCommandsEdge(false)
	case keymap.ActionNavBottom:
		a.selectCommandsEdge(true)
	}
	return false
}

// activateMessagesHelpAction runs a help-dialog action while the Messages view is active.
func (a *App) activateMessagesHelpAction(actionID string) bool {
	switch actionID {
	case keymap.ActionAppQuit:
		return a.handleQuit()
	case keymap.ActionAppQuitImmediate:
		return a.handleQuitImmediate()
	case keymap.ActionAppOpenMenu:
		a.openMenu()
		return false
	}
	if a.tryDispatchMessages(actionID) {
		return false
	}
	if a.tryDispatchAuxiliaryScreens(actionID) {
		return false
	}
	switch actionID {
	case keymap.ActionNavUp:
		a.moveMessagesSelection(-1)
	case keymap.ActionNavDown:
		a.moveMessagesSelection(1)
	case keymap.ActionNavPageUp:
		a.moveMessagesSelection(-5)
	case keymap.ActionNavPageDown:
		a.moveMessagesSelection(5)
	case keymap.ActionNavTop:
		a.selectMessagesEdge(false)
	case keymap.ActionNavBottom:
		a.selectMessagesEdge(true)
	}
	return false
}

// activateCompareHelpAction runs a help-dialog action while the Compare view is active.
func (a *App) activateCompareHelpAction(actionID string) bool {
	switch actionID {
	case keymap.ActionAppQuit:
		return a.handleQuit()
	case keymap.ActionAppQuitImmediate:
		return a.handleQuitImmediate()
	case keymap.ActionAppOpenMenu:
		a.openMenu()
		return false
	}
	if a.tryDispatchCompare(actionID) {
		return false
	}
	if a.tryDispatchAuxiliaryScreens(actionID) {
		return false
	}
	if actionID == keymap.ActionPanelExternalBrowser {
		a.dispatch(actionID)
		return false
	}

	visible := a.compareVisibleRows()
	switch actionID {
	case keymap.ActionPanelSelectToggle:
		if conflicts := a.compareCtrl.ToggleColumnSelection(); conflicts {
			a.setTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
		}
		a.moveCompareSelection(1)
	case keymap.ActionNavUp:
		a.moveCompareSelection(-1)
	case keymap.ActionNavDown:
		a.moveCompareSelection(1)
	case keymap.ActionNavPageUp:
		a.moveCompareSelection(-visible)
	case keymap.ActionNavPageDown:
		a.moveCompareSelection(visible)
	case keymap.ActionNavTop:
		a.selectCompareEdge(false)
	case keymap.ActionNavBottom:
		a.selectCompareEdge(true)
	case keymap.ActionNavOpen:
		a.compareCtrl.DiscardReturn()
		a.compareCtrl.NavigateFromSelection(visible)
		a.closeCompareView()
	}
	return false
}

func (a *App) activateDedupHelpAction(actionID string) bool {
	switch actionID {
	case keymap.ActionAppQuit:
		return a.handleQuit()
	case keymap.ActionAppQuitImmediate:
		return a.handleQuitImmediate()
	case keymap.ActionAppOpenMenu:
		a.openMenu()
		return false
	}

	visible := a.dedupVisibleRows()

	if a.tryDispatchDedup(actionID) {
		return false
	}
	if a.tryDispatchAuxiliaryScreens(actionID) {
		return false
	}

	switch actionID {
	case keymap.ActionPanelSelectToggle:
		a.dedupCtrl.SelectToggleAndAdvance()
		a.dedupCtrl.EnsureSelectionVisible(visible)
	case keymap.ActionPanelInvertSelection:
		if a.model.DedupView.FocusCopies {
			a.dedupCtrl.ToggleCopiesPaneSelectAll()
		}
	case keymap.ActionPanelSwitch:
		a.dedupCtrl.SwitchPane()
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
	default:
		return false
	}
	return false
}

func (a *App) handleHelpDialogKey(event *tcell.EventKey) bool {
	st := &a.model.HelpView

	if st.Focus == 0 {
		onChange := func() {
			a.syncHelpRanks()
			st.Selected = 0
			ensureHelpListScroll(st, a.helpListRows())
		}
		if a.handleScrollingQueryKey(event, true, helpViewScrollingQuery(st, a.helpDialogQueryWidth(), onChange)) {
			return false
		}
	}

	// Alt+O and Alt+C close the dialog.
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 'o', 'O', 'c', 'C':
			a.closeHelpDialog()
			return false
		}
	}

	switch event.Key() {
	case tcell.KeyEsc:
		a.closeHelpDialog()
		return false
	case tcell.KeyEnter:
		if st.Focus == 1 {
			a.closeHelpDialog()
			return false
		}
		if len(st.Ranked) == 0 {
			a.closeHelpDialog()
			return false
		}
		entIdx := st.Ranked[st.Selected]
		if entIdx < 0 || entIdx >= len(st.Entries) {
			a.closeHelpDialog()
			return false
		}
		actionID := st.Entries[entIdx].ActionID
		if !helpkeys.ActionRunnableInView(a.model.ViewMode, actionID) {
			return false
		}
		a.closeHelpDialog()
		return a.activateHelpAction(actionID)
	case tcell.KeyTab:
		st.Focus = (st.Focus + 1) % 2
	case tcell.KeyBacktab:
		st.Focus = (st.Focus + 1) % 2 // same as Tab for simple 2-focus dialog
	case tcell.KeyUp:
		if st.Focus == 0 {
			if len(st.Ranked) > 0 {
				st.Selected = dialog.ListClampedSelectionDelta(st.Selected, len(st.Ranked), -1)
				ensureHelpListScroll(st, a.helpListRows())
			}
		} else {
			st.Focus = 0 // back to list
			if len(st.Ranked) > 0 {
				st.Selected = len(st.Ranked) - 1
				ensureHelpListScroll(st, a.helpListRows())
			}
		}
	case tcell.KeyDown:
		if st.Focus == 0 {
			if len(st.Ranked) > 0 {
				next := dialog.ListClampedSelectionDelta(st.Selected, len(st.Ranked), 1)
				if next != st.Selected {
					st.Selected = next
					ensureHelpListScroll(st, a.helpListRows())
				} else if st.Selected == len(st.Ranked)-1 {
					st.Focus = 1 // move to Close button
				}
			}
		} // on Close button, Down does nothing
	case tcell.KeyPgUp, tcell.KeyPgDn, tcell.KeyHome, tcell.KeyEnd:
		a.applyHelpListNav(event.Key(), event.Modifiers())
	case tcell.KeyLeft:
		// Only one button (Close), no-op between buttons.
	case tcell.KeyRight:
		// Only one button, no-op.
	}
	return false
}

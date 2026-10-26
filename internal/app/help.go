package app

import (
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/app/helpkeys"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// helpContext describes what the F1 help dialog shows for a given ui.ViewMode.
// Entry visibility itself comes from ActionSpec.Views (internal/keymap/specs.go).
type helpContext struct {
	title     string                             // dialog title
	menuTitle string                             // prefix for `:` leader-menu transient messages (browser has none)
	activate  func(a *App, actionID string) bool // nil = dispatchActionLikeKeyboardShortcut
}

// helpContextFor resolves the help content for the current view. This is the single place
// that knows what "contextual help" means for each ui.ViewMode — new views plug in here
// instead of growing another branch in openHelpDialog/activateHelpAction. menuTitle doubles as
// the `:` per-view leader menu's transient-message prefix (openViewLeaderMenu); the menu itself
// has no visible title bar, so view context there comes from the group headers it renders.
func (a *App) helpContextFor(vm ui.ViewMode) helpContext {
	switch vm {
	case ui.ViewDedup:
		return helpContext{title: "Help — Dedup", menuTitle: "Dedup — Actions", activate: (*App).activateDedupHelpAction}
	case ui.ViewJobs:
		return helpContext{title: "Help — Jobs", menuTitle: "Jobs — Actions", activate: (*App).activateJobsHelpAction}
	case ui.ViewCommands:
		return helpContext{title: "Help — Commands", menuTitle: "Commands — Actions", activate: (*App).activateCommandsHelpAction}
	case ui.ViewMessages:
		return helpContext{title: "Help — Messages", menuTitle: "Messages — Actions", activate: (*App).activateMessagesHelpAction}
	case ui.ViewCompare:
		return helpContext{title: "Help — Compare", menuTitle: "Compare — Actions", activate: (*App).activateCompareHelpAction}
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
	}
	a.syncHelpRanks()
}

// textEditHelpPrefix selects the dialog text-input editing actions ([dialog.input]).
const textEditHelpPrefix = "ui.input."

// notTextEditHelp holds [dialog.input] actions that are not text editing and stay off the page.
var notTextEditHelp = map[string]bool{
	keymap.ActionDialogInputRestoreDefault: true,
	keymap.ActionDialogInputPathPickerAll:  true,
	keymap.ActionDialogOK:                  true,
	keymap.ActionDialogCancel:              true,
}

// buildTextEditHelpEntries lists the dialog text-input editing shortcuts.
func (a *App) buildTextEditHelpEntries() []dialog.HelpEntry {
	entries := []dialog.HelpEntry{}
	defaults := keymap.DefaultDialogInputOverlayKeys()
	for _, spec := range keymap.DefaultActionSpecs() {
		if !strings.HasPrefix(spec.ID, textEditHelpPrefix) || notTextEditHelp[spec.ID] {
			continue
		}
		var keys []string
		if a.keys.DialogInput != nil {
			keys = a.keys.DialogInput.BindingsForAction(spec.ID)
		}
		if len(keys) == 0 {
			keys = defaults[spec.ID]
		}
		if len(keys) == 0 {
			continue
		}
		entries = append(entries, dialog.HelpEntry{
			ActionID:   spec.ID,
			Title:      spec.Title,
			Keys:       helpkeys.JoinDisplay(keys, spec.PreferredKey),
			Section:    "Text editing",
			FuzzyExtra: strings.TrimSpace(spec.ID + helpkeys.ConcatKeywords(spec.Keywords)),
		})
	}
	sortHelpEntriesBySection(entries)
	return entries
}

// toggleHelpTextEdit switches the help dialog between the contextual page and the
// text-editing keys page, resetting query, selection and scroll.
func (a *App) toggleHelpTextEdit() {
	if a.model.HelpView.TextEdit {
		a.openHelpDialog()
		return
	}
	a.model.HelpView = dialog.HelpViewState{
		Open:     true,
		Title:    "Help — Text editing",
		Entries:  a.buildTextEditHelpEntries(),
		TextEdit: true,
	}
	a.syncHelpRanks()
}

// helpDialogOverlayFooterKeys returns the footer item that switches help pages.
func helpDialogOverlayFooterKeys(keys *keymap.Map, textEdit bool) []menu.FunctionKey {
	if keys == nil {
		return nil
	}
	hint := "Text edit keys"
	if textEdit {
		hint = "All keys"
	}
	if lbl := keys.MenuBindingLabel(keymap.ActionHelpTextEditKeys); lbl != "" {
		return []menu.FunctionKey{{KeyLabel: lbl, Hint: hint}}
	}
	return nil
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
		painted, _, _ := dialog.FormatHelpRow(st.Entries[idx], metrics.KeyColWidth, metrics.InputWidth)
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
	st.EnsureListScroll(a.helpListRows())
}

// helpListRows returns the visible list height from dialog.ComputeHelpDialogListMetrics (the
// fitted help spec), so scroll bookkeeping stays in lockstep with what actually gets drawn.
func (a *App) helpListRows() int {
	w, h := a.screen.Size()
	if m, ok := dialog.ComputeHelpDialogListMetrics(dialog.Layout{Width: w, Height: h}); ok {
		return m.ListH
	}
	return 4
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
	sortHelpEntriesBySection(entries)
	return entries
}

// sortHelpEntriesBySection stable-sorts entries by keymap.HelpSectionOrder priority (ties
// broken by existing order), making Entries section-contiguous. This runs unconditionally at
// build time (not just in browse mode) — grouping vs. falling back to the flat relevance list
// is purely a rendering concern keyed off the filter query, not a rebuild concern. A Section
// not present in HelpSectionOrder sorts alphabetically after all known sections.
func sortHelpEntriesBySection(entries []dialog.HelpEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		pi, ki := keymap.HelpSectionPriority[entries[i].Section]
		pj, kj := keymap.HelpSectionPriority[entries[j].Section]
		if ki != kj {
			return ki // known sections sort before unknown ones
		}
		if ki {
			return pi < pj
		}
		return entries[i].Section < entries[j].Section
	})
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
	if a.keys.Global != nil {
		add(a.keys.Global.BindingsForAction(actionID))
	}
	if a.keys.Jobs != nil {
		add(a.keys.Jobs.BindingsForAction(actionID))
	}
	if a.keys.Commands != nil {
		add(a.keys.Commands.BindingsForAction(actionID))
	}
	if a.keys.Messages != nil {
		add(a.keys.Messages.BindingsForAction(actionID))
	}
	if a.keys.BookmarkDialog != nil {
		add(a.keys.BookmarkDialog.BindingsForAction(actionID))
	}
	if a.keys.FindDialog != nil {
		add(a.keys.FindDialog.BindingsForAction(actionID))
	}
	if a.keys.HistoryDialog != nil {
		add(a.keys.HistoryDialog.BindingsForAction(actionID))
	}
	if a.keys.HelpDialog != nil {
		add(a.keys.HelpDialog.BindingsForAction(actionID))
	}
	if a.keys.UserMenu != nil {
		add(a.keys.UserMenu.BindingsForAction(actionID))
	}
	if a.keys.FlattenDialog != nil {
		add(a.keys.FlattenDialog.BindingsForAction(actionID))
	}
	if vm == ui.ViewDedup && a.keys.Dedup != nil {
		add(a.keys.Dedup.BindingsForAction(actionID))
	}
	if vm == ui.ViewCompare && a.keys.Compare != nil {
		add(a.keys.Compare.BindingsForAction(actionID))
	}
	if vm == ui.ViewFilePreview && a.keys.FilePreview != nil {
		add(a.keys.FilePreview.BindingsForAction(actionID))
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
	if od := keymap.DefaultHelpDialogOverlayKeys()[actionID]; len(od) > 0 {
		add(od)
	}
	if od := keymap.DefaultUserMenuOverlayKeys()[actionID]; len(od) > 0 {
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
	if a.jobsCtrl.TryDispatch(actionID) {
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
	if a.commandsCtrl.TryDispatch(actionID) {
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
		a.commandsCtrl.MoveSelection(-1)
	case keymap.ActionNavDown:
		a.commandsCtrl.MoveSelection(1)
	case keymap.ActionNavPageUp:
		a.commandsCtrl.MoveSelection(-5)
	case keymap.ActionNavPageDown:
		a.commandsCtrl.MoveSelection(5)
	case keymap.ActionNavTop:
		a.commandsCtrl.SelectEdge(false)
	case keymap.ActionNavBottom:
		a.commandsCtrl.SelectEdge(true)
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
	case keymap.ActionAppShowHelp: // `:` menu "?"
		if !a.model.HelpView.Open {
			a.openHelpDialog()
		}
		return false
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

	if a.keys.HelpDialog != nil {
		if id, ok := a.keys.HelpDialog.Lookup(event); ok && id == keymap.ActionHelpTextEditKeys {
			a.toggleHelpTextEdit()
			return false
		}
	}

	if st.Focus != dialog.HelpClose {
		onChange := func() {
			st.Selected = 0
			a.syncHelpRanks() // re-ranks and calls st.EnsureListScroll itself
		}
		edit := scrollquery.NewEdit(&st.Query, &st.QueryCursor, &st.QueryScroll, a.helpDialogQueryWidth(), onChange)
		if a.handleScrollingQueryKey(event, true, edit) {
			return false
		}
	}

	if a.handleHelpListKey(event) {
		return false
	}
	if ev, ok := dlg.HandleKey(a.helpView(), &st.Focus, event); ok {
		return a.applyHelpEvent(ev)
	}
	return false
}

// helpView is the help dialog's spec for the current screen.
func (a *App) helpView() dlg.Spec {
	w, h := a.screen.Size()
	spec, _, _ := dialog.HelpDialogView(a.model.HelpView, a.layoutForTerminalSize(w, h), a.styles)
	return spec
}

// applyHelpEvent acts on a widget event of the help dialog (key or click); reports whether the
// app should quit.
func (a *App) applyHelpEvent(ev dlg.Event) bool {
	switch {
	case ev.Kind == dlg.EventCaret:
		a.model.HelpView.QueryCursor = ev.Index
	case ev.Kind == dlg.EventListMove:
		a.model.HelpView.ApplyListMove(ev)
	case ev.ID == dialog.HelpClose:
		a.closeHelpDialog()
	case ev.ID == dialog.HelpFilter: // Enter in the filter activates the selected entry
		return a.activateHelpSelection()
	}
	return false
}

// handleHelpListKey handles the list keys the engine does not: Down from the last entry focuses
// Close, Up from Close returns to the filter on the last entry, and PgUp/PgDn page help's way
// (PageUpSelection / PageDownSelection: first to the edge of the view, then a page). Reports
// whether it took the key.
func (a *App) handleHelpListKey(event *tcell.EventKey) bool {
	st := &a.model.HelpView
	n := len(st.Ranked)
	switch event.Key() {
	case tcell.KeyUp:
		if st.Focus != dialog.HelpClose {
			return false
		}
		st.Focus = dialog.HelpFilter
		if n > 0 {
			st.Selected = n - 1
		}
	case tcell.KeyDown:
		if st.Focus == dialog.HelpClose || n == 0 || st.Selected < n-1 {
			return false
		}
		st.Focus = dialog.HelpClose
		return true
	case tcell.KeyPgUp, tcell.KeyPgDn:
		if st.Focus == dialog.HelpClose || n == 0 {
			return false
		}
		if event.Key() == tcell.KeyPgUp {
			st.Selected = st.PageUpSelection(a.helpListRows())
		} else {
			st.Selected = st.PageDownSelection(a.helpListRows())
		}
	default:
		return false
	}
	st.EnsureListScroll(a.helpListRows())
	return true
}

// activateHelpSelection runs the selected entry's action and closes the dialog. It closes without
// running anything when there are no results, and does nothing on the text-editing page or for an
// entry that cannot run in the current view. Reports whether the app should quit.
func (a *App) activateHelpSelection() bool {
	st := &a.model.HelpView
	if len(st.Ranked) == 0 {
		a.closeHelpDialog()
		return false
	}
	entIdx := st.Ranked[st.Selected]
	if entIdx < 0 || entIdx >= len(st.Entries) {
		a.closeHelpDialog()
		return false
	}
	if st.TextEdit {
		return false
	}
	actionID := st.Entries[entIdx].ActionID
	if !helpkeys.ActionRunnableInView(a.model.ViewMode, actionID) {
		return false
	}
	a.closeHelpDialog()
	return a.activateHelpAction(actionID)
}

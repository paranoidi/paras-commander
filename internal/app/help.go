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

func (a *App) openHelpDialog() {
	if ui.IsAuxiliaryView(a.model.ViewMode) {
		return
	}
	entries := a.buildHelpEntries()
	if entries == nil {
		entries = []ui.HelpEntry{}
	}
	a.model.HelpView = ui.HelpViewState{
		Open:       true,
		Query:      "",
		Entries:    entries,
		Selected:   0,
		ListScroll: 0,
		Focus:      0,
	}
	a.syncHelpRanks()
}

func (a *App) closeHelpDialog() {
	a.model.HelpView = ui.HelpViewState{}
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
	opts := search.Options{CaseInsensitive: a.config.CaseInsensitiveFilter}
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

func ensureHelpListScroll(st *ui.HelpViewState, listRows int) {
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

// buildHelpEntries constructs help entries from keymap action bindings only.
func (a *App) buildHelpEntries() []ui.HelpEntry {
	var entries []ui.HelpEntry

	specs := keymap.DefaultActionSpecs()
	for _, spec := range specs {
		if spec.ID == keymap.ActionAppShowHelp {
			continue // self-referential, omit
		}
		keys := a.effectiveKeyStrings(spec.ID, spec.DefaultKeys)
		if len(keys) == 0 {
			continue // unbound
		}
		displayKeys := helpkeys.JoinDisplay(keys, spec.PreferredKey)
		entries = append(entries, ui.HelpEntry{
			ActionID:   spec.ID,
			Title:      spec.Title,
			Keys:       displayKeys,
			Section:    spec.Section,
			FuzzyExtra: strings.TrimSpace(spec.ID + helpkeys.ConcatKeywords(spec.Keywords)),
		})
	}
	return entries
}

// effectiveKeyStrings returns the actual bound key strings for an action,
// merging global and jobs-overlay bindings, then falling back to defaults and overlay defaults.
func (a *App) effectiveKeyStrings(actionID string, defaults []string) []string {
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
	return out
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
		if !helpkeys.ActionRunnableInBrowser(actionID) {
			return false
		}
		a.closeHelpDialog()
		return a.dispatchActionLikeKeyboardShortcut(actionID)
	case tcell.KeyTab:
		st.Focus = (st.Focus + 1) % 2
	case tcell.KeyBacktab:
		st.Focus = (st.Focus + 1) % 2 // same as Tab for simple 2-focus dialog
	case tcell.KeyUp:
		if st.Focus == 0 {
			if len(st.Ranked) > 0 {
				st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), -1)
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
				next := ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), 1)
				if next != st.Selected {
					st.Selected = next
					ensureHelpListScroll(st, a.helpListRows())
				} else if st.Selected == len(st.Ranked)-1 {
					st.Focus = 1 // move to Close button
				}
			}
		} // on Close button, Down does nothing
	case tcell.KeyPgUp:
		if st.Focus == 0 && len(st.Ranked) > 0 {
			step := max(1, a.helpListRows()-1)
			st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), -step)
			ensureHelpListScroll(st, a.helpListRows())
		}
	case tcell.KeyPgDn:
		if st.Focus == 0 && len(st.Ranked) > 0 {
			step := max(1, a.helpListRows()-1)
			st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), step)
			ensureHelpListScroll(st, a.helpListRows())
		}
	case tcell.KeyHome:
		if st.Focus == 0 && event.Modifiers()&tcell.ModCtrl != 0 && len(st.Ranked) > 0 {
			st.Selected = 0
			ensureHelpListScroll(st, a.helpListRows())
		}
	case tcell.KeyEnd:
		if st.Focus == 0 && event.Modifiers()&tcell.ModCtrl != 0 && len(st.Ranked) > 0 {
			st.Selected = len(st.Ranked) - 1
			ensureHelpListScroll(st, a.helpListRows())
		}
	case tcell.KeyLeft:
		// Only one button (Close), no-op between buttons.
	case tcell.KeyRight:
		// Only one button, no-op.
	}
	return false
}

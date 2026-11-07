package app

import (
	"slices"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/app/helpkeys"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// helpOtherContextsSection groups the actions a key runs outside the view help was opened from.
const helpOtherContextsSection = "Other contexts"

// filterHelpByKey closes the prompt and narrows help to the actions bound to event. The query
// stays usable on the narrowed list and Esc (helpEsc) returns to the page.
func (a *App) filterHelpByKey(event *tcell.EventKey) {
	label := keymap.DisplayEvent(event)
	entries := a.helpEntriesForKey(event, label)
	title := "Help — Actions on " + label
	if len(entries) == 0 {
		title = "Help — Nothing bound to " + label
	}
	a.model.HelpView = dialog.HelpViewState{
		Open:      true,
		Title:     title,
		Entries:   entries,
		KeyFilter: label,
		TextEdit:  a.model.HelpView.TextEdit,
	}
	a.syncHelpRanks()
}

// helpEsc is Esc in help: it drops a key filter, and closes help otherwise.
func (a *App) helpEsc() {
	switch {
	case a.model.HelpView.KeyFilter == "":
		a.closeHelpDialog()
	case a.model.HelpView.TextEdit:
		a.openHelpTextEditPage()
	default:
		a.openHelpDialog()
	}
}

// helpEntriesForKey lists every action bound to event, labelled label. The action the key runs in
// the current view comes first with its full row. Shortcuts are unique within a table, so that
// lookup is the only one for this view, and an action a view overlay shadows is not reported as
// active. Every other table adds what the key does elsewhere, in helpOtherContextsSection.
func (a *App) helpEntriesForKey(event *tcell.EventKey, label string) []dialog.HelpEntry {
	vm := a.model.ViewMode
	var entries []dialog.HelpEntry
	active := a.actionFromKeyEvent(event)
	if spec, ok := keymap.SpecForAction(active); ok {
		switch {
		case helpkeys.ActionRunnableInView(vm, active):
			page := a.buildHelpEntriesForView(vm)
			if i := slices.IndexFunc(page, func(e dialog.HelpEntry) bool { return e.ActionID == active }); i >= 0 {
				entries = append(entries, page[i])
			}
		case spec.Views == 0:
			entries = append(entries, newHelpEntry(spec, label, spec.Section))
		}
	}
	listed := func(id string) bool {
		return slices.ContainsFunc(entries, func(e dialog.HelpEntry) bool { return e.ActionID == id })
	}
	for _, m := range append(a.keys.Overlays(), a.keys.Global) {
		id, ok := m.Lookup(event)
		if !ok || listed(id) {
			continue
		}
		if spec, ok := keymap.SpecForAction(id); ok {
			entries = append(entries, newHelpEntry(spec, label, helpOtherContextsSection))
		}
	}
	sortHelpEntriesBySection(entries)
	return entries
}

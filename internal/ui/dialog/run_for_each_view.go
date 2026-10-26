package dialog

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Widget IDs of the run-for-each form. The command input is FileFieldID(0); the buttons are
// dlg.OK and dlg.Cancel.
const (
	RunForEachInDirs  = dlg.ID("in-dirs")
	RunForEachPTY     = dlg.ID("pty")
	RunForEachNoPool  = dlg.ID("pool-none")
	runForEachPoolPre = "pool-"
)

// RunForEachPoolID is the radio ID of configured worker pool i (index into RunForEachPools).
func RunForEachPoolID(i int) dlg.ID { return dlg.IndexedID(runForEachPoolPre, i) }

// runForEachPoolIndex returns the RunForEachPools index named by a RunForEachPoolID.
func runForEachPoolIndex(id dlg.ID) (int, bool) {
	return dlg.IDIndex(id, runForEachPoolPre)
}

// runForEachView is the widget spec of the run-for-each command form: the help text and a rule,
// the Command input with its validation error or expanded preview under it, a rule, the
// directory / pseudo-TTY checkboxes and, when worker pools are configured, a rule above the pool
// radios. The checkboxes and radios are one Tab segment; the command input is another.
func runForEachView(st FileDialogState, ctx FileDialogViewContext) dlg.Spec {
	spec := dlg.Spec{
		Title: fileDialogOuterTitle(st), Width: fileDialogWidth(ctx.Layout.Width, st, ctx.IconLead), Unclamped: true,
		Buttons: []dlg.Button{
			dlg.OKButton,
			dlg.CancelButton,
		},
	}
	if strings.TrimSpace(st.Message) != "" {
		var help []dlg.Item
		for _, line := range strings.Split(st.Message, "\n") {
			help = append(help, dlg.Item{Kind: dlg.KindLabel, Label: line})
		}
		spec.Groups = append(spec.Groups, dlg.Group{Columns: [][]dlg.Item{append(help, dlg.Item{Kind: dlg.KindSeparator})}})
	}
	command := []dlg.Item{{Kind: dlg.KindLabel, Label: "Command:"}}
	if len(st.Fields) > 0 {
		f := st.Fields[0]
		command[0].Label = f.Label + ":"
		command = append(command, fileFieldInput(FileFieldID(0), f, f.InputInvalid))
	}
	if hint := runForEachCommandErrorText(st); hint != "" {
		command = append(command, dlg.Item{Kind: dlg.KindLabel, Label: hint, Style: dlg.TextError})
	} else if preview := runForEachPreviewText(st); preview != "" {
		command = append(command, dlg.Item{Kind: dlg.KindLabel, Label: "→ " + preview})
	}
	command = append(command, dlg.Item{Kind: dlg.KindSeparator})
	options := []dlg.Item{
		{ID: RunForEachInDirs, Kind: dlg.KindCheckbox, Label: "Run in each selected directory", Mnemonic: 'R', On: st.RunForEachInDirs},
		{ID: RunForEachPTY, Kind: dlg.KindCheckbox, Label: "Allocate pseudo-TTY (interactive)", Mnemonic: 'T', On: st.RunForEachPTY},
	}
	if runForEachHasPoolSelector(st) {
		selected := strings.TrimSpace(st.RunForEachPool)
		options = append(options,
			dlg.Item{Kind: dlg.KindSeparator},
			dlg.Item{Kind: dlg.KindLabel, Label: "Worker pool (optional):"},
			dlg.Item{ID: RunForEachNoPool, Kind: dlg.KindRadio, Label: "No pool (unlimited)", On: selected == ""},
		)
		for i, name := range st.RunForEachPools {
			options = append(options, dlg.Item{ID: RunForEachPoolID(i), Kind: dlg.KindRadio, Label: name, On: selected == strings.TrimSpace(name)})
		}
	}
	spec.Groups = append(spec.Groups, dlg.Group{Columns: [][]dlg.Item{command}}, dlg.Group{Columns: [][]dlg.Item{options}})
	return spec
}

// applyRunForEachEvent folds an activation of the run-for-each form's options into the state: the
// checkboxes toggle, a pool radio selects its pool.
func (st *FileDialogState) applyRunForEachEvent(ev dlg.Event) {
	switch ev.ID {
	case RunForEachInDirs:
		st.RunForEachInDirs = !st.RunForEachInDirs
	case RunForEachPTY:
		st.RunForEachPTY = !st.RunForEachPTY
	case RunForEachNoPool:
		st.RunForEachPool = ""
	}
	if i, ok := runForEachPoolIndex(ev.ID); ok && i >= 0 && i < len(st.RunForEachPools) {
		st.RunForEachPool = st.RunForEachPools[i]
	}
}

// Widget IDs of the fuzzy picker sub-screens (run-for-each command history; mass rename saved
// patterns, pattern history and overwrite). The list is driven by the filter input and has no
// focus of its own; an empty focus is the filter. The buttons are dlg.OK and dlg.Cancel.
const (
	PickerFilter = dlg.ID("picker-filter")
	PickerList   = dlg.ID("picker-list")
)

// PickerListRows is how many list rows a picker sub-screen shows on a layout of layoutHeight rows
// (4..18).
func PickerListRows(layoutHeight int) int { return min(max(layoutHeight-12, 4), 18) }

// pickerView is the spec of a picker sub-screen: a search input steering the ranked list below it,
// and OK / Cancel with plain letters. line returns the text of the item at an index into the
// state's items; ranked, ranges, selected and offset are the picker state.
func pickerView(st FileDialogState, ctx FileDialogViewContext, count int, line func(i int) string, ranked []int, ranges [][]search.Range, query string, cursor, scroll, selected, offset int) dlg.Spec {
	list := dlg.Item{
		ID: PickerList, Kind: dlg.KindList, Driven: true, Cursor: selected, Offset: offset, Height: PickerListRows(ctx.Layout.Height),
	}
	for _, ent := range ranked {
		var row dlg.ListRow
		if ent >= 0 && ent < count {
			row.Text = line(ent)
			if ent < len(ranges) {
				row.Match = ranges[ent]
			}
		}
		list.Rows = append(list.Rows, row)
	}
	filter := dlg.Item{
		ID: PickerFilter, Kind: dlg.KindInput, Drives: PickerList, LeadingIcon: ctx.Styles.IconSearchIcon(),
		Value: query, Cursor: cursor, Scroll: scroll,
	}
	return dlg.Spec{
		Title: fileDialogOuterTitle(st), Width: fileDialogWidth(ctx.Layout.Width, st, ctx.IconLead), Unclamped: true, PlainButtonKeys: true,
		Groups:  []dlg.Group{{Columns: [][]dlg.Item{{filter, {Kind: dlg.KindSeparator}, list}}}},
		Buttons: dlg.OKCancelButtons(),
	}
}

// runForEachHistoryView is the command-history picker: the recently run command lines, filtered.
func runForEachHistoryView(st FileDialogState, ctx FileDialogViewContext) dlg.Spec {
	pk := st.RunForEachHistoryPicker
	return pickerView(st, ctx, len(pk.Items), func(i int) string { return pk.Items[i] },
		pk.Ranked, pk.MatchRanges, pk.Query, pk.QueryCursor, pk.QueryScroll, pk.Selected, pk.ListScroll)
}

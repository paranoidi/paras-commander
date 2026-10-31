package dialog

import (
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// MetaEntryID is the widget ID of the checkbox row for entry i.
func MetaEntryID(i int) dlg.ID { return dlg.IndexedID("entry:", i) }

const metaDialogWidth = 56

// metaColumnGap is the gap cells after a first-column box in a two-column section; the marker
// width's trailing cell adds one more blank before the second column.
const metaColumnGap = 2

// MetaDialogView is the meta column picker's widget spec: one checkbox per meta entry above OK /
// Cancel, each entry Section in its own group under a separator headed by the section name, or
// the wrapped Notice when there are no entries. A section whose labels fit two side by side is
// laid out in two columns (metaSectionGroup). Plain o / c press the buttons.
func MetaDialogView(s MetaDialogState) dlg.Spec {
	shortcuts := MetaEntryShortcuts(s.Entries)
	var groups []dlg.Group
	var items []dlg.Item
	section := ""
	for i, e := range s.Entries {
		if e.Section != section {
			if len(items) > 0 {
				groups = append(groups, metaSectionGroup(items))
			}
			section = e.Section
			groups = append(groups, dlg.Group{Columns: [][]dlg.Item{{{Kind: dlg.KindSeparator, Label: section}}}})
			items = nil
		}
		items = append(items, dlg.Item{
			ID: MetaEntryID(i), Kind: dlg.KindCheckbox, Label: metaEntryDisplayLabel(e), Mnemonic: shortcuts[i],
			On: i < len(s.Checked) && s.Checked[i],
		})
	}
	if len(s.Entries) == 0 && s.Notice != "" {
		for _, line := range wrapMessageBody(s.Notice, metaDialogWidth-4) {
			items = append(items, dlg.Item{Kind: dlg.KindLabel, Label: line, Style: dlg.TextWarn})
		}
		return metaSpec(append(groups, dlg.Group{Columns: [][]dlg.Item{items}}), s.MarkerWidth)
	}
	return metaSpec(append(groups, metaSectionGroup(items)), s.MarkerWidth)
}

func metaSpec(groups []dlg.Group, markerW int) dlg.Spec {
	return dlg.Spec{
		Title: "Meta", Width: metaDialogWidth, MinWidth: 28, MinHeight: 5, PlainButtonKeys: true,
		Groups: groups, MarkerWidth: markerW,
		Buttons: []dlg.Button{
			dlg.OKButton,
			dlg.CancelButton,
		},
	}
}

// metaSectionGroup lays out one section's checkboxes: in rows of two, filled column by column
// (the first column gets the extra one), when two of the longest label fit side by side within
// the dialog; otherwise one per row. Each first-column box is padded to the longest
// first-column label, so the second column lines up whatever the theme's marker width.
func metaSectionGroup(items []dlg.Item) dlg.Group {
	longest := 0
	for _, it := range items {
		longest = max(longest, runewidth.StringWidth(it.Label))
	}
	asciiMarker := draw.DialogOptionWidth("") // the widest marker, so the check holds for any theme
	if len(items) < 2 || 2*(asciiMarker+longest)+metaColumnGap > metaDialogWidth-4 {
		return dlg.Group{Columns: [][]dlg.Item{items}}
	}
	rows := (len(items) + 1) / 2
	first := 0
	for _, it := range items[:rows] {
		first = max(first, runewidth.StringWidth(it.Label))
	}
	out := make([]dlg.Item, rows)
	for r := range rows {
		left := items[r]
		if r+rows >= len(items) {
			out[r] = left
			continue
		}
		left.Gap = first - runewidth.StringWidth(left.Label) + metaColumnGap
		out[r] = dlg.Item{Kind: dlg.KindRow, Cells: []dlg.Item{left, items[r+rows]}}
	}
	return dlg.Group{Columns: [][]dlg.Item{out}}
}

// Apply folds a checkbox activation from dlg.HandleKey into the state. OK and Cancel are the
// caller's to handle.
func (s *MetaDialogState) Apply(ev dlg.Event) {
	for i := range s.Entries {
		if ev.ID == MetaEntryID(i) && i < len(s.Checked) {
			s.Checked[i] = !s.Checked[i]
			return
		}
	}
}

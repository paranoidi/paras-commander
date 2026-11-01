package dialog

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/findcrit"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// FindAdvField names a text input of the find dialog's advanced section.
type FindAdvField int

const (
	FindAdvModifiedNewer FindAdvField = iota
	FindAdvModifiedOlder
	FindAdvChangedNewer
	FindAdvChangedOlder
	FindAdvAccessedNewer
	FindAdvAccessedOlder
	FindAdvSizeMin
	FindAdvSizeMax
	FindAdvOwner
	FindAdvExclude
	FindAdvFieldCount
)

// IsAge reports whether f is one of the newer / older than fields.
func (f FindAdvField) IsAge() bool { return f <= FindAdvAccessedOlder }

// IsSize reports whether f is the size min or max field.
func (f FindAdvField) IsSize() bool { return f == FindAdvSizeMin || f == FindAdvSizeMax }

// Advanced section widget IDs. The text inputs are FindAdvFieldID(f), the match mode radios
// FindAdvModeID(m) and the file-type checkboxes FindAdvTypeID(i) (FilterTypeLabels order).
const (
	FindAdvanced      = dlg.ID("find:advanced") // the Advanced checkbox on the query row
	FindAdvEmpty      = dlg.ID("find:adv-empty")
	FindAdvSymlinks   = dlg.ID("find:adv-symlinks")
	FindAdvBroken     = dlg.ID("find:adv-broken")
	FindAdvExecutable = dlg.ID("find:adv-executable")
	FindAdvNotMine    = dlg.ID("find:adv-not-mine")
	FindAdvCase       = dlg.ID("find:adv-case")

	findAdvFieldPrefix = "find:adv-field-"
	findAdvModePrefix  = "find:adv-mode-"
	findAdvTypePrefix  = "find:adv-type-"
)

// FindAdvFieldID is the ID of advanced text input f.
func FindAdvFieldID(f FindAdvField) dlg.ID { return dlg.IndexedID(findAdvFieldPrefix, int(f)) }

// FindAdvFieldOf is the inverse of FindAdvFieldID.
func FindAdvFieldOf(id dlg.ID) (FindAdvField, bool) {
	i, ok := dlg.IDIndex(id, findAdvFieldPrefix)
	return FindAdvField(i), ok && i >= 0 && i < int(FindAdvFieldCount)
}

// FindAdvModeID is the ID of match mode radio m.
func FindAdvModeID(m findcrit.Mode) dlg.ID { return dlg.IndexedID(findAdvModePrefix, int(m)) }

// FindAdvModeOf is the inverse of FindAdvModeID.
func FindAdvModeOf(id dlg.ID) (findcrit.Mode, bool) {
	i, ok := dlg.IDIndex(id, findAdvModePrefix)
	return findcrit.Mode(i), ok && i >= 0 && i < len(findcrit.ModeLabels)
}

// FindAdvTypeID is the ID of file-type checkbox i.
func FindAdvTypeID(i int) dlg.ID { return dlg.IndexedID(findAdvTypePrefix, i) }

// FindAdvTypeOf is the inverse of FindAdvTypeID.
func FindAdvTypeOf(id dlg.ID) (int, bool) {
	i, ok := dlg.IDIndex(id, findAdvTypePrefix)
	return i, ok && i >= 0 && i < FilterTypeCount
}

// FindAdvInput is one advanced text input: its text, caret, scroll and validity.
type FindAdvInput struct {
	Value          string
	Cursor, Scroll int
	// Invalid marks text that does not parse (or an unknown owner); it is not applied.
	Invalid bool
}

// FindAdvancedState is the find dialog's advanced section: match mode and criteria. The find
// handler parses it into findcrit.Criteria; it outlives the dialog for the session.
type FindAdvancedState struct {
	// Expanded shows the section (find.toggle-advanced); collapsed, the active criteria are
	// summarized on the separator above the results.
	Expanded                                                     bool
	Mode                                                         findcrit.Mode
	Fields                                                       [FindAdvFieldCount]FindAdvInput
	Empty, Symlinks, Broken, Executables, NotMine, CaseSensitive bool
	Types                                                        [FilterTypeCount]bool
}

// Active reports whether any criterion or a non-fuzzy match mode is set.
func (a FindAdvancedState) Active() bool { return len(a.summaryTerms()) > 0 }

// Clear resets the mode and every criterion, keeping Expanded.
func (a *FindAdvancedState) Clear() { *a = FindAdvancedState{Expanded: a.Expanded} }

func (a FindAdvancedState) value(f FindAdvField) string {
	in := a.Fields[f]
	if in.Invalid {
		return ""
	}
	return strings.TrimSpace(in.Value)
}

// summaryTerms are the applied settings in display order ("modified newer 3d", "regex", ...).
func (a FindAdvancedState) summaryTerms() []string {
	var out []string
	if tag := a.Mode.Tag(); tag != "" {
		out = append(out, tag)
	}
	if a.CaseSensitive {
		out = append(out, "case")
	}
	for _, t := range []struct {
		name         string
		newer, older FindAdvField
	}{
		{"modified", FindAdvModifiedNewer, FindAdvModifiedOlder},
		{"changed", FindAdvChangedNewer, FindAdvChangedOlder},
		{"accessed", FindAdvAccessedNewer, FindAdvAccessedOlder},
	} {
		if v := a.value(t.newer); v != "" {
			out = append(out, t.name+" newer "+v)
		}
		if v := a.value(t.older); v != "" {
			out = append(out, t.name+" older "+v)
		}
	}
	if v := a.value(FindAdvSizeMin); v != "" {
		out = append(out, "size ≥"+v)
	}
	if v := a.value(FindAdvSizeMax); v != "" {
		out = append(out, "size ≤"+v)
	}
	if v := a.value(FindAdvOwner); v != "" {
		out = append(out, "owner "+v)
	}
	if a.NotMine {
		out = append(out, "not mine")
	}
	for _, k := range []struct {
		on   bool
		name string
	}{{a.Empty, "empty"}, {a.Symlinks, "symlinks"}, {a.Broken, "broken symlinks"}, {a.Executables, "executables"}} {
		if k.on {
			out = append(out, k.name)
		}
	}
	for i, on := range a.Types {
		if on {
			out = append(out, strings.ToLower(FilterTypeLabels[i]))
		}
	}
	if v := a.value(FindAdvExclude); v != "" {
		out = append(out, "exclude "+v)
	}
	return out
}

// Summary is the criteria summary for the separator above the results, fitted to width cells
// ("" when nothing is set).
func (a FindAdvancedState) Summary(styles theme.Theme, width int) string {
	terms := a.summaryTerms()
	if len(terms) == 0 || width <= 0 {
		return ""
	}
	return primitive.TruncateRight(styles.IconDialogCriteria()+" "+strings.Join(terms, " · "), width)
}

// Layout of the advanced section: a label column, then per pair a short label and an input.
const (
	findAdvLabelW   = 10 // "Modified", "Exclude" ... column
	findAdvSubW     = 7  // "newer", "older", "min", "max" (with the input's lead cell)
	findAdvInputW   = 12
	findAdvOwnerW   = 14
	findAdvChunkGap = 3
	// findAdvPairW is the width of one "Label sub [input] sub [input]" chunk.
	findAdvPairW = findAdvLabelW + 2*(findAdvSubW+findAdvInputW)
)

// findAdvancedRows are the advanced section's rows for a row width of rowW cells: the three
// time rows beside size / owner / exclude when two chunks fit side by side (else one chunk per
// row), then the match mode, kind and file-type rows, each wrapping under its label.
func findAdvancedRows(a FindAdvancedState, rowW int) []dlg.Item {
	input := func(f FindAdvField, w int) dlg.Item {
		in := a.Fields[f]
		it := dlg.Item{ID: FindAdvFieldID(f), Kind: dlg.KindInput, Width: w, Value: in.Value, Cursor: in.Cursor, Scroll: in.Scroll, Invalid: in.Invalid}
		if w == 0 {
			it.Grow = true
		}
		return it
	}
	label := func(text string, w int) dlg.Item { return dlg.Item{Kind: dlg.KindLabel, Label: text, Width: w} }
	pair := func(name, a, b string, fa, fb FindAdvField) []dlg.Item {
		return []dlg.Item{
			label(name, findAdvLabelW), label(a, findAdvSubW), input(fa, findAdvInputW),
			label(b, findAdvSubW), input(fb, findAdvInputW),
		}
	}
	left := [][]dlg.Item{
		pair("Modified", "newer", "older", FindAdvModifiedNewer, FindAdvModifiedOlder),
		pair("Changed", "newer", "older", FindAdvChangedNewer, FindAdvChangedOlder),
		pair("Accessed", "newer", "older", FindAdvAccessedNewer, FindAdvAccessedOlder),
	}
	right := [][]dlg.Item{
		pair("Size", "min", "max", FindAdvSizeMin, FindAdvSizeMax),
		{
			label("Owner", findAdvLabelW), input(FindAdvOwner, findAdvOwnerW),
			{ID: FindAdvNotMine, Kind: dlg.KindCheckbox, Label: "Not mine", On: a.NotMine, Seg: 1},
		},
		{label("Exclude", findAdvLabelW), input(FindAdvExclude, 0)},
	}
	var rows []dlg.Item
	sideBySide := rowW >= 2*findAdvPairW+findAdvChunkGap
	for i := range left {
		if sideBySide {
			cells := append([]dlg.Item(nil), left[i]...)
			cells[len(cells)-1].Gap = findAdvChunkGap
			rows = append(rows, dlg.Item{Kind: dlg.KindRow, Cells: append(cells, right[i]...)})
			continue
		}
		rows = append(rows, dlg.Item{Kind: dlg.KindRow, Cells: left[i]})
	}
	if !sideBySide {
		for _, cells := range right {
			rows = append(rows, dlg.Item{Kind: dlg.KindRow, Cells: cells})
		}
	}

	var modes []dlg.Item
	for m, text := range findcrit.ModeLabels {
		modes = append(modes, dlg.Item{ID: FindAdvModeID(findcrit.Mode(m)), Kind: dlg.KindRadio, Label: text, On: a.Mode == findcrit.Mode(m), Seg: 2, Gap: 1})
	}
	modes[len(modes)-1].Gap = findAdvChunkGap
	modes = append(modes, dlg.Item{ID: FindAdvCase, Kind: dlg.KindCheckbox, Label: "Case sensitive", On: a.CaseSensitive, Seg: 3})
	rows = append(rows, flowRows("Match", modes, rowW)...)

	kinds := []dlg.Item{
		{ID: FindAdvEmpty, Kind: dlg.KindCheckbox, Label: "Empty", On: a.Empty},
		{ID: FindAdvSymlinks, Kind: dlg.KindCheckbox, Label: "Symlinks", On: a.Symlinks},
		{ID: FindAdvBroken, Kind: dlg.KindCheckbox, Label: "Broken symlinks", On: a.Broken},
		{ID: FindAdvExecutable, Kind: dlg.KindCheckbox, Label: "Executables", On: a.Executables},
	}
	for i := range kinds {
		kinds[i].Seg, kinds[i].Gap = 4, 1
	}
	rows = append(rows, flowRows("Kind", kinds, rowW)...)

	types := make([]dlg.Item, FilterTypeCount)
	for i := range types {
		types[i] = dlg.Item{ID: FindAdvTypeID(i), Kind: dlg.KindCheckbox, Label: FilterTypeLabels[i], On: a.Types[i], Seg: 5, Gap: 1}
	}
	return append(rows, flowRows("Types", types, rowW)...)
}

// flowRows lays items out after a label cell, wrapping onto further rows (indented under the
// label) where the next item would not fit in rowW cells.
func flowRows(name string, items []dlg.Item, rowW int) []dlg.Item {
	head := dlg.Item{Kind: dlg.KindLabel, Label: name, Width: findAdvLabelW}
	var rows []dlg.Item
	cells := []dlg.Item{head}
	used := findAdvLabelW
	for _, it := range items {
		w := draw.DialogOptionWidth(it.Label) + it.Gap
		if len(cells) > 1 && used+w > rowW {
			rows = append(rows, dlg.Item{Kind: dlg.KindRow, Cells: cells})
			cells = []dlg.Item{{Kind: dlg.KindLabel, Width: findAdvLabelW}}
			used = findAdvLabelW
		}
		cells = append(cells, it)
		used += w
	}
	return append(rows, dlg.Item{Kind: dlg.KindRow, Cells: cells})
}

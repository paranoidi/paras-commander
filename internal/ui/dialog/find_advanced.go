package dialog

import (
	"strings"
	"unicode"

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

// findAdvancedLabel is the label of the checkbox that shows the advanced section.
const findAdvancedLabel = "Advanced"

// FindAdvancedMnemonic is the letter of the Advanced checkbox to highlight for the chords bound
// to find.toggle-advanced: the first plain Alt+letter chord whose letter is in the label, upper
// case (0 when none is).
func FindAdvancedMnemonic(chords []string) rune {
	for _, c := range chords {
		rest, ok := strings.CutPrefix(c, "M-")
		if !ok || len(rest) != 1 {
			continue
		}
		r := unicode.ToUpper(rune(rest[0]))
		if strings.ContainsRune(strings.ToUpper(findAdvancedLabel), r) {
			return r
		}
	}
	return 0
}

// NoMatches reports whether a typed query has no results that more indexing could change: the
// index is complete and no match pass is pending (the query input is then drawn in error style,
// like the panel quick filter's no-match input).
func (s *FindDialogState) NoMatches() bool {
	return strings.TrimSpace(s.Query) != "" && s.IndexDone && !s.Indexing && !s.RankPending && len(s.Ranked) == 0
}

// textInputFocused reports whether focus is on a text input (the query, depth or an advanced
// field), where letters are text.
func (s *FindDialogState) textInputFocused() bool {
	if _, ok := FindAdvFieldOf(s.Focus); ok {
		return true
	}
	return s.Focus == "" || s.Focus == FindQuery || s.Focus == FindDepth
}

// AdvancedFocused reports whether focus is on a widget of the advanced section (the result
// list's keys do not reach it then, so the list is drawn inert).
func (s *FindDialogState) AdvancedFocused() bool {
	id := s.Focus
	if _, ok := FindAdvFieldOf(id); ok {
		return true
	}
	if _, ok := FindAdvModeOf(id); ok {
		return true
	}
	if _, ok := FindAdvTypeOf(id); ok {
		return true
	}
	switch id {
	case FindAdvEmpty, FindAdvSymlinks, FindAdvBroken, FindAdvExecutable, FindAdvNotMine, FindAdvCase:
		return true
	}
	return false
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
	findAdvChunkGap = 2
	// findAdvPairW is the width of one "Label sub [input] sub [input]" chunk.
	findAdvPairW = findAdvLabelW + 2*(findAdvSubW+findAdvInputW)
)

// findAdvancedRows are the advanced section's rows for a row width of rowW cells: the match
// mode, kind and file-type rows on one aligned grid, then the three time rows beside size /
// owner / exclude when two chunks fit side by side (else one chunk per row).
func findAdvancedRows(a FindAdvancedState, rowW int, letters bool) []dlg.Item {
	rows := findAdvancedLayout(a, rowW)
	if letters {
		for i := range rows {
			for j := range rows[i].Cells {
				rows[i].Cells[j].Mnemonic = findAdvMnemonics[rows[i].Cells[j].ID]
			}
		}
	}
	return rows
}

// findAdvHints are the example texts of the advanced inputs, shown while they are empty.
var findAdvHints = [FindAdvFieldCount]string{
	FindAdvModifiedNewer: "3d",
	FindAdvModifiedOlder: "2026-01-31",
	FindAdvChangedNewer:  "12h",
	FindAdvChangedOlder:  "1w2d",
	FindAdvAccessedNewer: "30m",
	FindAdvAccessedOlder: "6mo",
	FindAdvSizeMin:       "10M",
	FindAdvSizeMax:       "1.5G",
	FindAdvOwner:         "root or 1000",
	FindAdvExclude:       "node_modules, .git, *.o, build/out",
}

// findAdvMnemonics are the letters of the advanced section's options. They are live (and
// highlighted) only while focus is off the text inputs, where they work plain or with Alt; they
// avoid the letters of the rest of the dialog (N, Y, I, V, S, the Advanced key, OK, Cancel).
var findAdvMnemonics = map[dlg.ID]rune{
	FindAdvModeID(findcrit.ModeFuzzy): 'F',
	FindAdvModeID(findcrit.ModeGlob):  'G',
	FindAdvModeID(findcrit.ModeRegex): 'X',
	FindAdvCase:                       'E',
	FindAdvEmpty:                      'P',
	FindAdvSymlinks:                   'K',
	FindAdvBroken:                     'B',
	FindAdvExecutable:                 'L',
	FindAdvTypeID(0):                  'D', // Videos
	FindAdvTypeID(1):                  'R', // Pictures
	FindAdvTypeID(2):                  'H', // Archives
	FindAdvTypeID(3):                  'U', // Audio
	FindAdvTypeID(4):                  'M', // Documents
	FindAdvNotMine:                    'T',
}

// findAdvancedLayout lays out the advanced section's rows (see findAdvancedRows).
func findAdvancedLayout(a FindAdvancedState, rowW int) []dlg.Item {
	input := func(f FindAdvField, w int) dlg.Item {
		in := a.Fields[f]
		it := dlg.Item{
			ID: FindAdvFieldID(f), Kind: dlg.KindInput, Width: w, Hint: findAdvHints[f],
			Value: in.Value, Cursor: in.Cursor, Scroll: in.Scroll, Invalid: in.Invalid,
		}
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
	rows := findAdvancedOptionRows(a, rowW)
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

	return rows
}

// findAdvOptionGap is the blank cells after each option of the Match / Kind / Types grid.
const findAdvOptionGap = 2

// findAdvancedOptionRows are the Match, Kind and Types rows: their options share columns (each
// column as wide as its widest option), with as many columns as fit in rowW cells; a row with
// more options than columns continues on the next row, indented under its label.
func findAdvancedOptionRows(a FindAdvancedState, rowW int) []dlg.Item {
	var modes []dlg.Item
	for m, text := range findcrit.ModeLabels {
		modes = append(modes, dlg.Item{ID: FindAdvModeID(findcrit.Mode(m)), Kind: dlg.KindRadio, Label: text, On: a.Mode == findcrit.Mode(m), Seg: 2})
	}
	modes = append(modes, dlg.Item{ID: FindAdvCase, Kind: dlg.KindCheckbox, Label: "Case sensitive", On: a.CaseSensitive, Seg: 3})
	kinds := []dlg.Item{
		{ID: FindAdvEmpty, Kind: dlg.KindCheckbox, Label: "Empty", On: a.Empty, Seg: 4},
		{ID: FindAdvSymlinks, Kind: dlg.KindCheckbox, Label: "Symlinks", On: a.Symlinks, Seg: 4},
		{ID: FindAdvBroken, Kind: dlg.KindCheckbox, Label: "Broken symlinks", On: a.Broken, Seg: 4},
		{ID: FindAdvExecutable, Kind: dlg.KindCheckbox, Label: "Executables", On: a.Executables, Seg: 4},
	}
	types := make([]dlg.Item, FilterTypeCount)
	for i := range types {
		types[i] = dlg.Item{ID: FindAdvTypeID(i), Kind: dlg.KindCheckbox, Label: FilterTypeLabels[i], On: a.Types[i], Seg: 5}
	}
	return optionGridRows([]string{"Match", "Kind", "Types"}, [][]dlg.Item{modes, kinds, types}, rowW)
}

// optionGridRows lays each labelled option row out on shared columns (see
// findAdvancedOptionRows); the column count is the largest that fits rowW.
func optionGridRows(names []string, groups [][]dlg.Item, rowW int) []dlg.Item {
	maxItems := 0
	for _, g := range groups {
		maxItems = max(maxItems, len(g))
	}
	var widths []int
	for cols := maxItems; cols >= 1; cols-- {
		widths = make([]int, cols)
		for _, g := range groups {
			for i, it := range g {
				widths[i%cols] = max(widths[i%cols], draw.DialogOptionWidth(it.Label)+findAdvOptionGap)
			}
		}
		total := findAdvLabelW
		for _, w := range widths {
			total += w
		}
		if total <= rowW {
			break
		}
	}
	cols := len(widths)
	var rows []dlg.Item
	for gi, g := range groups {
		for from := 0; from < len(g); from += cols {
			head := dlg.Item{Kind: dlg.KindLabel, Width: findAdvLabelW}
			if from == 0 {
				head.Label = names[gi]
			}
			cells := []dlg.Item{head}
			for i, it := range g[from:min(from+cols, len(g))] {
				it.Width = widths[i]
				cells = append(cells, it)
			}
			rows = append(rows, dlg.Item{Kind: dlg.KindRow, Cells: cells})
		}
	}
	return rows
}

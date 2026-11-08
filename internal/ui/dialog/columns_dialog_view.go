package dialog

import (
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/columnprofile"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// ColumnsPhase is the screen the Columns dialog shows.
type ColumnsPhase int

const (
	// ColumnsPhaseMain is the column checkboxes.
	ColumnsPhaseMain ColumnsPhase = iota
	// ColumnsPhaseLoadPicker lists every profile; picking one fills the checkboxes.
	ColumnsPhaseLoadPicker
	// ColumnsPhaseOverwritePicker lists the saved profiles; picking one overwrites it.
	ColumnsPhaseOverwritePicker
	// ColumnsPhaseSavePrompt asks for a new profile's name and description.
	ColumnsPhaseSavePrompt
)

// ColumnsProfilePicker is the fuzzy-filtered profile list of the two picker phases.
type ColumnsProfilePicker struct {
	Items       []columnprofile.Profile
	Query       string
	QueryCursor int
	QueryScroll int
	Ranked      []int
	MatchRanges [][]search.Range
	Selected    int
	ListScroll  int
}

// ColumnsPromptField is one text input of the save prompt.
type ColumnsPromptField struct {
	Value  string
	Cursor int
	Scroll int
}

// ColumnsDialogState is the Columns dialog: which file and meta columns a panel shows, and the
// column profiles they can be saved to and loaded from.
type ColumnsDialogState struct {
	Open    bool
	PanelID int
	Phase   ColumnsPhase
	// FileColumns is the ticked file columns.
	FileColumns panel.ColumnSet
	Entries     []MetaEntry
	Checked     []bool // parallel to Entries
	// HiddenMeta is the meta columns of the panel or a loaded profile that the panel's directory
	// does not offer. They have no checkbox, and OK and save keep them.
	HiddenMeta []string
	Focus      dlg.ID // the focused widget of the current phase; empty is its first
	// MarkerWidth is the theme's checkbox marker width (dlg.MarkerWidth), set at open; 0 lays
	// out for the ASCII marker.
	MarkerWidth int
	// SearchIcon is the theme's search icon for the picker filter, set at open.
	SearchIcon string
	// DefaultProfile is the startup profile, marked in the pickers.
	DefaultProfile string
	Picker         ColumnsProfilePicker
	SaveName       ColumnsPromptField
	SaveDesc       ColumnsPromptField
}

// ColumnsFileID is the widget ID of the checkbox of file column i (panel.Columns order).
func ColumnsFileID(i int) dlg.ID { return dlg.IndexedID("file:", i) }

// MetaEntryID is the widget ID of the checkbox row for entry i.
func MetaEntryID(i int) dlg.ID { return dlg.IndexedID("entry:", i) }

// Widgets of the save prompt.
const (
	ColumnsSaveNameInput dlg.ID = "columns-save-name"
	ColumnsSaveDescInput dlg.ID = "columns-save-desc"
)

const (
	metaDialogWidth         = 56
	columnsPickerWidth      = 70
	columnsFileSectionLabel = "File"
	columnsMetaSectionLabel = "Meta"
)

// metaColumnGap is the gap cells after a first-column box in a two-column section; the marker
// width's trailing cell adds one more blank before the second column.
const metaColumnGap = 2

// ColumnsDialogView is the widget spec of the dialog's current phase on a screen screenH rows high.
func ColumnsDialogView(s ColumnsDialogState, screenH int) dlg.Spec {
	switch s.Phase {
	case ColumnsPhaseLoadPicker, ColumnsPhaseOverwritePicker:
		return columnsPickerView(s, screenH)
	case ColumnsPhaseSavePrompt:
		return columnsSaveView(s)
	default:
		return columnsMainView(s)
	}
}

// columnsMainView has one checkbox per column above OK / Cancel: the File section, then each meta
// entry Section under a separator headed by its name (entries with no section under Meta). A
// section with no entries is not shown. A section whose labels fit two side by side is laid out in
// two columns (metaSectionGroup). Plain o / c press the buttons.
func columnsMainView(s ColumnsDialogState) dlg.Spec {
	shortcuts := ColumnsDialogShortcuts(s.Entries)
	specs := panel.Columns()
	groups := []dlg.Group{columnsSectionHeader(columnsFileSectionLabel)}
	items := make([]dlg.Item, 0, len(specs))
	for i, spec := range specs {
		items = append(items, dlg.Item{
			ID: ColumnsFileID(i), Kind: dlg.KindCheckbox, Label: spec.Title, Mnemonic: shortcuts[i], On: s.FileColumns.Has(spec.ID),
		})
	}
	section := columnsFileSectionLabel
	for i, e := range s.Entries {
		label := e.Section
		if label == "" {
			label = columnsMetaSectionLabel
		}
		if label != section {
			groups = append(groups, metaSectionGroup(items), columnsSectionHeader(label))
			section, items = label, nil
		}
		items = append(items, dlg.Item{
			ID: MetaEntryID(i), Kind: dlg.KindCheckbox, Label: metaEntryDisplayLabel(e), Mnemonic: shortcuts[len(specs)+i],
			On: i < len(s.Checked) && s.Checked[i],
		})
	}
	return dlg.Spec{
		Title: "Columns", Width: metaDialogWidth, MinWidth: 28, MinHeight: 5,
		Groups: append(groups, metaSectionGroup(items)), MarkerWidth: s.MarkerWidth,
		Buttons: []dlg.Button{dlg.OKButton, dlg.CancelButton},
	}
}

func columnsSectionHeader(label string) dlg.Group {
	return dlg.Group{Columns: [][]dlg.Item{{{Kind: dlg.KindSeparator, Label: label}}}}
}

func columnsPickerView(s ColumnsDialogState, screenH int) dlg.Spec {
	title := "Load profile"
	if s.Phase == ColumnsPhaseOverwritePicker {
		title = "Overwrite profile"
	}
	pk := s.Picker
	return pickerSpec(title, columnsPickerWidth, screenH, s.SearchIcon, len(pk.Items),
		func(i int) string { return ColumnsProfileLine(pk.Items[i], s.DefaultProfile) },
		pk.Ranked, pk.MatchRanges, pk.Query, pk.QueryCursor, pk.QueryScroll, pk.Selected, pk.ListScroll)
}

// ColumnsProfileLine is a profile's picker row, also its fuzzy-search corpus.
func ColumnsProfileLine(p columnprofile.Profile, defaultProfile string) string {
	line := p.Name
	if p.Description != "" {
		line += " — " + p.Description
	}
	if p.Builtin {
		line += " (built-in)"
	}
	if p.Name == defaultProfile {
		line += " (default)"
	}
	return line
}

func columnsSaveView(s ColumnsDialogState) dlg.Spec {
	input := func(id dlg.ID, f ColumnsPromptField) dlg.Item {
		return dlg.Item{ID: id, Kind: dlg.KindInput, Value: f.Value, Cursor: f.Cursor, Scroll: f.Scroll}
	}
	return dlg.Spec{
		Title: "Save profile", Width: metaDialogWidth, MinWidth: 28,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{{
			{Kind: dlg.KindLabel, Label: "Name:"},
			input(ColumnsSaveNameInput, s.SaveName),
			{Kind: dlg.KindLabel, Label: "Description:"},
			input(ColumnsSaveDescInput, s.SaveDesc),
		}}}},
		Buttons: dlg.OKCancelButtons(),
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
func (s *ColumnsDialogState) Apply(ev dlg.Event) {
	if i, ok := dlg.IDIndex(ev.ID, "file:"); ok && i >= 0 && i < len(panel.Columns()) {
		id := panel.Columns()[i].ID
		if s.FileColumns.Has(id) {
			s.FileColumns = s.FileColumns.Without(id)
		} else {
			s.FileColumns = s.FileColumns.With(id)
		}
		return
	}
	for i := range s.Entries {
		if ev.ID == MetaEntryID(i) && i < len(s.Checked) {
			s.Checked[i] = !s.Checked[i]
			return
		}
	}
}

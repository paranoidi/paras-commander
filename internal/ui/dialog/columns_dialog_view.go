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
func ColumnsFileID(i int) dlg.ID { return dlg.IndexedID(columnsFileIDPrefix, i) }

// ColumnsFileIndex is the inverse of ColumnsFileID.
func ColumnsFileIndex(id dlg.ID) (int, bool) {
	i, ok := dlg.IDIndex(id, columnsFileIDPrefix)
	return i, ok && i >= 0 && i < len(panel.Columns())
}

// MetaEntryID is the widget ID of the checkbox row for entry i.
func MetaEntryID(i int) dlg.ID { return dlg.IndexedID(metaEntryIDPrefix, i) }

// Widgets of the save prompt.
const (
	ColumnsSaveNameInput dlg.ID = "columns-save-name"
	ColumnsSaveDescInput dlg.ID = "columns-save-desc"
)

const (
	columnsPromptWidth = 56
	columnsPickerWidth = 70
	columnsMinWidth    = 28
	// columnsScreenMargin is the cells a dialog leaves free on the screen (draw.ClampCenteredDialogRect).
	columnsScreenMargin     = 4
	columnsMetaSectionLabel = "Meta"
	// columnsGridColumns is how many checkbox columns the main screen has when the screen is wide
	// enough for them.
	columnsGridColumns = 3
	columnsGridGap     = 2
	// columnsGridMaxLabel caps the labels that widen the grid; a longer one is clipped.
	columnsGridMaxLabel = 22
	columnsFileIDPrefix = "file:"
	metaEntryIDPrefix   = "entry:"
)

// ColumnsDialogView is the widget spec of the dialog's current phase on a screenW by screenH screen.
func ColumnsDialogView(s ColumnsDialogState, screenW, screenH int) dlg.Spec {
	switch s.Phase {
	case ColumnsPhaseLoadPicker, ColumnsPhaseOverwritePicker:
		return columnsPickerView(s, screenH)
	case ColumnsPhaseSavePrompt:
		return columnsSaveView(s)
	default:
		return columnsMainView(s, screenW)
	}
}

// columnsMainView has one checkbox per column above OK / Cancel: the file columns with no heading,
// then each meta entry Section under a separator and a label row with its name (entries with no
// section under Meta). A section with no entries is not shown. Every section uses one grid (newColumnsGrid), so the
// columns line up across sections. Plain o / c press the buttons.
func columnsMainView(s ColumnsDialogState, screenW int) dlg.Spec {
	shortcuts := ColumnsDialogShortcuts(s.Entries)
	specs := panel.Columns()
	grid := newColumnsGrid(s, screenW)
	var groups []dlg.Group
	items := make([]dlg.Item, 0, len(specs))
	for i, spec := range specs {
		items = append(items, dlg.Item{
			ID: ColumnsFileID(i), Kind: dlg.KindCheckbox, Label: spec.Title, Mnemonic: shortcuts[i], On: s.FileColumns.Has(spec.ID),
		})
	}
	section := ""
	for i, e := range s.Entries {
		label := e.Section
		if label == "" {
			label = columnsMetaSectionLabel
		}
		if label != section {
			groups = append(groups, grid.group(items), columnsSectionHeader(label))
			section, items = label, nil
		}
		items = append(items, dlg.Item{
			ID: MetaEntryID(i), Kind: dlg.KindCheckbox, Label: metaEntryDisplayLabel(e), Mnemonic: shortcuts[len(specs)+i],
			On: i < len(s.Checked) && s.Checked[i],
		})
	}
	groups = append(groups, grid.group(items))
	if columnsHasDescriptions(s.Entries) {
		groups = append(groups, dlg.Group{Columns: [][]dlg.Item{{
			{Kind: dlg.KindSeparator},
			{Kind: dlg.KindLabel, Label: columnsFocusedDescription(s)},
		}}})
	}
	return dlg.Spec{
		Title: "Columns", Width: max(grid.dialogWidth(), columnsMinWidth), MinWidth: columnsMinWidth, MinHeight: 5,
		Groups: groups, MarkerWidth: s.MarkerWidth,
		Buttons: dlg.OKCancelButtons(),
	}
}

func columnsHasDescriptions(entries []MetaEntry) bool {
	for _, e := range entries {
		if e.Description != "" {
			return true
		}
	}
	return false
}

// columnsFocusedDescription is the description of the entry under the focus; it has no room in the
// grid. The row stays reserved while any entry has one, so the dialog does not change height.
func columnsFocusedDescription(s ColumnsDialogState) string {
	for i, e := range s.Entries {
		if s.Focus == MetaEntryID(i) {
			return e.Description
		}
	}
	return ""
}

// columnsGrid is the checkbox grid every section of the main screen shares.
type columnsGrid struct {
	columns   int
	cellWidth int
}

// newColumnsGrid sizes the grid from the longest label of all sections and the theme's marker, with
// as many columns, up to columnsGridColumns, as fit on the screen.
func newColumnsGrid(s ColumnsDialogState, screenW int) columnsGrid {
	longest := 0
	for _, spec := range panel.Columns() {
		longest = max(longest, runewidth.StringWidth(spec.Title))
	}
	for _, e := range s.Entries {
		longest = max(longest, runewidth.StringWidth(metaEntryDisplayLabel(e)))
	}
	marker := s.MarkerWidth
	if marker == 0 {
		marker = draw.DialogOptionWidth("") // 0 lays out for the ASCII marker
	}
	g := columnsGrid{columns: columnsGridColumns, cellWidth: marker + min(longest, columnsGridMaxLabel)}
	for g.columns > 1 && g.dialogWidth() > draw.MaxDialogWidth(screenW) {
		g.columns--
	}
	return g
}

func (g columnsGrid) dialogWidth() int {
	return draw.DialogWidthForContent(g.columns*g.cellWidth + (g.columns-1)*columnsGridGap)
}

// group lays out one section's checkboxes in fixed-width cells, filled column by column (the first
// columns get the extra ones).
func (g columnsGrid) group(items []dlg.Item) dlg.Group {
	rows := (len(items) + g.columns - 1) / g.columns
	out := make([]dlg.Item, rows)
	for r := range rows {
		var cells []dlg.Item
		for c := range g.columns {
			i := c*rows + r
			if i >= len(items) {
				break
			}
			cell := items[i]
			cell.Width, cell.Gap = g.cellWidth, columnsGridGap
			cells = append(cells, cell)
		}
		out[r] = dlg.Item{Kind: dlg.KindRow, Cells: cells}
	}
	return dlg.Group{Columns: [][]dlg.Item{out}}
}

// columnsSectionHeader starts a section the way the preview settings dialog does: a plain
// separator, then the section's name on a label row.
func columnsSectionHeader(label string) dlg.Group {
	return dlg.Group{Columns: [][]dlg.Item{{{Kind: dlg.KindSeparator}, {Kind: dlg.KindLabel, Label: label + ":"}}}}
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
		Title: "Save profile", Width: columnsPromptWidth, MinWidth: 28,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{{
			{Kind: dlg.KindLabel, Label: "Name:"},
			input(ColumnsSaveNameInput, s.SaveName),
			{Kind: dlg.KindLabel, Label: "Description:"},
			input(ColumnsSaveDescInput, s.SaveDesc),
		}}}},
		Buttons: dlg.OKCancelButtons(),
	}
}

// Apply folds a checkbox activation from dlg.HandleKey into the state. OK and Cancel are the
// caller's to handle.
func (s *ColumnsDialogState) Apply(ev dlg.Event) {
	if i, ok := ColumnsFileIndex(ev.ID); ok {
		id := panel.Columns()[i].ID
		if s.FileColumns.Has(id) {
			s.FileColumns = s.FileColumns.Without(id)
		} else {
			s.FileColumns = s.FileColumns.With(id)
		}
		return
	}
	if i, ok := dlg.IDIndex(ev.ID, metaEntryIDPrefix); ok && i >= 0 && i < len(s.Checked) {
		s.Checked[i] = !s.Checked[i]
	}
}

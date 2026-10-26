package dialog

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Widget IDs of the help dialog. An empty state Focus is the filter. The entry list has no focus
// of its own: the filter drives it, and its section titles are Header rows.
const (
	HelpFilter = dlg.ID("help-filter")
	HelpClose  = dlg.ID("close")
	helpList   = dlg.ID("help-list")
)

// HelpDialogView is the help dialog's widget spec: the search filter, a rule, the entry list (the
// filter drives it; section titles are Header rows, the key column a span) and a Close button. ok
// is false when the dialog does not fit the layout.
func HelpDialogView(state HelpViewState, layout Layout, styles theme.Theme) (spec dlg.Spec, listH int, ok bool) {
	prefW, listH, ok := helpDialogSize(layout)
	title := state.Title
	if title == "" {
		title = "Help"
	}
	spec = dlg.Spec{
		Title: title, Width: prefW, MinWidth: prefW, Unclamped: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{{
			{ID: HelpFilter, Kind: dlg.KindInput, Drives: helpList, LeadingIcon: styles.IconSearchIcon(), Value: state.Query, Cursor: state.QueryCursor, Scroll: state.QueryScroll},
			{Kind: dlg.KindSeparator},
			{ID: helpList, Kind: dlg.KindList, Driven: true, Height: listH},
		}}}},
		Buttons: []dlg.Button{{ID: HelpClose, Label: "Close", Mnemonic: 'C'}},
	}
	// The rows are formatted to the list's placed width (key column, title), so place it first.
	width := 0
	if p, placed := dlg.Place(layout.Width, layout.Height, spec); placed {
		pi, _ := p.Item(helpList)
		width = pi.Rect.Width
	}
	spec.Groups[0].Columns[0][2] = helpListItem(state, listH, width, styles)
	return spec, listH, ok
}

// helpListItem is the entry list: the visual rows of BuildHelpVisualRows (section Header rows and
// entries formatted by FormatHelpRow to width, with their match ranges and the key column as a
// span) with the cursor on the selected entry. With width <= 0 the rows carry only their Header
// flags, enough for the viewport (HelpViewState.EnsureListScroll).
func helpListItem(st HelpViewState, height, width int, styles theme.Theme) dlg.Item {
	visual := BuildHelpVisualRows(st.Entries, st.Ranked)
	_, bg, _ := styles.DialogSurface.Decompose()
	keyStyle := styles.DialogHelpKey.Background(bg)
	keyCol := min(28, width-3)
	rows := make([]dlg.ListRow, len(visual))
	cursor := 0
	for i, vr := range visual {
		switch {
		case vr.IsHeader:
			rows[i] = dlg.ListRow{Text: vr.Header, Header: true}
		case vr.RankedIdx == st.Selected:
			cursor = i
		}
		if vr.IsHeader || width <= 0 {
			continue
		}
		entIdx := st.Ranked[vr.RankedIdx]
		if entIdx < 0 || entIdx >= len(st.Entries) {
			continue
		}
		line, keyStart, keyEnd := FormatHelpRow(st.Entries[entIdx], keyCol, width)
		rows[i].Text = line
		if entIdx < len(st.MatchRanges) {
			rows[i].Match = st.MatchRanges[entIdx]
		}
		if keyStart >= 0 {
			rows[i].Spans = []primitive.Span{{Start: keyStart, End: keyEnd, Style: keyStyle}}
		}
	}
	return dlg.Item{
		ID: helpList, Kind: dlg.KindList, Driven: true, Height: height, Rows: rows, Cursor: cursor, Offset: st.ListScroll,
	}
}

// DrawHelpDialog renders the centered help dialog with fuzzy shortcut search from its widget spec.
func DrawHelpDialog(screen tcell.Screen, layout Layout, state HelpViewState, styles theme.Theme) {
	if spec, _, fits := HelpDialogView(state, layout, styles); fits {
		dlg.Draw(screen, layout.Width, layout.Height, spec, state.Focus, styles)
	}
}

// FormatHelpRow builds a single text line for a help entry: the shortcut keys right-aligned
// within a fixed-width column on the left, then a guaranteed one-space gap, then the title.
// Keys wider than keyColWidth still get their one-space gap (the column grows for that row).
// keyStart/keyEnd is the rune range where the key text sits, for styling it distinctly from
// the rest of the row.
func FormatHelpRow(ent HelpEntry, keyColWidth, width int) (line string, keyStart, keyEnd int) {
	if width <= 0 {
		return "", 0, 0
	}
	keys := []rune(ent.Keys)
	pad := keyColWidth - len(keys)
	if pad < 0 {
		pad = 0
	}
	keyStart, keyEnd = pad, pad+len(keys)
	line = strings.Repeat(" ", pad) + string(keys) + " " + ent.Title
	if r := []rune(line); len(r) > width {
		line = string(r[:width])
	}
	return line, keyStart, keyEnd
}

// HelpVisualRow is one on-screen row of the help list: a non-selectable section-header row
// (IsHeader true, Header set) or an entry row (IsHeader false, RankedIdx indexes into
// Ranked/Entries).
type HelpVisualRow struct {
	IsHeader  bool
	Header    string
	RankedIdx int
}

// BuildHelpVisualRows expands Ranked into on-screen rows, inserting a section-header row
// before each run of same-section entries, in Ranked order — in browse mode (empty filter
// query) that's section-contiguous because Entries itself was built in section-priority order
// (see App.buildHelpEntriesForView); while filtering, Ranked is relevance-ordered instead, so
// a header can repeat if matches from the same section aren't adjacent in the ranked results.
// Entries follow their header immediately, with no spacer row between sections — the dialog's
// separator above the list already provides that break for the first header, and repeating it
// for every section would waste a row of vertical space.
func BuildHelpVisualRows(entries []HelpEntry, ranked []int) []HelpVisualRow {
	rows := make([]HelpVisualRow, 0, len(ranked))
	prevSection := ""
	havePrev := false
	for i, entIdx := range ranked {
		if entIdx < 0 || entIdx >= len(entries) {
			continue
		}
		section := entries[entIdx].Section
		if !havePrev || section != prevSection {
			rows = append(rows, HelpVisualRow{IsHeader: true, Header: section})
		}
		rows = append(rows, HelpVisualRow{RankedIdx: i})
		prevSection = section
		havePrev = true
	}
	return rows
}

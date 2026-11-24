package dialog

import (
	"cmp"
	"slices"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

const (
	// indexTagsColumns is how many columns the tag checkboxes are laid out in.
	indexTagsColumns = 2
	// indexTagsJumpMaxRunes is the longest jump text: what fits left of the title at indexTagsMinWidth.
	indexTagsJumpMaxRunes = 14
	indexTagsMinWidth     = 46
	indexTagsMinListRows  = 3
	indexTagsWidthPadding = 8 // borders, margins and the gap between the columns
)

// IndexTagsGrid is the widget ID of the tag grid in both dialogs that show it.
const IndexTagsGrid dlg.ID = "index-tags-grid"

// IndexTag is one checkbox of a tag grid: State as staged, Initial as it opened
// (dlg.CheckMixed when only some targets carry the tag).
type IndexTag struct {
	Name           string
	Initial, State dlg.CheckState
}

// IndexTagGrid is the tag checkbox grid of the tag editor and of Browse by tags, with its
// type-to-jump. Tags are sorted by name and fill the grid left to right, so tag i sits on row
// i/indexTagsColumns; Cursor, Col and Offset are the grid's cursor row, cell and first visible row.
// JumpQuery is the type-to-jump text and JumpIndex the current match of it (see jumpMatches).
type IndexTagGrid struct {
	Tags                []IndexTag
	Cursor, Col, Offset int
	JumpQuery           string
	JumpIndex           int
}

// tagAt is the index into Tags of the cell (row, col).
func tagAt(row, col int) int { return row*indexTagsColumns + col }

// cursorTag is the index into Tags under the grid's cursor.
func (g *IndexTagGrid) cursorTag() int { return tagAt(g.Cursor, g.Col) }

// moveTo puts the grid's cursor on tag i.
func (g *IndexTagGrid) moveTo(i int) {
	g.Cursor, g.Col = i/indexTagsColumns, i%indexTagsColumns
}

// ToggleCell flips the tag at cell (row, col): off and on swap, and a tag that opened mixed cycles
// mixed, on, off and back to mixed.
func (g *IndexTagGrid) ToggleCell(row, col int) {
	i := tagAt(row, col)
	if i < 0 || i >= len(g.Tags) {
		return
	}
	t := &g.Tags[i]
	switch {
	case t.State == dlg.CheckOn:
		t.State = dlg.CheckOff
	case t.State == dlg.CheckOff && t.Initial == dlg.CheckMixed:
		t.State = dlg.CheckMixed
	default:
		t.State = dlg.CheckOn
	}
}

// ToggleCursor is ToggleCell on the grid's cursor.
func (g *IndexTagGrid) ToggleCursor() { g.ToggleCell(g.Cursor, g.Col) }

// jumpMatches are the indexes into Tags that match query, best first, name order among equals.
func (g *IndexTagGrid) jumpMatches(query string) []int {
	type match struct{ tag, score int }
	var found []match
	for i, t := range g.Tags {
		if r := search.Fuzzy(query, t.Name, search.Options{CaseInsensitive: true}); r.Matched {
			found = append(found, match{i, r.Score})
		}
	}
	slices.SortStableFunc(found, func(a, b match) int { return cmp.Compare(b.score, a.score) })
	out := make([]int, len(found))
	for i, m := range found {
		out[i] = m.tag
	}
	return out
}

// setJump makes query the jump text and moves to its best match; with no match the cursor stays.
func (g *IndexTagGrid) setJump(query string) {
	g.JumpQuery, g.JumpIndex = query, 0
	if matches := g.jumpMatches(query); len(matches) > 0 {
		g.moveTo(matches[0])
	}
}

// jumpNoMatch reports whether there is jump text that no tag matches.
func (g *IndexTagGrid) jumpNoMatch() bool {
	return g.JumpQuery != "" && len(g.jumpMatches(g.JumpQuery)) == 0
}

// TypeJump appends r to the jump text, up to indexTagsJumpMaxRunes.
func (g *IndexTagGrid) TypeJump(r rune) {
	if utf8.RuneCountInString(g.JumpQuery) < indexTagsJumpMaxRunes {
		g.setJump(g.JumpQuery + string(r))
	}
}

// BackspaceJump drops the last rune of the jump text.
func (g *IndexTagGrid) BackspaceJump() {
	_, size := utf8.DecodeLastRuneInString(g.JumpQuery)
	query := g.JumpQuery[:len(g.JumpQuery)-size]
	if query == "" {
		g.ClearJump()
		return
	}
	g.setJump(query)
}

// CanCycleJump reports whether the jump text has several matches to cycle.
func (g *IndexTagGrid) CanCycleJump() bool { return len(g.jumpMatches(g.JumpQuery)) > 1 }

// CycleJump moves delta matches on, wrapping around.
func (g *IndexTagGrid) CycleJump(delta int) {
	matches := g.jumpMatches(g.JumpQuery)
	if len(matches) == 0 {
		return
	}
	g.JumpIndex = (g.JumpIndex + delta + len(matches)) % len(matches)
	g.moveTo(matches[g.JumpIndex])
}

// ClearJump ends type-to-jump; the cursor stays where it is.
func (g *IndexTagGrid) ClearJump() { g.JumpQuery, g.JumpIndex = "", 0 }

// Width is the dialog width that shows the longest tag in both columns.
func (g *IndexTagGrid) Width(styles theme.Theme) int {
	longest := 0
	for _, t := range g.Tags {
		longest = max(longest, runewidth.StringWidth(t.Name))
	}
	return max(indexTagsColumns*(longest+dlg.MarkerWidth(styles))+indexTagsWidthPadding, indexTagsMinWidth)
}

// Item is the checkbox grid; the cells matching the jump text show the matched letters.
func (g *IndexTagGrid) Item() dlg.Item {
	rows := (len(g.Tags) + indexTagsColumns - 1) / indexTagsColumns
	grid := dlg.Item{
		ID: IndexTagsGrid, Kind: dlg.KindList, Markers: true, CellColumns: indexTagsColumns, Scrollbar: true,
		Cursor: g.Cursor, Col: g.Col, Offset: g.Offset, Height: rows, MinHeight: min(indexTagsMinListRows, rows),
		Rows: make([]dlg.ListRow, rows),
	}
	for i, t := range g.Tags {
		cell := dlg.ListCell{Text: t.Name, Check: t.State}
		if g.JumpQuery != "" {
			cell.Match = search.Fuzzy(g.JumpQuery, t.Name, search.Options{CaseInsensitive: true}).Ranges
		}
		row := &grid.Rows[i/indexTagsColumns]
		row.Cells = append(row.Cells, cell)
	}
	return grid
}

// ApplyEvent applies a list-move or activation of the grid: it reports false for any other event.
func (g *IndexTagGrid) ApplyEvent(ev dlg.Event) bool {
	switch {
	case ev.ID != IndexTagsGrid:
		return false
	case ev.Kind == dlg.EventListMove:
		g.Cursor, g.Col, g.Offset = ev.Index, ev.Col, ev.Offset
	default:
		if !ev.Enter { // a click: it lands on the cell it toggles
			g.Cursor, g.Col = ev.Index, ev.Col
		}
		g.ToggleCell(ev.Index, ev.Col)
	}
	g.ClearJump()
	return true
}

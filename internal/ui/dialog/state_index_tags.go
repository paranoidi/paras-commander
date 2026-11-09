package dialog

import (
	"cmp"
	"slices"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// indexTagsColumns is how many columns the tag checkboxes are laid out in.
const indexTagsColumns = 2

// IndexTag is one checkbox of the fileindex tag editor: State as staged, Initial as it opened
// (dlg.CheckMixed when only some targets carry the tag).
type IndexTag struct {
	Name           string
	Initial, State dlg.CheckState
}

// IndexTagsDialogState is the fileindex tag editor. Edits stay staged here until OK. Tags are
// sorted by name and fill the grid left to right, so tag i sits on row i/indexTagsColumns; Cursor,
// Col and Offset are the grid's cursor row, cell and first visible row. JumpQuery is the
// type-to-jump text of the grid and JumpIndex the current match of it (see jumpMatches).
type IndexTagsDialogState struct {
	Open                bool
	Targets             []string
	Tags                []IndexTag
	Input               string
	InputCursor         int
	InputScroll         int
	Cursor, Col, Offset int
	JumpQuery           string
	JumpIndex           int
	Recursive           bool
	Focus               dlg.ID
}

// NewIndexTagsDialog opens the editor for targets over every tag of the database: a tag is on when
// all of the total targets carry it (counts[name]) and mixed when only some do. Focus starts on the
// grid, or on the new-tags input when the database has no tags.
func NewIndexTagsDialog(targetPaths []string, names []string, counts map[string]int, total int) IndexTagsDialogState {
	st := IndexTagsDialogState{Open: true, Targets: targetPaths, Focus: IndexTagsGrid}
	if len(names) == 0 {
		st.Focus = IndexTagsInput
	}
	for _, name := range names {
		state := dlg.CheckOff
		switch n := counts[name]; {
		case n > 0 && n >= total:
			state = dlg.CheckOn
		case n > 0:
			state = dlg.CheckMixed
		}
		st.Tags = append(st.Tags, IndexTag{Name: name, Initial: state, State: state})
	}
	return st
}

// tagAt is the index into Tags of the cell (row, col).
func tagAt(row, col int) int { return row*indexTagsColumns + col }

// cursorTag is the index into Tags under the grid's cursor.
func (st *IndexTagsDialogState) cursorTag() int { return tagAt(st.Cursor, st.Col) }

// moveTo puts the grid's cursor on tag i.
func (st *IndexTagsDialogState) moveTo(i int) {
	st.Cursor, st.Col = i/indexTagsColumns, i%indexTagsColumns
}

// ToggleCell flips the tag at cell (row, col): off and on swap, and a tag that opened mixed cycles
// mixed, on, off and back to mixed.
func (st *IndexTagsDialogState) ToggleCell(row, col int) {
	i := tagAt(row, col)
	if i < 0 || i >= len(st.Tags) {
		return
	}
	t := &st.Tags[i]
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
func (st *IndexTagsDialogState) ToggleCursor() { st.ToggleCell(st.Cursor, st.Col) }

// AddTags checks names, inserting the ones the database does not have yet in sorted position,
// and moves the grid's cursor to the last one.
func (st *IndexTagsDialogState) AddTags(names []string) {
	for _, name := range names {
		i, found := slices.BinarySearchFunc(st.Tags, name, func(t IndexTag, n string) int { return cmp.Compare(t.Name, n) })
		if found {
			st.Tags[i].State = dlg.CheckOn
		} else {
			st.Tags = slices.Insert(st.Tags, i, IndexTag{Name: name, State: dlg.CheckOn})
		}
	}
	st.ClearJump()
	if len(names) > 0 {
		i, _ := slices.BinarySearchFunc(st.Tags, names[len(names)-1], func(t IndexTag, n string) int { return cmp.Compare(t.Name, n) })
		st.moveTo(i)
	}
}

// Edits are the staged changes: the tags to add to and to remove from the targets. A tag back in
// its opening state is neither.
func (st *IndexTagsDialogState) Edits() (add, remove []string) {
	for _, t := range st.Tags {
		if t.State == t.Initial {
			continue
		}
		switch t.State {
		case dlg.CheckOn:
			add = append(add, t.Name)
		case dlg.CheckOff:
			remove = append(remove, t.Name)
		}
	}
	return add, remove
}

// jumpMatches are the indexes into Tags that match query, best first, name order among equals.
func (st *IndexTagsDialogState) jumpMatches(query string) []int {
	type match struct{ tag, score int }
	var found []match
	for i, t := range st.Tags {
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
func (st *IndexTagsDialogState) setJump(query string) {
	st.JumpQuery, st.JumpIndex = query, 0
	if matches := st.jumpMatches(query); len(matches) > 0 {
		st.moveTo(matches[0])
	}
}

// jumpNoMatch reports whether there is jump text that no tag matches.
func (st *IndexTagsDialogState) jumpNoMatch() bool {
	return st.JumpQuery != "" && len(st.jumpMatches(st.JumpQuery)) == 0
}

// TypeJump appends r to the jump text.
func (st *IndexTagsDialogState) TypeJump(r rune) { st.setJump(st.JumpQuery + string(r)) }

// BackspaceJump drops the last rune of the jump text.
func (st *IndexTagsDialogState) BackspaceJump() {
	_, size := utf8.DecodeLastRuneInString(st.JumpQuery)
	query := st.JumpQuery[:len(st.JumpQuery)-size]
	if query == "" {
		st.ClearJump()
		return
	}
	st.setJump(query)
}

// CanCycleJump reports whether the jump text has several matches to cycle.
func (st *IndexTagsDialogState) CanCycleJump() bool { return len(st.jumpMatches(st.JumpQuery)) > 1 }

// CycleJump moves delta matches on, wrapping around.
func (st *IndexTagsDialogState) CycleJump(delta int) {
	matches := st.jumpMatches(st.JumpQuery)
	if len(matches) == 0 {
		return
	}
	st.JumpIndex = (st.JumpIndex + delta + len(matches)) % len(matches)
	st.moveTo(matches[st.JumpIndex])
}

// ClearJump ends type-to-jump; the cursor stays where it is.
func (st *IndexTagsDialogState) ClearJump() { st.JumpQuery, st.JumpIndex = "", 0 }

package dialog

import (
	"cmp"
	"slices"

	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// IndexTagsDialogState is the fileindex tag editor. Edits stay staged here until OK. The grid's
// Cursor, Col and Offset are its cursor row, cell and first visible row.
type IndexTagsDialogState struct {
	Open bool
	IndexTagGrid
	Targets     []string
	Input       string
	InputCursor int
	InputScroll int
	HasDir      bool // a target is a directory, so Recursive can apply
	Recursive   bool
	Focus       dlg.ID
}

// NewIndexTagsDialog opens the editor for targets over every tag of the database: a tag is on when
// all of the total targets carry it (counts[name]) and mixed when only some do. Focus starts on the
// grid, or on the new-tags input when the database has no tags. hasDir tells whether any target is a
// directory.
func NewIndexTagsDialog(targetPaths []string, names []string, counts map[string]int, total int, hasDir bool) IndexTagsDialogState {
	st := IndexTagsDialogState{Open: true, Targets: targetPaths, HasDir: hasDir, Focus: IndexTagsGrid}
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

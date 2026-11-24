package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

// IndexTagBrowseDialogState is Browse by tags: the tag grid, how the checked tags match (all or
// any of them) and how the matches show (one flat list or in their directory structure).
type IndexTagBrowseDialogState struct {
	Open bool
	IndexTagGrid
	Any       bool
	Structure bool
	Focus     dlg.ID
}

// NewIndexTagBrowseDialog opens Browse by tags over every tag of the database with checked on, and
// the cursor on the first checked tag. Focus starts on the grid.
func NewIndexTagBrowseDialog(names, checked []string, anyTag, structure bool) IndexTagBrowseDialogState {
	st := IndexTagBrowseDialogState{Open: true, Any: anyTag, Structure: structure, Focus: IndexTagsGrid}
	first := -1
	for i, name := range names {
		state := dlg.CheckOff
		for _, c := range checked {
			if c == name {
				state = dlg.CheckOn
				if first < 0 {
					first = i
				}
			}
		}
		st.Tags = append(st.Tags, IndexTag{Name: name, State: state, Initial: dlg.CheckOff})
	}
	if first >= 0 {
		st.moveTo(first)
	}
	return st
}

// Checked are the names of the checked tags, in grid order.
func (st *IndexTagBrowseDialogState) Checked() []string {
	var out []string
	for _, t := range st.Tags {
		if t.State == dlg.CheckOn {
			out = append(out, t.Name)
		}
	}
	return out
}

// Apply folds a radio activation from dlg.HandleKey into the state; the grid, the checkboxes of
// the grid and the buttons are the caller's.
func (st *IndexTagBrowseDialogState) Apply(ev dlg.Event) {
	switch ev.ID {
	case IndexTagBrowseAll:
		st.Any = false
	case IndexTagBrowseAny:
		st.Any = true
	case IndexTagBrowseFlat:
		st.Structure = false
	case IndexTagBrowseStructure:
		st.Structure = true
	}
}

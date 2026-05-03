package ui

import "testing"

func TestJobsViewStateEnsureSelectionVisible(t *testing.T) {
	state := JobsViewState{Selected: 9, ListScroll: 0}

	state.EnsureSelectionVisible(10, 4)

	if state.Selected != 9 {
		t.Fatalf("Selected = %d, want 9", state.Selected)
	}
	if state.ListScroll != 6 {
		t.Fatalf("ListScroll = %d, want 6", state.ListScroll)
	}
}

func TestJobsViewStateEnsureSelectionVisibleEmpty(t *testing.T) {
	state := JobsViewState{Selected: 3, ListScroll: 2}

	state.EnsureSelectionVisible(0, 4)

	if state.Selected != 0 || state.ListScroll != 0 {
		t.Fatalf("state = %+v, want zero selection and scroll", state)
	}
}

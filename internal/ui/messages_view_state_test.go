package ui

import "testing"

func TestMessagesViewStateEnsureSelectionVisible(t *testing.T) {
	var state MessagesViewState
	state.Selected = 9
	state.EnsureSelectionVisible(10, 4)
	if state.Selected != 9 || state.ListScroll != 6 {
		t.Fatalf("Selected=%d ListScroll=%d, want Selected=9 ListScroll=6", state.Selected, state.ListScroll)
	}
}

func TestMessagesViewStateEnsureSelectionVisibleEmpty(t *testing.T) {
	var state MessagesViewState
	state.Selected = 3
	state.EnsureSelectionVisible(0, 4)
	if state.Selected != 0 || state.ListScroll != 0 {
		t.Fatalf("Selected=%d ListScroll=%d, want 0,0", state.Selected, state.ListScroll)
	}
}

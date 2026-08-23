package dialog

import "testing"

func twoSectionEntries() []HelpEntry {
	return []HelpEntry{
		{Title: "Alpha 1", Section: "Alpha"},
		{Title: "Alpha 2", Section: "Alpha"},
		{Title: "Alpha 3", Section: "Alpha"},
		{Title: "Beta 1", Section: "Beta"},
		{Title: "Beta 2", Section: "Beta"},
		{Title: "Beta 3", Section: "Beta"},
	}
}

// TestPageDownSelectionJumpsToLastVisibleItemNoScroll reproduces the reported PgDn regression:
// from the top of a freshly opened list, PgDn must select the last item already visible on
// screen — not the first item of the next section. Visual rows are
// [Ahdr,a1,a2,a3,Bhdr,b1,b2,b3]; with a 5-row viewport starting at Selected=a1 (row 1), the
// current view is rows [0,4] = [Ahdr,a1,a2,a3,Bhdr] — the last row (4) is the "Beta" header,
// not a selectable entry, so the last *item* actually visible is "Alpha 3" (row 3). Before the
// fix this landed one row further, on "Beta 1" — the first entry of the next, not-yet-visible
// section.
func TestPageDownSelectionJumpsToLastVisibleItemNoScroll(t *testing.T) {
	st := &HelpViewState{
		Entries:  twoSectionEntries(),
		Ranked:   []int{0, 1, 2, 3, 4, 5},
		Selected: 0, // Alpha 1
	}
	const listRows = 5
	st.Selected = st.PageDownSelection(listRows)
	st.EnsureListScroll(listRows)
	if st.Selected != 2 {
		t.Fatalf("Selected = %d, want 2 (Alpha 3)", st.Selected)
	}
	if st.ListScroll != 0 {
		t.Fatalf("ListScroll = %d, want 0 (the whole page already fit; no scroll should trigger)", st.ListScroll)
	}
}

// TestPageDownSelectionAdvancesFullPageWhenAlreadyAtBottom checks that once the selection is
// already the last visible item, a further PgDn scrolls a full page forward and lands on the
// bottom entry of the new page, rather than getting stuck.
func TestPageDownSelectionAdvancesFullPageWhenAlreadyAtBottom(t *testing.T) {
	st := &HelpViewState{
		Entries:  twoSectionEntries(),
		Ranked:   []int{0, 1, 2, 3, 4, 5},
		Selected: 2, // Alpha 3, visual row 3 — already the last item visible in [0,4]
	}
	const listRows = 5
	st.Selected = st.PageDownSelection(listRows)
	st.EnsureListScroll(listRows)
	// Visual rows: [Ahdr(0),a1(1),a2(2),a3(3),Bhdr(4),b1(5),b2(6),b3(7)]; row 3 + 5 = row 8,
	// clamped to the last row (7, "Beta 3").
	if st.Selected != 5 {
		t.Fatalf("Selected = %d, want 5 (Beta 3)", st.Selected)
	}
}

// TestPageUpSelectionJumpsToFirstVisibleItemNoScroll is PageDownSelection's mirror: from the
// bottom of the list, PgUp must select the first item already visible on screen.
func TestPageUpSelectionJumpsToFirstVisibleItemNoScroll(t *testing.T) {
	st := &HelpViewState{
		Entries:    twoSectionEntries(),
		Ranked:     []int{0, 1, 2, 3, 4, 5},
		Selected:   5, // Beta 3
		ListScroll: 3, // view = [3,7] = [a3, Bhdr, b1, b2, b3]; first item visible is a3
	}
	const listRows = 5
	st.Selected = st.PageUpSelection(listRows)
	st.EnsureListScroll(listRows)
	if st.Selected != 2 {
		t.Fatalf("Selected = %d, want 2 (Alpha 3)", st.Selected)
	}
}

// TestPageUpSelectionAdvancesFullPageWhenAlreadyAtTop checks that once the selection is already
// the first visible item, a further PgUp scrolls a full page backward.
func TestPageUpSelectionAdvancesFullPageWhenAlreadyAtTop(t *testing.T) {
	st := &HelpViewState{
		Entries:    twoSectionEntries(),
		Ranked:     []int{0, 1, 2, 3, 4, 5},
		Selected:   2, // Alpha 3, already the top item visible
		ListScroll: 3,
	}
	const listRows = 5
	st.Selected = st.PageUpSelection(listRows)
	if st.Selected != 0 {
		t.Fatalf("Selected = %d, want 0 (Alpha 1, clamped to the start of the list)", st.Selected)
	}
}

// TestPageDownUpSelectionClampToListEnds checks that paging past either end of the list clamps
// to the first/last entry instead of going out of range.
func TestPageDownUpSelectionClampToListEnds(t *testing.T) {
	st := &HelpViewState{
		Entries:  twoSectionEntries(),
		Ranked:   []int{0, 1, 2, 3, 4, 5},
		Selected: 5, // Beta 3, already the last entry
	}
	if got := st.PageDownSelection(100); got != 5 {
		t.Fatalf("PageDownSelection(100) = %d, want 5 (clamped to last entry)", got)
	}
	st.Selected = 0
	if got := st.PageUpSelection(100); got != 0 {
		t.Fatalf("PageUpSelection(100) = %d, want 0 (clamped to first entry)", got)
	}
}

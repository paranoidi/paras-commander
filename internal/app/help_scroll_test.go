package app

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func testHelpEntries() []dialog.HelpEntry {
	return []dialog.HelpEntry{
		{Title: "Up", Section: "Navigation"},
		{Title: "Down", Section: "Navigation"},
		{Title: "Copy", Section: "File operations"},
		{Title: "Move", Section: "File operations"},
	}
}

// TestEnsureHelpListScrollAccountsForHeaderRows checks that ListScroll (a visual-row offset)
// advances past section-header rows, not just entry rows, when scrolling to keep a selected
// entry in a later section visible — the header rows eat screen space too. Headers apply the
// same way whether or not a filter query is active.
func TestEnsureHelpListScrollAccountsForHeaderRows(t *testing.T) {
	st := &dialog.HelpViewState{
		Query:    "",
		Entries:  testHelpEntries(),
		Ranked:   []int{0, 1, 2, 3},
		Selected: 3, // "Move", the last entry, in the second section
	}
	// Visual rows: [Navigation, Up, Down, File operations, Copy, Move] = 6 rows (no blank
	// spacer). Move is row 5; with listRows=2, selecting it must scroll so ListScroll+2 > 5,
	// i.e. ListScroll >= 4.
	st.EnsureListScroll(2)
	if st.ListScroll < 4 {
		t.Fatalf("ListScroll = %d, want >= 4 (must account for the header rows)", st.ListScroll)
	}

	// Same selection, but filtering (Query != ""): headers still apply, same scroll math.
	st.Query = "x"
	st.ListScroll = 0
	st.EnsureListScroll(2)
	if st.ListScroll < 4 {
		t.Fatalf("filtering ListScroll = %d, want >= 4 (headers stay visible while filtering)", st.ListScroll)
	}
}

// TestEnsureHelpListScrollRevealsHeaderWhenScrollingBackToTop reproduces a reported bug:
// scrolling down to a later entry, then back up to the very first entry, must bring the first
// entry's section header back into view. Before the fix, ListScroll only ever snapped to the
// selected entry's own row, permanently hiding whatever header preceded it once ListScroll had
// advanced past that row.
func TestEnsureHelpListScrollRevealsHeaderWhenScrollingBackToTop(t *testing.T) {
	st := &dialog.HelpViewState{
		Query:    "",
		Entries:  testHelpEntries(),
		Ranked:   []int{0, 1, 2, 3},
		Selected: 3,
	}
	st.EnsureListScroll(2) // scroll down to the last entry, as above
	if st.ListScroll == 0 {
		t.Fatalf("setup: ListScroll = 0, expected to have scrolled down first")
	}

	st.Selected = 0 // scroll back up to the very first entry ("Up", under "Navigation")
	st.EnsureListScroll(2)
	if st.ListScroll != 0 {
		t.Fatalf("ListScroll = %d, want 0 (must reveal the Navigation header again)", st.ListScroll)
	}
}

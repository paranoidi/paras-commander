package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestQuickActionShortcutsPinnedAndDynamic(t *testing.T) {
	items := []QuickActionItem{
		{Key: 'g', Label: "lazygit"},
		{Label: "Print working directory"},
		{Label: "Open folder"},
	}
	got := QuickActionShortcuts(items)
	if got[0] != 'g' {
		t.Fatalf("pinned key: got %q, want g", got[0])
	}
	if got[1] != 'p' {
		t.Fatalf("dynamic print: got %q, want p", got[1])
	}
	if got[2] != 'o' {
		t.Fatalf("open folder: got %q, want o (first letter, unreserved unlike the old Cancel/OK dialog)", got[2])
	}
}

func TestQuickActionShortcutsNoReservedLetters(t *testing.T) {
	// The old user-menu dialog reserved 'c'/'o' for Cancel/OK; the buttonless
	// quick-action widget has no buttons, so "Copy" is now free to claim 'c'.
	items := []QuickActionItem{{Label: "Copy"}}
	got := QuickActionShortcuts(items)
	if got[0] != 'c' {
		t.Fatalf("copy shortcut = %q, want c (no OK/Cancel reservation)", got[0])
	}
}

func TestQuickActionIndexForKeyCaseInsensitive(t *testing.T) {
	items := []QuickActionItem{{Key: 'g', Label: "lazygit"}, {Label: "Print working directory"}}
	if i, ok := QuickActionIndexForKey(items, 'G'); !ok || i != 0 {
		t.Fatalf("uppercase match: i=%d ok=%v, want 0/true", i, ok)
	}
	if i, ok := QuickActionIndexForKey(items, 'p'); !ok || i != 1 {
		t.Fatalf("lowercase match: i=%d ok=%v, want 1/true", i, ok)
	}
	if _, ok := QuickActionIndexForKey(items, 'z'); ok {
		t.Fatal("unmatched letter should not resolve")
	}
	if _, ok := QuickActionIndexForKey(items, 0); ok {
		t.Fatal("zero rune should not resolve")
	}
}

func TestQuickActionEnsureScrollClamps(t *testing.T) {
	st := &QuickActionState{Items: make([]QuickActionItem, 10), Selected: 9}
	QuickActionEnsureScroll(st, 4)
	if st.ScrollOffset != 6 {
		t.Fatalf("ScrollOffset = %d, want 6 (selection kept visible)", st.ScrollOffset)
	}

	st2 := &QuickActionState{Items: make([]QuickActionItem, 10), Selected: 0, ScrollOffset: 5}
	QuickActionEnsureScroll(st2, 4)
	if st2.ScrollOffset != 0 {
		t.Fatalf("ScrollOffset = %d, want 0 (scroll pulled back to selection)", st2.ScrollOffset)
	}

	st3 := &QuickActionState{Items: nil, Selected: 3, ScrollOffset: 3}
	QuickActionEnsureScroll(st3, 4)
	if st3.ScrollOffset != 0 {
		t.Fatalf("empty list ScrollOffset = %d, want 0", st3.ScrollOffset)
	}
}

func TestQuickActionRectAnchoredClampedInsideLayout(t *testing.T) {
	layout := Layout{Width: 40, Height: 20}
	state := QuickActionState{Anchored: true, AnchorX: 1000, AnchorY: 1000}
	rect := quickActionRect(layout, state, 12, 5)
	if rect.X+rect.Width > layout.Width {
		t.Fatalf("rect extends past layout width: rect=%+v layout=%+v", rect, layout)
	}
	if rect.Y+rect.Height > layout.Height {
		t.Fatalf("rect extends past layout height: rect=%+v layout=%+v", rect, layout)
	}
	if rect.X < 0 || rect.Y < 0 {
		t.Fatalf("rect has negative origin: %+v", rect)
	}

	// Centered (not anchored) ignores AnchorX/AnchorY and still fits inside layout.
	state.Anchored = false
	rect = quickActionRect(layout, state, 12, 5)
	if rect.X < 0 || rect.Y < 0 || rect.X+rect.Width > layout.Width || rect.Y+rect.Height > layout.Height {
		t.Fatalf("centered rect out of bounds: rect=%+v layout=%+v", rect, layout)
	}
}

func newQuickActionTestScreen(t *testing.T, w, h int) tcell.SimulationScreen {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(w, h)
	t.Cleanup(screen.Fini)
	return screen
}

// runeIndex returns the index of the first occurrence of needle in haystack, or -1.
func runeIndex(haystack, needle []rune) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if string(haystack[i:i+len(needle)]) == string(needle) {
			return i
		}
	}
	return -1
}

func screenLine(screen tcell.SimulationScreen, y, width int) string {
	var line strings.Builder
	for x := 0; x < width; x++ {
		str, _, _ := screen.Get(x, y)
		line.WriteString(str)
	}
	return line.String()
}

func TestDrawQuickActionDialogRendersLetterAndLabel(t *testing.T) {
	screen := newQuickActionTestScreen(t, 80, 24)
	layout := Layout{Width: 80, Height: 22}
	state := QuickActionState{
		Open:  true,
		Title: "User menu",
		Items: []QuickActionItem{
			{Key: 'g', Label: "lazygit"},
			{Label: "Print working directory"},
		},
		Selected: 0,
	}

	DrawQuickActionDialog(screen, layout, state, theme.Default())

	var found bool
	for y := 0; y < 24; y++ {
		s := screenLine(screen, y, 80)
		if strings.Contains(s, " OK ") || strings.Contains(s, "Cancel") {
			t.Fatalf("quick action dialog must not show OK/Cancel buttons: %q", s)
		}
		runes := []rune(s)
		ridx := runeIndex(runes, []rune("lazygit"))
		if ridx < 3 {
			continue
		}
		// Layout: letter at label_start-3, two blank gap cells, then the label.
		if string(runes[ridx-3]) == "g" && strings.TrimSpace(string(runes[ridx-2:ridx])) == "" {
			found = true
		}
	}
	if !found {
		t.Fatal("quick action row (letter + 2-space gap + label) not found")
	}
}

func TestDrawQuickActionDialogTitledAndUntitledFrame(t *testing.T) {
	layout := Layout{Width: 80, Height: 22}
	items := []QuickActionItem{{Label: "Copy"}, {Label: "Move"}}

	titled := newQuickActionTestScreen(t, 80, 24)
	DrawQuickActionDialog(titled, layout, QuickActionState{Open: true, Title: "User menu", Items: items}, theme.Default())
	var sawTitle bool
	for y := 0; y < 24; y++ {
		if strings.Contains(screenLine(titled, y, 80), "User menu") {
			sawTitle = true
		}
	}
	if !sawTitle {
		t.Fatal("titled frame should render the title text")
	}

	untitled := newQuickActionTestScreen(t, 80, 24)
	DrawQuickActionDialog(untitled, layout, QuickActionState{Open: true, Items: items}, theme.Default())
	for y := 0; y < 24; y++ {
		if strings.Contains(screenLine(untitled, y, 80), "User menu") {
			t.Fatal("untitled frame should not render a title")
		}
	}
}

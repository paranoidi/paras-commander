package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/pathpick"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// fixedWidthCompletionItems returns n items whose names are all the same width, so the
// dropdown's computed box width only changes by the scrollbar reservation between test cases.
func fixedWidthCompletionItems(n int) []pathpick.Candidate {
	names := []string{"item0", "item1", "item2", "item3", "item4", "item5", "item6"}
	items := make([]pathpick.Candidate, n)
	for i := 0; i < n; i++ {
		items[i] = pathpick.Candidate{Name: names[i]}
	}
	return items
}

func TestDrawPathCompletionDropdownScrollbarAbsentWithFiveItems(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)

	var c PathCompletion
	c.Set("root", 0, fixedWidthCompletionItems(PathCompletionRows))

	const textX, y = 10, 5
	drawPathCompletionDropdown(screen, textX, y, 0, c, uiscrollbar.StyleThumb, theme.Default())

	// Box width without a scrollbar column is maxNameW(5) + sidePad(2) = 7, left edge at
	// textX-1, so the last column sits at textX-1+7-1 = textX+5.
	scrollbarCol := textX + 5
	for row := 0; row < PathCompletionRows; row++ {
		ch, _, _ := screen.Get(scrollbarCol, y+row)
		if ch != " " {
			t.Fatalf("row %d: expected no scrollbar glyph at col %d with %d items, got %q", row, scrollbarCol, PathCompletionRows, ch)
		}
	}
}

func TestDrawPathCompletionDropdownScrollbarShownWithSixItems(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)

	var c PathCompletion
	c.Set("root", 0, fixedWidthCompletionItems(PathCompletionRows+1))

	const textX, y = 10, 5
	drawPathCompletionDropdown(screen, textX, y, 0, c, uiscrollbar.StyleThumb, theme.Default())

	// Box width with the scrollbar column reserved is maxNameW(5) + sidePad(2) + 1 = 8, left
	// edge at textX-1, so the scrollbar's own column sits at textX-1+8-1 = textX+6.
	scrollbarCol := textX + 6
	found := false
	for row := 0; row < PathCompletionRows; row++ {
		ch, _, _ := screen.Get(scrollbarCol, y+row)
		if ch != " " {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a scrollbar glyph somewhere in column %d with %d items", scrollbarCol, PathCompletionRows+1)
	}

	// The column just left of the scrollbar is the right padding cell: painted with the row's
	// background, not left as whatever was underneath.
	padCol := scrollbarCol - 1
	_, wantBg, _ := theme.Default().DialogCompletionItem.Decompose()
	for row := 1; row < PathCompletionRows; row++ { // row 0 is the selected row
		_, st, _ := screen.Get(padCol, y+row)
		if _, bg, _ := st.Decompose(); bg != wantBg {
			t.Fatalf("row %d: padding cell col %d bg = %v, want %v", row, padCol, bg, wantBg)
		}
	}
	ch, _, _ := screen.Get(padCol, y)
	if ch == "" {
		t.Fatalf("expected padding content at col %d, got empty", padCol)
	}
}

func TestDrawPathCompletionDropdownWidthStaticWhileScrolling(t *testing.T) {
	boxRight := func(scroll int) int {
		screen := tcell.NewSimulationScreen("UTF-8")
		if err := screen.Init(); err != nil {
			t.Fatalf("Init: %v", err)
		}
		defer screen.Fini()
		screen.SetSize(80, 24)
		items := fixedWidthCompletionItems(PathCompletionRows + 1)
		items[len(items)-1].Name = "lantern-orchard-meadow" // only visible when scrolled down
		var c PathCompletion
		c.Set("root", 0, items)
		c.Scroll = scroll
		drawPathCompletionDropdown(screen, 10, 5, 0, c, uiscrollbar.StyleThumb, theme.Default())
		right := -1
		for x := 0; x < 80; x++ {
			if _, st, _ := screen.Get(x, 6); st != tcell.StyleDefault {
				right = x
			}
		}
		return right
	}
	if a, b := boxRight(0), boxRight(1); a != b {
		t.Fatalf("dropdown right edge moved while scrolling: %d at top, %d scrolled", a, b)
	}
}

package panelcarousel

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestSideColumnRowsTakeFolderColor(t *testing.T) {
	t.Parallel()
	tint := tcell.NewRGBColor(0x65, 0x43, 0x21)
	parent := Column{Kind: ColumnParent, Populated: true, Snapshot: panel.ListingSnapshot{
		Entries: []localfs.Entry{
			{Name: "orchard", Path: "/vol/orchard", Type: localfs.EntryDirectory},
			{Name: "lantern", Path: "/vol/lantern", Type: localfs.EntryDirectory},
			{Name: "pebble", Path: "/vol/pebble", Type: localfs.EntryDirectory},
		},
		Cursor: 1,
	}}
	body := BodyParams{
		Styles:      theme.Default(),
		FolderColor: func(p string) (tcell.Color, bool) { return tint, p != "/vol/pebble" },
	}
	fg := func(row int) tcell.Color {
		e, ok := resolveCarouselColumnEntry(carouselColumnParams{Body: body, C: parent, Inactive: true}, row)
		if !ok {
			t.Fatalf("row %d missing", row)
		}
		c, _, _ := e.Style.Decompose()
		return c
	}
	if fg(0) != tint {
		t.Fatal("coloured side-column row is not tinted")
	}
	// Row 1 is the cursor: the theme's cursor style decides (one without fg keeps the tint, like
	// it keeps the directory colour), so it is not asserted here.
	if fg(2) == tint {
		t.Fatal("uncoloured side-column row is tinted")
	}
	body.ChromeBlocked = true
	if fg(0) == tint {
		t.Fatal("blocked chrome still tinted")
	}
}

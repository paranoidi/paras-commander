package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func carouselTestEntries(dir string, dirs bool, names ...string) []localfs.Entry {
	typ := localfs.EntryFile
	if dirs {
		typ = localfs.EntryDirectory
	}
	out := make([]localfs.Entry, len(names))
	for i, n := range names {
		out[i] = localfs.Entry{Name: n, Path: dir + "/" + n, Type: typ}
	}
	return out
}

func carouselHitModel(center []localfs.Entry, cursor int) *Model {
	state := panel.State{Path: pathloc.MustParse("/harbor"), Entries: center, Cursor: cursor, CarouselMode: true, Columns: panel.ColumnsBrief}
	state.CarouselSideCache.Parent = panel.ListingSnapshot{Path: pathloc.MustParse("/"), Entries: carouselTestEntries("", true, "meadow", "harbor", "walnut")}
	state.CarouselSideCache.ParentOK = true
	state.CarouselSideCache.Child = panel.ListingSnapshot{Path: pathloc.MustParse("/harbor/lantern"), Entries: carouselTestEntries("/harbor/lantern", false, "orchid", "pepper")}
	state.CarouselSideCache.ChildOK = true
	return &Model{
		Primary:        state,
		ActivePanel:    PrimaryPanel,
		CarouselLayout: panelcarousel.DefaultLayout(),
		MetaResults:    [2][]MetaColumnState{PrimaryPanel: wideMetaColumns(1)},
	}
}

func TestPanelHeaderColumnAtMatchesCarouselHeader(t *testing.T) {
	model := carouselHitModel(carouselTestEntries("/harbor", false, "canyon", "ribbon"), 0)
	model.Primary.Sort = panel.SortState{Mode: panel.SortMeta, MetaColumn: "meadow0"}
	file := Rect{X: 0, Y: 1, Width: 150, Height: 12}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(file.Width, file.Y+file.Height)
	drawPanel(screen, file, model.Primary,
		PanelStyleConfig{Styles: theme.Default(), ScrollbarStyle: model.PanelScrollbar},
		PanelContext{PanelID: PrimaryPanel, FileListActive: true, ActivePanel: PrimaryPanel, SyncDriverPanelID: -1, QuickViewDriverPanelID: -1},
		PanelDisplayConfig{CarouselLayout: model.CarouselLayout, MetaColumns: model.MetaResults[PrimaryPanel]})

	spans := map[string]string{}
	y := file.Y + 1
	for x := file.X + 1; x < file.X+file.Width-1; x++ {
		col, onTitle, onHeader := PanelHeaderColumnAt(file, model, PrimaryPanel, false, x, y)
		if !onHeader {
			t.Fatalf("x=%d: not on the header row", x)
		}
		if !onTitle {
			continue
		}
		var b strings.Builder
		for cx := col.X; cx < col.X+col.Width; cx++ {
			r, _, _ := screen.Get(cx, y)
			b.WriteString(r)
		}
		key := col.MetaColumn
		if key == "" {
			key = col.Mode.String()
		}
		spans[key] = b.String()
	}
	// The drawn header carries the sort arrow of the sorted meta column, and the hit spans cover it.
	if got := spans["meadow0"]; !strings.Contains(got, "meadow0") || !strings.ContainsAny(got, "↑↓") {
		t.Fatalf("sorted meta title = %q, want the title with a sort arrow (all spans %v)", got, spans)
	}
	if spans["Name"] != "Name" || spans["Size"] != "Size" {
		t.Fatalf("spans = %v, want Name and Size titles", spans)
	}
}

func TestCarouselEntryAt(t *testing.T) {
	file := Rect{X: 0, Y: 1, Width: 150, Height: 12}
	layout := panelcarousel.DefaultLayout()
	cols := panelcarousel.SplitColumns(file, true, layout, [3]int{})
	model := carouselHitModel(carouselTestEntries("/harbor", true, "lantern", "velvet"), 0)

	at := func(m *Model, col, row int) (CarouselHit, bool) {
		hit, carousel, ok := CarouselEntryAt(file, m, PrimaryPanel, false, cols[col].X+1, cols[col].Y+row)
		if !carousel {
			t.Fatal("not laid out as a carousel")
		}
		return hit, ok
	}
	if hit, ok := at(model, 1, 1); !ok || hit.Column != 1 || hit.Index != 1 || hit.Entry.Name != "velvet" {
		t.Fatalf("center = %+v %v", hit, ok)
	}
	if _, ok := at(model, 1, 5); ok {
		t.Fatal("center row past the end hit")
	}
	if hit, ok := at(model, 0, 2); !ok || hit.Column != 0 || hit.Entry.Name != "walnut" || hit.Dir != "/" {
		t.Fatalf("parent = %+v %v", hit, ok)
	}
	if _, ok := at(model, 0, 3); ok {
		t.Fatal("parent row past the end hit")
	}
	// The child column lists the cursor directory.
	model.Primary.CarouselSideCache.Child.Entries = carouselTestEntries("/harbor/lantern", false, "orchid", "pepper")
	if hit, ok := at(model, 2, 1); !ok || hit.Column != 2 || hit.Entry.Name != "pepper" || hit.Dir != "/harbor/lantern" {
		t.Fatalf("child = %+v %v", hit, ok)
	}
	if _, ok := at(model, 2, 2); ok {
		t.Fatal("child row past the end hit")
	}

	// With the cursor on a file the child column is the file preview, not a listing.
	fileModel := carouselHitModel(append(carouselTestEntries("/harbor", true, "lantern"), carouselTestEntries("/harbor", false, "canyon")...), 1)
	hit, carousel, ok := CarouselEntryAt(file, fileModel, PrimaryPanel, true, cols[2].X+1, cols[2].Y)
	if !carousel || ok {
		t.Fatalf("file preview child = %+v carousel=%v ok=%v, want carousel without a hit", hit, carousel, ok)
	}
	if col, preview, ok := CarouselColumnAt(file, fileModel, PrimaryPanel, true, cols[2].X+1, cols[2].Y); !ok || col != 2 || !preview {
		t.Fatalf("CarouselColumnAt over the file preview = %d %v %v", col, preview, ok)
	}

	// A frame too narrow for the carousel falls back to the classic list.
	narrow := Rect{X: 0, Y: 1, Width: 20, Height: 12}
	if _, carousel, _ := CarouselEntryAt(narrow, model, PrimaryPanel, false, 3, narrow.Y+2); carousel {
		t.Fatal("narrow frame reported as a carousel")
	}
}

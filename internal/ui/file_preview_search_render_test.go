package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func searchBarModel(text string) Model {
	return Model{
		Primary:     panel.State{Path: pathloc.MustParse("/tmp")},
		Secondary:   panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: PrimaryPanel,
		ViewMode:    ViewFilePreview,
		FullscreenFilePreviewDraw: FilePreviewState{
			Open:         true,
			Phase:        FilePreviewPhaseDone,
			CombinedText: text,
			Search:       previewpanel.SearchState{Active: true, Editing: true, Current: -1},
		},
		FullscreenFilePreviewSearchField: dialog.FileDialogField{Value: "needle", Cursor: len([]rune("needle"))},
		FooterKeys:                       []menu.FunctionKey{{KeyLabel: "Esc", Hint: "Close"}},
	}
}

func TestRenderFilePreviewSearchBarNerdfontIcon(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	styles := theme.Default()
	styles.UseNerdfontIcons = true
	icon := styles.IconSearchIcon()
	Render(screen, searchBarModel("needle hay needle\n"), styles)

	line := tcelltest.TextAt(screen, 0, height-2, width)
	want := icon + " needle"
	if !strings.HasPrefix(line, want) {
		t.Fatalf("search row = %q, want prefix %q", line, want)
	}
	if strings.Contains(line, "search:") {
		t.Fatalf("search row = %q, want no \"search:\" text with icons enabled", line)
	}
	// The query starts right after the icon and one gap cell.
	if r, _, _ := screen.Get(dialog.LeadingIconCols(icon), height-2); r != "n" {
		t.Fatalf("first query cell = %q, want \"n\"", r)
	}
}

func TestRenderFilePreviewSearchBarIconStyle(t *testing.T) {
	const width, height = 80, 12
	iconStyle := func(styles theme.Theme) (icon, row tcell.Style) {
		screen := tcell.NewSimulationScreen("UTF-8")
		if err := screen.Init(); err != nil {
			t.Fatalf("Init() error = %v", err)
		}
		defer screen.Fini()
		screen.SetSize(width, height)
		model := searchBarModel("needle hay needle\n")
		model.FullscreenFilePreviewDraw.Search.Matches = []previewpanel.SearchMatch{{Start: 0, End: 6}}
		Render(screen, model, styles)
		_, icon, _ = screen.Get(0, height-2)
		_, row, _ = screen.Get(width-1, height-2)
		return icon, row
	}

	styles := theme.Default()
	styles.UseNerdfontIcons = true
	icon, row := iconStyle(styles)
	wantFG, _, _ := styles.PreviewSearchIcon.Decompose()
	_, rowBG, _ := row.Decompose()
	gotFG, gotBG, _ := icon.Decompose()
	if wantFG == tcell.ColorDefault || gotFG != wantFG {
		t.Fatalf("icon fg = %v, want preview.search_icon fg %v", gotFG, wantFG)
	}
	if gotBG != rowBG {
		t.Fatalf("icon bg = %v, want the row bg %v", gotBG, rowBG)
	}

	// A theme without preview.search_icon draws the icon in the row style.
	styles.PreviewSearchIcon = tcell.Style{}
	icon, row = iconStyle(styles)
	if icon != row {
		t.Fatalf("icon style = %v, want row style %v without the key", icon, row)
	}
}

func TestRenderFilePreviewSearchBarNoMatchNerdfontIcon(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	styles := theme.Default()
	styles.UseNerdfontIcons = true
	Render(screen, searchBarModel("hay hay hay\n"), styles)

	for _, col := range []int{0, dialog.LeadingIconCols(styles.IconSearchIcon())} {
		_, cellSt, _ := screen.Get(col, height-2)
		if cellSt != styles.FuzzyInputNomatch {
			t.Fatalf("col %d style = %v, want FuzzyInputNomatch", col, cellSt)
		}
	}
}

func TestRenderFilePreviewSearchBarAboveFooter(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	styles := theme.Default()
	search := previewpanel.SearchState{Active: true, Editing: true, Current: -1}
	model := Model{
		Primary:     panel.State{Path: pathloc.MustParse("/tmp")},
		Secondary:   panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: PrimaryPanel,
		ViewMode:    ViewFilePreview,
		FullscreenFilePreviewDraw: FilePreviewState{
			Open:         true,
			Phase:        FilePreviewPhaseDone,
			CombinedText: "needle hay needle\n",
			Search:       search,
		},
		FullscreenFilePreviewSearchField: dialog.FileDialogField{Value: "needle", Cursor: len([]rune("needle"))},
		FooterKeys:                       []menu.FunctionKey{{KeyLabel: "Esc", Hint: "Close"}},
	}

	Render(screen, model, styles)

	searchY := height - 2
	line := tcelltest.TextAt(screen, 0, searchY, width)
	if !strings.HasPrefix(line, "search: needle") {
		t.Fatalf("search row = %q, want prefix \"search: needle\"", line)
	}
	footer := tcelltest.TextAt(screen, 0, height-1, width)
	if !strings.Contains(footer, "Esc") {
		t.Fatalf("footer = %q, want Esc hint on row below search bar", footer)
	}
}

func TestRenderFilePreviewSearchBarNoMatch(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	styles := theme.Default()
	search := previewpanel.SearchState{Active: true, Editing: true, Current: -1}
	model := Model{
		Primary:     panel.State{Path: pathloc.MustParse("/tmp")},
		Secondary:   panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: PrimaryPanel,
		ViewMode:    ViewFilePreview,
		FullscreenFilePreviewDraw: FilePreviewState{
			Open:         true,
			Phase:        FilePreviewPhaseDone,
			CombinedText: "hay hay hay\n",
			Search:       search,
		},
		FullscreenFilePreviewSearchField: dialog.FileDialogField{Value: "needle", Cursor: len([]rune("needle"))},
		FooterKeys:                       []menu.FunctionKey{{KeyLabel: "Esc", Hint: "Close"}},
	}

	Render(screen, model, styles)

	searchY := height - 2
	// "search: " label is 8 columns wide; first query rune follows it.
	_, cellSt, _ := screen.Get(8, searchY)
	if cellSt != styles.FuzzyInputNomatch {
		t.Fatalf("search query cell style = %v, want FuzzyInputNomatch", cellSt)
	}
}

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

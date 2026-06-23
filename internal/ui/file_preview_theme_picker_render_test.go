package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestRenderFilePreviewThemePickerShowsAllLabels(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)

	styles := theme.Default()
	choices := []ThemeChoice{
		{Name: "monokai", Label: "monokai"},
		{Name: "github", Label: "github"},
		{Name: "dracula", Label: "dracula"},
	}
	model := Model{
		Primary:     panel.State{Path: pathloc.MustParse("/tmp")},
		Secondary:   panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: PrimaryPanel,
		ViewMode:    ViewFilePreview,
		FullscreenFilePreviewDraw: FilePreviewState{
			Open: true, Phase: FilePreviewPhaseDone, CombinedText: "hi\n",
		},
		FilePreviewThemePicker: FilePreviewThemePickerState{
			Open:         true,
			Choices:      choices,
			DisplayLines: []string{"monokai", "github", "dracula"},
			Ranked:       []int{0, 1, 2},
			Selected:     0,
		},
	}
	Render(screen, model, styles)

	layout := CalculateLayout(80, 24, true, PanelWidthSplit{})
	union := MergeTwinPanelRects(layout.Primary, layout.Secondary)
	_, picker := SplitFullscreenPreviewRects(union, true, choices)
	listTop := picker.Y + 3
	for i, want := range []string{"monokai", "github", "dracula"} {
		y := listTop + i
		got := strings.TrimSpace(tcelltest.TextAt(screen, picker.X+2, y, picker.Width-4))
		if !strings.Contains(got, want) {
			t.Fatalf("row %d = %q, want substring %q", i, got, want)
		}
	}
}

func TestRenderFilePreviewThemePickerShowsLabelsFromChoices(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)

	styles := theme.Default()
	choices := []ThemeChoice{
		{Name: "monokai", Label: "monokai"},
		{Name: "github", Label: "github"},
	}
	model := Model{
		Primary:     panel.State{Path: pathloc.MustParse("/tmp")},
		Secondary:   panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: PrimaryPanel,
		ViewMode:    ViewFilePreview,
		FullscreenFilePreviewDraw: FilePreviewState{
			Open: true, Phase: FilePreviewPhaseDone, CombinedText: "hi\n",
		},
		FilePreviewThemePicker: FilePreviewThemePickerState{
			Open:     true,
			Choices:  choices,
			Ranked:   []int{0, 1},
			Selected: 0,
		},
	}
	Render(screen, model, styles)

	layout := CalculateLayout(80, 24, true, PanelWidthSplit{})
	union := MergeTwinPanelRects(layout.Primary, layout.Secondary)
	_, picker := SplitFullscreenPreviewRects(union, true, choices)
	listTop := picker.Y + 3
	got := strings.TrimSpace(tcelltest.TextAt(screen, picker.X+2, listTop+1, picker.Width-4))
	if !strings.Contains(got, "github") {
		t.Fatalf("row 1 = %q, want github from Choices", got)
	}
}

func TestRenderFilePreviewThemePickerFilteredOmitsEmptyRows(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)

	choices := []ThemeChoice{
		{Name: "monokai", Label: "monokai"},
		{Name: "monokailight", Label: "monokailight"},
		{Name: "github", Label: "github"},
	}
	model := Model{
		Primary:     panel.State{Path: pathloc.MustParse("/tmp")},
		Secondary:   panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: PrimaryPanel,
		ViewMode:    ViewFilePreview,
		FullscreenFilePreviewDraw: FilePreviewState{
			Open: true, Phase: FilePreviewPhaseDone, CombinedText: "hi\n",
		},
		FilePreviewThemePicker: FilePreviewThemePickerState{
			Open:         true,
			Choices:      choices,
			DisplayLines: []string{"monokai", "monokailight", "github"},
			Ranked:       []int{0, 1},
			Selected:     0,
			Query:        "mono",
		},
	}
	Render(screen, model, theme.Default())

	layout := CalculateLayout(80, 24, true, PanelWidthSplit{})
	union := MergeTwinPanelRects(layout.Primary, layout.Secondary)
	_, picker := SplitFullscreenPreviewRects(union, true, choices)
	listTop := picker.Y + 3
	for row := 2; row < FilePreviewThemePickerListRows(picker); row++ {
		y := listTop + row
		got := strings.TrimSpace(tcelltest.TextAt(screen, picker.X+2, y, picker.Width-4))
		if strings.Contains(got, "( )") || strings.Contains(got, "(*)") {
			t.Fatalf("row %d should be blank after filtered matches, got %q", row, got)
		}
	}
}

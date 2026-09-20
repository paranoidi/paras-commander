package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// staleFullscreenLayout returns a Layout whose Menu rect is a non-zero row-0 rect, as if a
// caller computed it before MenuBarLayoutReserved() started reporting false for ViewFilePreview.
// The partial painters under test must ignore it rather than paint into fullscreen preview's
// row 0 (see MenuBarLayoutReserved's ViewFilePreview guard, internal/ui/render.go).
func staleFullscreenLayout(width int) Layout {
	return Layout{Width: width, Height: 24, Menu: Rect{X: 0, Y: 0, Width: width, Height: 1}}
}

func TestDrawMenuBarJobsGapOnlyIgnoresStaleLayoutInFullscreenPreview(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width = 80
	screen.SetSize(width, 24)
	sentinel := tcell.StyleDefault.Foreground(tcell.ColorFuchsia)
	for x := 0; x < width; x++ {
		screen.SetContent(x, 0, '#', nil, sentinel)
	}

	styles := theme.Default()
	model := Model{
		ViewMode: ViewFilePreview,
		MenuBarJobs: MenuBarJobsStrip{
			Groups:       []MenuBarJobGroup{{Status: "queued", Count: 2}},
			HasProgress:  true,
			ProgressFrac: 0.5,
		},
	}
	layout := staleFullscreenLayout(width)

	painted, exited := DrawMenuBarJobsGapOnly(screen, layout, model, testBrowserMenuDefinitions(t), styles)
	if painted || exited {
		t.Fatalf("painted=%v exited=%v, want false, false (ViewFilePreview must not paint)", painted, exited)
	}
	for x := 0; x < width; x++ {
		r, style, _ := screen.Get(x, 0)
		if r != "#" || style != sentinel {
			t.Fatalf("cell (%d,0) = %q/%v, want untouched sentinel", x, r, style)
		}
	}
}

func TestDrawMenuBarSpinnerOnlyIgnoresStaleLayoutInFullscreenPreview(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width = 80
	screen.SetSize(width, 24)
	sentinel := tcell.StyleDefault.Foreground(tcell.ColorFuchsia)
	for x := 0; x < width; x++ {
		screen.SetContent(x, 0, '#', nil, sentinel)
	}

	styles := theme.Default()
	model := Model{
		ViewMode:               ViewFilePreview,
		MenuBarActivitySpinner: false,
	}
	layout := staleFullscreenLayout(width)

	if painted := DrawMenuBarSpinnerOnly(screen, layout, model, styles); painted {
		t.Fatal("painted = true, want false (spinner not active)")
	}
	for x := 0; x < width; x++ {
		r, style, _ := screen.Get(x, 0)
		if r != "#" || style != sentinel {
			t.Fatalf("cell (%d,0) = %q/%v, want untouched sentinel", x, r, style)
		}
	}
}

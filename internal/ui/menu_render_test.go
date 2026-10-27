package ui

import (
	"unicode/utf8"

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

// With a modal open the menu bar is blank; the pin badge must still keep one margin cell
// after the status_command text instead of abutting it.
func TestDrawMenuBarBlankKeepsMarginAfterStatusCommand(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, statusW = 80, 6
	screen.SetSize(width, 24)
	styles := theme.Default()
	rect := Rect{X: statusW, Y: 0, Width: width - statusW, Height: 1}
	drawMenuBarBlank(screen, rect, styles, MenuBarJobsStrip{}, "", "", false, 0, 3)
	if r, _, _ := screen.Get(statusW, 0); r != " " {
		t.Fatalf("cell (%d,0) = %q, want margin space", statusW, r)
	}
	if r, _, _ := screen.Get(statusW+1, 0); r == " " {
		t.Fatalf("cell (%d,0) is blank, want pin badge", statusW+1)
	}
}

// The reserved width must equal what drawMenuBarStatusCommands paints: " t1 · t2 · ", each
// text in its segment style, gaps in menu.status and the separator in menu.status.separator.
func TestDrawMenuBarStatusCommandsMatchesWidth(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(40, 1)
	styles := theme.Default()
	colored := styles.MenuStatus.Foreground(tcell.ColorYellow)
	segs := []StatusCommandSegment{{Text: "0.52 0.48", Style: styles.MenuStatus}, {Text: "5.2G/31G", Style: colored}}
	w := StatusCommandWidth(segs)
	if want := utf8.RuneCountInString(" 0.52 0.48 · 5.2G/31G · "); w != want {
		t.Fatalf("StatusCommandWidth = %d, want %d", w, want)
	}
	drawMenuBarStatusCommands(screen, Rect{Width: w, Height: 1}, segs, styles)
	sep := string(styles.IconMenuStatusSeparator())
	var got string
	for x := range w {
		r, style, _ := screen.Get(x, 0)
		got += r
		want := styles.MenuStatus
		switch {
		case r == sep:
			want = styles.MenuStatusSeparator
		case x >= 13 && x < 21: // "5.2G/31G"
			want = colored
		}
		if style != want {
			t.Fatalf("cell %d %q style = %v, want %v", x, r, style, want)
		}
	}
	if want := " 0.52 0.48 " + sep + " 5.2G/31G " + sep + " "; got != want {
		t.Fatalf("drawn %q, want %q", got, want)
	}
	if StatusCommandWidth(nil) != 0 {
		t.Fatal("no segments must reserve nothing")
	}
}

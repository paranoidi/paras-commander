package ui

import (
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// TestDrawPanelRowQuickViewIndicatorSurvivesScrollbar guards against the indicator icon and the
// panel scrollbar sharing the same border column: the scrollbar paints its whole track/thumb
// after the row loop, so the indicator must be painted after the scrollbar too or it gets
// overdrawn as soon as a directory has more entries than fit on screen.
func TestDrawPanelRowQuickViewIndicatorSurvivesScrollbar(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	const width, height = 60, 10
	screen.SetSize(width, height)

	styles := theme.Default()
	root := "/vol"
	entries := make([]localfs.Entry, 30)
	for i := range entries {
		entries[i] = localfs.Entry{Name: "f.txt", Path: root + "/f.txt", Type: localfs.EntryFile}
	}
	state := panel.State{Path: pathloc.MustParse(root), Entries: entries, Cursor: 0}
	rect := Rect{X: 0, Y: 0, Width: width, Height: height}
	drawPanel(screen, rect, state,
		PanelStyleConfig{Styles: styles, ScrollbarStyle: uiscrollbar.StyleBar},
		PanelContext{
			PanelID: PrimaryPanel, FileListActive: true, CursorRowActive: true, ActivePanel: PrimaryPanel,
			SyncDriverPanelID: -1, QuickViewDriverPanelID: -1,
			QuickViewIndicator: true, QuickViewIndicatorRight: true,
		},
		PanelDisplayConfig{ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout()})

	x, cursorRowY := rect.X+rect.Width-1, rect.Y+2
	ch, _, _ := screen.Get(x, cursorRowY)
	r, _ := utf8.DecodeRuneInString(ch)
	if r != quickViewIndicatorIconRight {
		t.Fatalf("indicator icon at (%d,%d) = %q, want %q (scrollbar likely overdrew it)", x, cursorRowY, r, quickViewIndicatorIconRight)
	}
}

func TestDrawPanelRowQuickViewIndicatorIcon(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		right      bool
		wantIcon   rune
		wantColX   func(rect Rect) int
		unwantColX func(rect Rect) int
	}{
		{
			name:       "right",
			right:      true,
			wantIcon:   quickViewIndicatorIconRight,
			wantColX:   func(rect Rect) int { return rect.X + rect.Width - 1 },
			unwantColX: func(rect Rect) int { return rect.X },
		},
		{
			name:       "left",
			right:      false,
			wantIcon:   quickViewIndicatorIconLeft,
			wantColX:   func(rect Rect) int { return rect.X },
			unwantColX: func(rect Rect) int { return rect.X + rect.Width - 1 },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatalf("Init: %v", err)
			}
			t.Cleanup(screen.Fini)
			const width, height = 60, 10
			screen.SetSize(width, height)

			styles := theme.Default()
			root := "/vol"
			state := panel.State{
				Path: pathloc.MustParse(root),
				Entries: []localfs.Entry{
					{Name: "cursor.txt", Path: root + "/cursor.txt", Type: localfs.EntryFile},
					{Name: "other.txt", Path: root + "/other.txt", Type: localfs.EntryFile},
				},
				Cursor: 0,
			}
			rect := Rect{X: 0, Y: 0, Width: width, Height: height}
			drawPanel(screen, rect, state,
				PanelStyleConfig{Styles: styles},
				PanelContext{
					PanelID: PrimaryPanel, FileListActive: true, CursorRowActive: true, ActivePanel: PrimaryPanel,
					SyncDriverPanelID: -1, QuickViewDriverPanelID: -1,
					QuickViewIndicator: true, QuickViewIndicatorRight: tc.right,
				},
				PanelDisplayConfig{ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout()})

			cursorRowY := rect.Y + 2
			otherRowY := rect.Y + 3

			ch, gotStyle, _ := screen.Get(tc.wantColX(rect), cursorRowY)
			r, _ := utf8.DecodeRuneInString(ch)
			if r != tc.wantIcon {
				t.Fatalf("cursor row border icon = %q, want %q", r, tc.wantIcon)
			}

			cursorState, _ := panelRowStyle(state.Entries[0], 0, state,
				PanelContext{ChromeBlocked: false, CursorRowActive: true}, styles)
			_, wantBG, _ := cursorState.Decompose()
			gotFG, _, _ := gotStyle.Decompose()
			if gotFG != wantBG {
				t.Fatalf("icon foreground = %v, want cursor row background %v", gotFG, wantBG)
			}

			if ch2, _, _ := screen.Get(tc.unwantColX(rect), cursorRowY); func() bool {
				r2, _ := utf8.DecodeRuneInString(ch2)
				return r2 == tc.wantIcon
			}() {
				t.Fatalf("icon unexpectedly found on the non-facing border")
			}

			if ch3, _, _ := screen.Get(tc.wantColX(rect), otherRowY); func() bool {
				r3, _ := utf8.DecodeRuneInString(ch3)
				return r3 == tc.wantIcon
			}() {
				t.Fatalf("icon unexpectedly found on a non-cursor row")
			}
		})
	}
}

package ui

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// carouselPrefetchMarkPainted renders a carousel panel over root with the given display config
// and reports whether the prefetch loading glyph appears anywhere in the panel.
func carouselPrefetchMarkPainted(t *testing.T, root string, loading map[string]struct{}) bool {
	t.Helper()
	state, err := panel.New(root)
	if err != nil {
		t.Fatal(err)
	}
	state.CarouselMode = true

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	const width, height = 100, 20
	screen.SetSize(width, height)
	primitive.Fill(screen, primitive.Rect{Width: width, Height: height}, ' ', tcell.StyleDefault)

	rect := Rect{X: 0, Y: 1, Width: width, Height: height - 3}
	styles := theme.Default()
	drawPanel(screen, rect, state,
		PanelStyleConfig{Styles: styles},
		PanelContext{PanelID: PrimaryPanel, FileListActive: true, ActivePanel: PrimaryPanel, SyncDriverPanelID: -1, QuickViewDriverPanelID: -1},
		PanelDisplayConfig{ShowIcons: true, CarouselLayout: panelcarousel.DefaultLayout(), PreviewPrefetchLoading: loading})

	want := styles.SymbolFilelistPreviewLoading()
	for y := rect.Y; y < rect.Y+rect.Height; y++ {
		for x := rect.X; x < rect.X+rect.Width; x++ {
			ch, _, _ := screen.Get(x, y)
			if r, _ := utf8.DecodeRuneInString(ch); r == want {
				return true
			}
		}
	}
	return false
}

// Carousel column rows must carry the same preview-prefetch marks as the plain file list —
// both row painters build their icon-strip context through panelIconStripContextFor.
func TestCarouselRowPaintsPrefetchLoadingMark(t *testing.T) {
	root := t.TempDir()
	image := filepath.Join(root, "meadow.png")
	if err := os.WriteFile(image, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "thicket"), 0o755); err != nil {
		t.Fatal(err)
	}

	if carouselPrefetchMarkPainted(t, root, nil) {
		t.Fatal("prefetch loading glyph painted with no in-flight prefetch")
	}
	if !carouselPrefetchMarkPainted(t, root, map[string]struct{}{image: {}}) {
		t.Fatal("prefetch loading glyph missing on carousel row for in-flight prefetch")
	}
}

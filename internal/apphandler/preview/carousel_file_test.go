package preview

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// Regression: with a fit-to-content split ("<33%") whose parent/center columns hold only short
// names, the carousel file preview's text width must reflect the actual measured (narrow) column
// widths, not the unmeasured 33%-cap worst case — otherwise the preview pre-wraps (markdown in
// particular) far narrower than the column it's actually painted into.
func TestCarouselChildPreviewLayoutMetricsUsesMeasuredFitWidth(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "work")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "doc.txt"), []byte("body\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	h, _ := newTestHandler(t, 300, 30)
	h.model.Primary = panel.State{Path: pathloc.MustParse(work)}
	if err := h.model.Primary.Load(work); err != nil {
		t.Fatal(err)
	}
	h.model.HideInactivePanel = true
	h.model.Primary.CarouselMode = true
	if !h.model.Primary.SelectVisibleEntry("doc.txt") {
		t.Fatal("doc.txt not found")
	}

	layout, err := panelcarousel.ParseLayout([]string{"<33%", "<33%", "*"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	h.model.CarouselLayout = layout

	rect, ok := h.activePanelFileColumnRect()
	if !ok {
		t.Fatal("activePanelFileColumnRect: not ok")
	}
	worstCase := panelcarousel.ChildColumnWidth(rect, layout)

	tw, _, ok := h.carouselChildPreviewLayoutMetrics()
	if !ok {
		t.Fatal("carouselChildPreviewLayoutMetrics: not ok")
	}
	if tw+2 <= worstCase {
		t.Fatalf("measured child text width+2 = %d, want > unmeasured worst-case width %d "+
			"(short parent/center names should measure well under the 33%% cap)", tw+2, worstCase)
	}
}

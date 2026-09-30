package preview

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// A directory under the carousel cursor that a [[preview.commands]] rule matches opens the child
// preview as a directory preview; once every matching rule declines, the preview closes and
// CarouselDirRule reports false so the child listing shows instead.
func TestCarouselDirRuleOpensAndDeclines(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "lantern"), 0o755); err != nil {
		t.Fatal(err)
	}
	h, fh := newTestHandler(t, 300, 30)
	fh.cfg.Preview.Commands = []config.PreviewCommandRule{
		{When: []string{"t d"}, Command: "sh -c 'exit 1'"},
	}
	h.model.Primary = panel.State{Path: pathloc.MustParse(root)}
	if err := h.model.Primary.Load(root); err != nil {
		t.Fatal(err)
	}
	h.model.HideInactivePanel = true
	h.model.Primary.CarouselMode = true
	h.model.Primary.PreviewDirRule = h.CarouselDirRule
	if !h.model.Primary.SelectVisibleEntry("lantern") {
		t.Fatal("lantern not found")
	}
	layout, err := panelcarousel.ParseLayout([]string{"<33%", "<33%", "*"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	h.model.CarouselLayout = layout

	h.applyCarouselFilePreviewNow()
	h.mu.RLock()
	st := h.model.CarouselFilePreview
	h.mu.RUnlock()
	if !st.Open || !st.IsDir {
		t.Fatalf("CarouselFilePreview Open=%v IsDir=%v, want a directory preview", st.Open, st.IsDir)
	}

	var payload DirRuleDeclinedPayload
	for {
		interrupt, ok := h.screen.PollEvent().(*tcell.EventInterrupt)
		if !ok {
			continue
		}
		if p, ok := interrupt.Data().(DirRuleDeclinedPayload); ok {
			payload = p
			break
		}
	}
	if !h.ApplyDirRuleDeclined(payload) {
		t.Fatal("ApplyDirRuleDeclined = false, want true")
	}
	h.mu.RLock()
	open := h.model.CarouselFilePreview.Open
	h.mu.RUnlock()
	if open {
		t.Fatal("carousel preview still open after decline")
	}
	if h.CarouselDirRule(filepath.Join(root, "lantern")) {
		t.Fatal("CarouselDirRule = true for a declined directory")
	}
}

package preview

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/keymap"
)

func TestFullscreenPreviewVimScrollActions(t *testing.T) {
	words := []string{"amber", "breeze", "candle", "dolphin", "ember", "forest", "garden", "harbor"}
	var b strings.Builder
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&b, "%s %s\n", words[i%len(words)], words[(i*3)%len(words)])
	}
	path := filepath.Join(t.TempDir(), "meadow.txt")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	h, _ := newTestHandler(t, 80, 24)
	openFullscreenPreviewSync(t, h, path)

	_, ch, lc := h.fullscreenFilePreviewScrollMetrics()
	maxStart := lc - ch
	if maxStart < 10 {
		t.Fatalf("test file too short: lc=%d ch=%d", lc, ch)
	}
	half := max(1, ch/2)
	scroll := func() int { return h.model.FullscreenFilePreview.Scroll }
	do := func(action string, want int) {
		t.Helper()
		if _, handled := h.tryFilePreviewAction(action); !handled {
			t.Fatalf("%s not handled", action)
		}
		if scroll() != want {
			t.Fatalf("%s: Scroll = %d, want %d", action, scroll(), want)
		}
	}

	do(keymap.ActionPreviewScrollUp, 0) // clamp at top
	do(keymap.ActionPreviewScrollDown, 1)
	do(keymap.ActionPreviewScrollDown, 2)
	do(keymap.ActionPreviewScrollUp, 1)
	do(keymap.ActionPreviewHalfPageDown, 1+half)
	do(keymap.ActionPreviewHalfPageUp, 1)
	do(keymap.ActionPreviewPageDown, 1+ch)
	do(keymap.ActionPreviewPageUp, 1)
	do(keymap.ActionPreviewBottom, maxStart)
	do(keymap.ActionPreviewScrollDown, maxStart) // clamp at bottom
	do(keymap.ActionPreviewHalfPageDown, maxStart)
	do(keymap.ActionPreviewPageDown, maxStart)
	do(keymap.ActionPreviewTop, 0)
	do(keymap.ActionPreviewHalfPageUp, 0)
	do(keymap.ActionPreviewPageUp, 0)
}

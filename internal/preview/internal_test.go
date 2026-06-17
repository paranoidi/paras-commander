package preview_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/preview"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func TestRunInternalHighlightsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := preview.Run(t.Context(), preview.Request{
		Path:      path,
		TextWidth: 40,
		Preview: config.PreviewConfig{
			Mode:        config.PreviewModeInternal,
			Style:       config.DefaultPreviewStyle,
			LineNumbers: true,
		},
		BaseStyle: tcell.StyleDefault,
	})
	if res.ErrorMsg != "" {
		t.Fatalf("ErrorMsg = %q", res.ErrorMsg)
	}
	if len(res.HighlightedCells) == 0 {
		t.Fatal("expected highlighted cells")
	}
	if !strings.Contains(cellsPrefix(res.HighlightedCells), "1") {
		t.Fatalf("line numbers missing in %q", cellsPrefix(res.HighlightedCells))
	}
}

func TestRunInternalTruncatesLargeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bulk.txt")
	data := strings.Repeat("x", config.DefaultMaxPreviewBytes+10)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	res := preview.Run(t.Context(), preview.Request{
		Path:      path,
		TextWidth: 40,
		Preview: config.PreviewConfig{
			Mode:  config.PreviewModeInternal,
			Style: config.DefaultPreviewStyle,
		},
		BaseStyle: tcell.StyleDefault,
	})
	if !res.Truncated {
		t.Fatal("expected truncated result")
	}
	flat := cellsString(res.HighlightedCells)
	if !strings.Contains(flat, "[output truncated]") {
		t.Fatal("expected truncation marker in highlighted cells")
	}
}

func cellsPrefix(cells []previewpanel.AnsiCell) string {
	var b strings.Builder
	for _, c := range cells {
		if c.R == '\n' {
			break
		}
		b.WriteRune(c.R)
	}
	return b.String()
}

func cellsString(cells []previewpanel.AnsiCell) string {
	var b strings.Builder
	for _, c := range cells {
		b.WriteRune(c.R)
	}
	return b.String()
}

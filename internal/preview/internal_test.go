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

func TestRunMarkdownRendersHeadingInsteadOfRawTokens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "meadow.md")
	if err := os.WriteFile(path, []byte("# Wandering Otters\n\nA calm river story.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := preview.Run(t.Context(), preview.Request{
		Path:      path,
		TextWidth: 60,
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
	if res.GutterWidth != 0 {
		t.Errorf("GutterWidth = %d, want 0 (no line-number gutter for markdown even with LineNumbers=true)", res.GutterWidth)
	}
	flat := cellsString(res.HighlightedCells)
	if !strings.Contains(flat, "# Wandering Otters") {
		t.Fatalf("expected rendered heading with # prefix, got %q", flat)
	}
}

func TestRunMarkdownExtensionCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.MARKDOWN")
	if err := os.WriteFile(path, []byte("plain paragraph text\n"), 0o644); err != nil {
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
	if res.GutterWidth != 0 {
		t.Errorf("GutterWidth = %d, want 0 for .MARKDOWN file routed through runMarkdown", res.GutterWidth)
	}
}

func TestRunRawMarkdownRoutesToInternalHighlighting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "meadow.md")
	if err := os.WriteFile(path, []byte("# Wandering Otters\n\nA calm river story.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	req := preview.Request{
		Path:      path,
		TextWidth: 60,
		Preview: config.PreviewConfig{
			Mode:        config.PreviewModeInternal,
			Style:       config.DefaultPreviewStyle,
			LineNumbers: true,
		},
		BaseStyle: tcell.StyleDefault,
	}

	rendered := preview.Run(t.Context(), req)
	if rendered.GutterWidth != 0 {
		t.Fatalf("rendered GutterWidth = %d, want 0 (markdown never gets a gutter)", rendered.GutterWidth)
	}

	req.RawMarkdown = true
	raw := preview.Run(t.Context(), req)
	if raw.ErrorMsg != "" {
		t.Fatalf("ErrorMsg = %q", raw.ErrorMsg)
	}
	if raw.GutterWidth == 0 {
		t.Fatal("raw GutterWidth = 0, want >0 (raw path runs runInternal with line_numbers on)")
	}
	rawFlat := cellsString(raw.HighlightedCells)
	if !strings.Contains(rawFlat, "# Wandering Otters") {
		t.Fatalf("expected raw markdown source with # prefix, got %q", rawFlat)
	}
	if cellsString(rendered.HighlightedCells) == rawFlat {
		t.Fatal("rendered and raw cell strings must differ (raw keeps the gutter prefix)")
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

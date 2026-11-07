package preview_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/preview"
)

func writeLantern(t *testing.T, name string, bytes int) string {
	t.Helper()
	var b strings.Builder
	for i := 0; b.Len() < bytes; i++ {
		fmt.Fprintf(&b, "var lantern%d = \"meadow copper willow\" // harbor\n", i)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func partialRequest(path string, onPartial func(preview.Result)) preview.Request {
	return preview.Request{
		Path: path, TextWidth: 80, ViewportRows: 24, BaseStyle: tcell.StyleDefault,
		Preview: config.Default().Preview, OnPartial: onPartial,
	}
}

func TestRunLargeFileReportsPartialWithSameCellCount(t *testing.T) {
	var partials []preview.Result
	path := writeLantern(t, "harbor.go", config.DefaultMaxPreviewBytes+1024)
	res := preview.Run(t.Context(), partialRequest(path, func(r preview.Result) { partials = append(partials, r) }))
	if len(partials) != 1 {
		t.Fatalf("partials = %d, want 1", len(partials))
	}
	if !res.Truncated || !partials[0].Truncated {
		t.Fatal("expected a truncated body in both results")
	}
	if len(partials[0].HighlightedCells) != len(res.HighlightedCells) {
		t.Fatalf("cells = %d, want %d", len(partials[0].HighlightedCells), len(res.HighlightedCells))
	}
}

func TestRunGitDiffFallbackPartialCarriesStatus(t *testing.T) {
	var partial preview.Result
	req := partialRequest(writeLantern(t, "harbor.go", 64*1024), func(r preview.Result) { partial = r })
	req.GitDiff = true
	req.GitStatus = &gitstatus.Cell{Unstaged: gitstatus.New}
	res := preview.Run(t.Context(), req)
	if res.StatusText == "" || partial.StatusText != res.StatusText {
		t.Fatalf("partial status = %q, final = %q", partial.StatusText, res.StatusText)
	}
}

func TestRunReturnsCanceledResult(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, name := range []string{"harbor.go", "harbor.md"} {
		res := preview.Run(ctx, partialRequest(writeLantern(t, name, 64*1024), nil))
		if res.ErrorMsg != "Canceled" {
			t.Fatalf("%s: ErrorMsg = %q", name, res.ErrorMsg)
		}
	}
}

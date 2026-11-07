package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestFilePreviewWrapCacheCountsRows(t *testing.T) {
	base := tcell.StyleDefault.Foreground(tcell.ColorWhite)
	text := strings.Repeat("word ", 200)
	st := FilePreviewState{
		Open:         true,
		Phase:        FilePreviewPhaseDone,
		CombinedText: text,
	}
	first := st.WrappedRows(20, base)
	if count, ok := st.CachedWrappedLineCount(20); !ok || count != len(first) {
		t.Fatalf("cache miss after wrap: ok=%v count=%d want %d", ok, count, len(first))
	}
}

func TestFilePreviewWrapCacheInvalidatesOnTextChange(t *testing.T) {
	base := tcell.StyleDefault
	st := FilePreviewState{
		Open:         true,
		Phase:        FilePreviewPhaseDone,
		CombinedText: "alpha beta",
	}
	st.EnsureWrapLayout(10, base)
	st.CombinedText = "gamma delta"
	lines := st.WrappedRows(10, base)
	if len(lines) == 0 {
		t.Fatal("expected wrapped lines after text change")
	}
	if count, ok := st.CachedWrappedLineCount(10); !ok || count != len(lines) {
		t.Fatalf("cache after text change: ok=%v count=%d want %d", ok, count, len(lines))
	}
}

func BenchmarkFilePreviewWrapCacheScroll(b *testing.B) {
	base := tcell.StyleDefault
	var sb strings.Builder
	for i := 0; i < 5000; i++ {
		sb.WriteString("echo line ")
		sb.WriteString(strings.Repeat("x", 40))
		sb.WriteByte('\n')
	}
	st := FilePreviewState{
		Open:         true,
		Phase:        FilePreviewPhaseDone,
		CombinedText: sb.String(),
	}
	if rows := st.WrappedLineCount(80, base); rows < 100 {
		b.Fatalf("expected many wrapped lines, got %d", rows)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = st.WrappedLineCount(80, base)
	}
}

func BenchmarkFilePreviewWrapCacheColdParse(b *testing.B) {
	base := tcell.StyleDefault
	var sb strings.Builder
	for i := 0; i < 5000; i++ {
		sb.WriteString("echo line ")
		sb.WriteString(strings.Repeat("x", 40))
		sb.WriteByte('\n')
	}
	text := sb.String()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st := FilePreviewState{
			Open:         true,
			Phase:        FilePreviewPhaseDone,
			CombinedText: text,
		}
		st.EnsureWrapLayout(80, base)
	}
}

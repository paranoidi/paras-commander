package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestFilePreviewWrapCacheReusesLines(t *testing.T) {
	base := tcell.StyleDefault.Foreground(tcell.ColorWhite)
	text := strings.Repeat("word ", 200)
	st := FilePreviewState{
		Open:         true,
		Phase:        FilePreviewPhaseDone,
		CombinedText: text,
	}
	first := st.EnsureWrappedLines(20, base)
	second := st.EnsureWrappedLines(20, base)
	if &first[0][0] != &second[0][0] {
		t.Fatal("expected cached wrapped lines slice to be reused")
	}
	if st.wrapCombinedText != text {
		t.Fatalf("wrapCombinedText = %q, want cached source text", st.wrapCombinedText)
	}
}

func TestFilePreviewWrapCacheInvalidatesOnTextChange(t *testing.T) {
	base := tcell.StyleDefault
	st := FilePreviewState{
		Open:         true,
		Phase:        FilePreviewPhaseDone,
		CombinedText: "alpha beta",
	}
	_ = st.EnsureWrappedLines(10, base)
	st.CombinedText = "gamma delta"
	lines := st.EnsureWrappedLines(10, base)
	if len(lines) == 0 {
		t.Fatal("expected wrapped lines after text change")
	}
	if st.wrapCombinedText != "gamma delta" {
		t.Fatalf("wrapCombinedText = %q, want updated text", st.wrapCombinedText)
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
	lines := st.EnsureWrappedLines(80, base)
	if len(lines) < 100 {
		b.Fatalf("expected many wrapped lines, got %d", len(lines))
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
		_ = st.EnsureWrappedLines(80, base)
	}
}

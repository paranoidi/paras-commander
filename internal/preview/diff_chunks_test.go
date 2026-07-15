package preview

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseDiffChangeChunkLines(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/meadow.md b/meadow.md",
		"--- a/meadow.md",
		"+++ b/meadow.md",
		"@@ -1,10 +1,10 @@",
		" line-01",
		" line-02",
		"-old-middle-a",
		"+new-middle-a",
		" line-04",
		" line-05",
		"-old-middle-b1",
		"-old-middle-b2",
		"+new-middle-b",
		" line-09",
		" line-10",
	}, "\n")
	got := parseDiffChangeChunkLines(diff)
	// first change at -old-middle-a; second at -old-middle-b1 (contiguous +/- run)
	want := []int{6, 10}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDiffChangeChunkLines = %v, want %v", got, want)
	}
}

func TestParseDiffChangeChunkLines_noChanges(t *testing.T) {
	diff := strings.Join([]string{
		"--- a/x",
		"+++ b/x",
		"@@ -1,3 +1,3 @@",
		" a",
		" b",
		" c",
	}, "\n")
	got := parseDiffChangeChunkLines(diff)
	if len(got) != 0 {
		t.Fatalf("got %v, want no chunks when only context lines", got)
	}
}

func TestIsUnifiedDiffChangeLine_allowsTripleDashContent(t *testing.T) {
	// Deleted line whose content is "--" becomes "---" without a trailing space —
	// must still count as a change, unlike the "--- a/file" header.
	if !isUnifiedDiffChangeLine("---") {
		t.Fatal(`deleted content "--" should count as a change line`)
	}
	if isUnifiedDiffChangeLine("--- a/meadow.md") {
		t.Fatal("file header must not count as a change line")
	}
	if !isUnifiedDiffChangeLine("+++") {
		t.Fatal(`added content "++" should count as a change line`)
	}
	if isUnifiedDiffChangeLine("+++ b/meadow.md") {
		t.Fatal("file header must not count as a change line")
	}
}

func TestStripANSI_andParseChangeChunks(t *testing.T) {
	in := "\x1b[31m-removed\x1b[m\n\x1b[32m+added\x1b[m"
	got := stripANSI(in)
	want := "-removed\n+added"
	if got != want {
		t.Fatalf("stripANSI = %q, want %q", got, want)
	}
	chunks := parseDiffChangeChunkLines(got)
	if !reflect.DeepEqual(chunks, []int{0}) {
		t.Fatalf("chunks = %v, want [0] (contiguous +/- is one chunk)", chunks)
	}
}

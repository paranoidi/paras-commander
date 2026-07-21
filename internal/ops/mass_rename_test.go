package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/search"
)

func TestMassRenameComputeSimple(t *testing.T) {
	dir := t.TempDir()
	entries := []localfs.Entry{
		{Name: "foo_bar.txt", Path: filepath.Join(dir, "foo_bar.txt"), Type: localfs.EntryFile},
		{Name: "foo_baz.txt", Path: filepath.Join(dir, "foo_baz.txt"), Type: localfs.EntryFile},
	}
	rows, err := MassRenameCompute(entries, dir, MassRenameModeSimple, "foo_", "x_", false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("len = %d", len(rows))
	}
	if rows[0].NewBase != "x_bar.txt" || rows[1].NewBase != "x_baz.txt" {
		t.Fatalf("got %#v", rows)
	}
}

func TestMassRenameComputeSimpleCaseFold(t *testing.T) {
	dir := t.TempDir()
	entries := []localfs.Entry{
		{Name: "AbC.txt", Path: filepath.Join(dir, "AbC.txt"), Type: localfs.EntryFile},
	}
	rows, err := MassRenameCompute(entries, dir, MassRenameModeSimple, "bc", "xx", true, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].NewBase != "Axx.txt" {
		t.Fatalf("got %q", rows[0].NewBase)
	}
}

func TestMassRenameComputeStripSpaces(t *testing.T) {
	dir := t.TempDir()
	entries := []localfs.Entry{
		{Name: "  alpha  ", Path: filepath.Join(dir, "  alpha  "), Type: localfs.EntryFile},
	}
	rows, err := MassRenameCompute(entries, dir, MassRenameModeSimple, "", "", false, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].NewBase != "alpha" {
		t.Fatalf("strip on: got %q, want alpha", rows[0].NewBase)
	}
	rows, err = MassRenameCompute(entries, dir, MassRenameModeSimple, "", "", false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].NewBase != "  alpha  " {
		t.Fatalf("strip off: got %q, want padded name", rows[0].NewBase)
	}
}

func TestMassRenameComputeRegexCaseFold(t *testing.T) {
	dir := t.TempDir()
	re, err := MassRenameCompileRegex(`hello`, true)
	if err != nil {
		t.Fatal(err)
	}
	entries := []localfs.Entry{
		{Name: "HelloWorld.txt", Path: filepath.Join(dir, "HelloWorld.txt"), Type: localfs.EntryFile},
	}
	rows, err := MassRenameCompute(entries, dir, MassRenameModeRegex, "", "X", false, false, re)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].NewBase != "XWorld.txt" {
		t.Fatalf("got %q, want XWorld.txt", rows[0].NewBase)
	}
	reSens, err := MassRenameCompileRegex(`hello`, false)
	if err != nil {
		t.Fatal(err)
	}
	rows, err = MassRenameCompute(entries, dir, MassRenameModeRegex, "", "X", false, false, reSens)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].NewBase != "HelloWorld.txt" {
		t.Fatalf("case-sensitive: got %q, want unchanged", rows[0].NewBase)
	}
}

func TestMassRenameComputeRegex(t *testing.T) {
	dir := t.TempDir()
	re, err := MassRenameCompileRegex(`^(.+)_(.+)\.txt$`, false)
	if err != nil {
		t.Fatal(err)
	}
	entries := []localfs.Entry{
		{Name: "aa_bb.txt", Path: filepath.Join(dir, "aa_bb.txt"), Type: localfs.EntryFile},
	}
	rows, err := MassRenameCompute(entries, dir, MassRenameModeRegex, "", `${2}_${1}.txt`, false, false, re)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].NewBase != "bb_aa.txt" {
		t.Fatalf("got %q", rows[0].NewBase)
	}
}

func TestMassRenameComputeRegexSeasonDigitPad(t *testing.T) {
	dir := t.TempDir()
	re, err := MassRenameCompileRegex(`(\d)$`, false)
	if err != nil {
		t.Fatal(err)
	}
	entries := []localfs.Entry{
		{Name: "Season 1", Path: filepath.Join(dir, "Season 1"), Type: localfs.EntryFile},
	}
	for _, repl := range []string{"0${1}", "0$1", "0${0}"} {
		rows, err := MassRenameCompute(entries, dir, MassRenameModeRegex, "", repl, false, false, re)
		if err != nil {
			t.Fatal(err)
		}
		if rows[0].NewBase != "Season 01" {
			t.Fatalf("replace %q: got %q, want Season 01", repl, rows[0].NewBase)
		}
	}
}

func TestMassRenameComputeRegexAmbiguousDollarGroup(t *testing.T) {
	dir := t.TempDir()
	re, err := MassRenameCompileRegex(`(\d)$`, false)
	if err != nil {
		t.Fatal(err)
	}
	entries := []localfs.Entry{
		{Name: "Season 1", Path: filepath.Join(dir, "Season 1"), Type: localfs.EntryFile},
	}
	rows, err := MassRenameCompute(entries, dir, MassRenameModeRegex, "", "0$10", false, false, re)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].NewBase != "Season 0" {
		t.Fatalf("0$10: got %q, want Season 0 (group 10 empty)", rows[0].NewBase)
	}
	rows, err = MassRenameCompute(entries, dir, MassRenameModeRegex, "", "0${1}0", false, false, re)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].NewBase != "Season 010" {
		t.Fatalf("0${1}0: got %q, want Season 010", rows[0].NewBase)
	}
}

func TestMassRenameComputeRegexBackslashGroup(t *testing.T) {
	dir := t.TempDir()
	re, err := MassRenameCompileRegex(`(\d)$`, false)
	if err != nil {
		t.Fatal(err)
	}
	entries := []localfs.Entry{
		{Name: "Season 1", Path: filepath.Join(dir, "Season 1"), Type: localfs.EntryFile},
	}
	rows, err := MassRenameCompute(entries, dir, MassRenameModeRegex, "", `0\1`, false, false, re)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].NewBase != "Season 01" {
		t.Fatalf("got %q, want Season 01", rows[0].NewBase)
	}
}

func TestMassRenameReplacementSyntaxHint(t *testing.T) {
	rx, err := MassRenameCompileRegex(`(\d)`, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := MassRenameReplacementSyntaxHint(rx); got == "" {
		t.Fatal("expected hint for capture group pattern")
	}
	rxPlain, err := MassRenameCompileRegex(`\.txt$`, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := MassRenameReplacementSyntaxHint(rxPlain); got != "" {
		t.Fatalf("expected no hint for pattern without groups, got %q", got)
	}
	if got := MassRenameReplacementSyntaxHint(nil); got != "" {
		t.Fatalf("expected no hint for nil rx, got %q", got)
	}
}

func TestMassRenameValidateRowsDuplicate(t *testing.T) {
	dir := t.TempDir()
	rows := []MassRenameRow{
		{SourcePath: filepath.Join(dir, "a.txt"), OldBase: "a.txt", NewBase: "z.txt"},
		{SourcePath: filepath.Join(dir, "b.txt"), OldBase: "b.txt", NewBase: "z.txt"},
	}
	if err := MassRenameValidateRows(rows); err == nil {
		t.Fatal("expected error")
	}
}

func TestMassRenameExecuteSwap(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(a, []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("2"), 0o644); err != nil {
		t.Fatal(err)
	}
	rows := []MassRenameRow{
		{SourcePath: a, OldBase: "a.txt", NewBase: "b.txt"},
		{SourcePath: b, OldBase: "b.txt", NewBase: "a.txt"},
	}
	if err := MassRenameValidateRows(rows); err != nil {
		t.Fatal(err)
	}
	if err := ExecuteMassRename(rows); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.txt")); err != nil {
		t.Fatalf("a.txt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.txt")); err != nil {
		t.Fatalf("b.txt: %v", err)
	}
	b1, _ := os.ReadFile(filepath.Join(dir, "a.txt"))
	b2, _ := os.ReadFile(filepath.Join(dir, "b.txt"))
	if string(b1) != "2" || string(b2) != "1" {
		t.Fatalf("content swap failed: %s %s", b1, b2)
	}
}

func TestMassRenameExecuteChain(t *testing.T) {
	dir := t.TempDir()
	paths := []string{"a.txt", "b.txt", "c.txt"}
	for _, n := range paths {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte(n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rows := []MassRenameRow{
		{SourcePath: filepath.Join(dir, "a.txt"), OldBase: "a.txt", NewBase: "b.txt"},
		{SourcePath: filepath.Join(dir, "b.txt"), OldBase: "b.txt", NewBase: "c.txt"},
		{SourcePath: filepath.Join(dir, "c.txt"), OldBase: "c.txt", NewBase: "a.txt"},
	}
	if err := MassRenameValidateRows(rows); err != nil {
		t.Fatal(err)
	}
	if err := ExecuteMassRename(rows); err != nil {
		t.Fatal(err)
	}
	for _, n := range paths {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Fatalf("missing %s: %v", n, err)
		}
	}
}

func TestMassRenameValidateExternalCollision(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "taken.txt")
	if err := os.WriteFile(existing, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(src, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	rows := []MassRenameRow{
		{SourcePath: src, OldBase: "a.txt", NewBase: "taken.txt"},
	}
	if err := MassRenameValidateRows(rows); err == nil {
		t.Fatal("expected collision error")
	}
}

func TestMassRenameRowErrorsMarksConflictingRows(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "Season2"), 0o755); err != nil {
		t.Fatal(err)
	}
	rows := []MassRenameRow{
		{SourcePath: filepath.Join(dir, "Season1"), OldBase: "Season1", NewBase: "Season2"},
		{SourcePath: filepath.Join(dir, "Season3"), OldBase: "Season3", NewBase: "Season3"},
	}
	errs := MassRenameRowErrors(rows)
	if errs[0] == nil {
		t.Fatal("expected Season1 row error")
	}
	if errs[1] != nil {
		t.Fatalf("Season3 should be valid, got %v", errs[1])
	}
}

func TestMassRenameRowErrorsDuplicateTargetsBothRows(t *testing.T) {
	dir := t.TempDir()
	rows := []MassRenameRow{
		{SourcePath: filepath.Join(dir, "a.txt"), OldBase: "a.txt", NewBase: "z.txt"},
		{SourcePath: filepath.Join(dir, "b.txt"), OldBase: "b.txt", NewBase: "z.txt"},
	}
	errs := MassRenameRowErrors(rows)
	if errs[0] == nil || errs[1] == nil {
		t.Fatalf("expected both rows invalid, got %#v", errs)
	}
}

func TestMassRenameRowErrorsAllowsMixedDirectories(t *testing.T) {
	root := t.TempDir()
	dirA := filepath.Join(root, "dirA")
	dirB := filepath.Join(root, "dirB")
	if err := os.Mkdir(dirA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dirB, 0o755); err != nil {
		t.Fatal(err)
	}
	rows := []MassRenameRow{
		{SourcePath: filepath.Join(dirA, "foo.txt"), OldBase: "foo.txt", NewBase: "renamed.txt"},
		{SourcePath: filepath.Join(dirB, "bar.txt"), OldBase: "bar.txt", NewBase: "other.txt"},
	}
	errs := MassRenameRowErrors(rows)
	for i, err := range errs {
		if err != nil {
			t.Fatalf("row %d: unexpected error %v", i, err)
		}
	}
}

func TestMassRenameRowErrorsDuplicateScopedPerDirectory(t *testing.T) {
	root := t.TempDir()
	dirA := filepath.Join(root, "dirA")
	dirB := filepath.Join(root, "dirB")
	if err := os.Mkdir(dirA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dirB, 0o755); err != nil {
		t.Fatal(err)
	}
	rows := []MassRenameRow{
		{SourcePath: filepath.Join(dirA, "a.txt"), OldBase: "a.txt", NewBase: "same.txt"},
		{SourcePath: filepath.Join(dirB, "b.txt"), OldBase: "b.txt", NewBase: "same.txt"},
	}
	errs := MassRenameRowErrors(rows)
	if errs[0] != nil || errs[1] != nil {
		t.Fatalf("same target name in different directories should not conflict, got %#v", errs)
	}
}

func TestMassRenameExecuteMultiDirectory(t *testing.T) {
	root := t.TempDir()
	dirA := filepath.Join(root, "dirA")
	dirB := filepath.Join(root, "dirB")
	if err := os.Mkdir(dirA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dirB, 0o755); err != nil {
		t.Fatal(err)
	}
	srcA := filepath.Join(dirA, "foo.txt")
	srcB := filepath.Join(dirB, "bar.txt")
	if err := os.WriteFile(srcA, []byte("A"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcB, []byte("B"), 0o644); err != nil {
		t.Fatal(err)
	}
	rows := []MassRenameRow{
		{SourcePath: srcA, OldBase: "foo.txt", NewBase: "renamed.txt"},
		{SourcePath: srcB, OldBase: "bar.txt", NewBase: "renamed.txt"},
	}
	if err := MassRenameValidateRows(rows); err != nil {
		t.Fatal(err)
	}
	if err := ExecuteMassRename(rows); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dirA, "renamed.txt")); err != nil || string(b) != "A" {
		t.Fatalf("dirA/renamed.txt: content %q, err %v", b, err)
	}
	if b, err := os.ReadFile(filepath.Join(dirB, "renamed.txt")); err != nil || string(b) != "B" {
		t.Fatalf("dirB/renamed.txt: content %q, err %v", b, err)
	}
}

func TestMassRenameRegexCompileUserMessage(t *testing.T) {
	_, err := MassRenameCompileRegex("a++", false)
	if err == nil {
		t.Fatal("expected compile error")
	}
	got := MassRenameRegexCompileUserMessage(err)
	if got == "" {
		t.Fatal("empty message")
	}
	if strings.Contains(got, "mass-rename") {
		t.Fatalf("should not include op wrapper: %q", got)
	}
	if strings.Contains(got, "error parsing regexp:") {
		t.Fatalf("should strip parse prefix: %q", got)
	}
	if !strings.Contains(got, "nested repetition") {
		t.Fatalf("unexpected detail: %q", got)
	}
}

func TestMassRenameRegexCompileUserMessageBackslash(t *testing.T) {
	_, err := MassRenameCompileRegex(`\`, false)
	if err == nil {
		t.Fatal("expected compile error")
	}
	got := MassRenameRegexCompileUserMessage(err)
	if got == "" {
		t.Fatal("empty message for trailing backslash pattern")
	}
}

func TestMassRenameCompileRegexEmpty(t *testing.T) {
	if _, err := MassRenameCompileRegex("   ", false); err == nil {
		t.Fatal("expected error")
	}
}

func TestMassRenameComputeSimpleEmptyFindIsIdentity(t *testing.T) {
	dir := t.TempDir()
	entries := []localfs.Entry{
		{Name: "x.txt", Path: filepath.Join(dir, "x.txt"), Type: localfs.EntryFile},
	}
	rows, err := MassRenameCompute(entries, dir, MassRenameModeSimple, "", "y", false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].OldBase != "x.txt" || rows[0].NewBase != "x.txt" {
		t.Fatalf("got %#v", rows)
	}
}

func TestMassRenameFindMatchesAnySimple(t *testing.T) {
	rows := []MassRenameRow{
		{OldBase: "foo_a.txt", NewBase: "foo_a.txt"},
		{OldBase: "bar_b.txt", NewBase: "bar_b.txt"},
	}
	if !MassRenameFindMatchesAny(rows, MassRenameModeSimple, "foo", false, nil) {
		t.Fatal("expected match for foo")
	}
	if MassRenameFindMatchesAny(rows, MassRenameModeSimple, "zzz", false, nil) {
		t.Fatal("expected no match for zzz")
	}
	if !MassRenameFindMatchesAny(rows, MassRenameModeSimple, "", false, nil) {
		t.Fatal("empty find should match")
	}
}

func TestMassRenameFindMatchesAnySimpleCaseFold(t *testing.T) {
	rows := []MassRenameRow{{OldBase: "Alpha.txt", NewBase: "Alpha.txt"}}
	if !MassRenameFindMatchesAny(rows, MassRenameModeSimple, "alpha", true, nil) {
		t.Fatal("expected case-fold match")
	}
	if MassRenameFindMatchesAny(rows, MassRenameModeSimple, "alpha", false, nil) {
		t.Fatal("expected no case-sensitive match")
	}
}

func TestMassRenameFindMatchesAnyRegex(t *testing.T) {
	re, err := MassRenameCompileRegex(`\.txt$`, false)
	if err != nil {
		t.Fatal(err)
	}
	rows := []MassRenameRow{{OldBase: "a.txt", NewBase: "a.txt"}}
	if !MassRenameFindMatchesAny(rows, MassRenameModeRegex, "", false, re) {
		t.Fatal("expected regex match")
	}
	if !MassRenameFindMatchesAny(rows, MassRenameModeRegex, "", false, nil) {
		t.Fatal("nil regex should match all")
	}
	if MassRenameFindMatchesAny([]MassRenameRow{{OldBase: "a.dat", NewBase: "a.dat"}}, MassRenameModeRegex, "", false, re) {
		t.Fatal("expected no regex match on .dat")
	}
}

func TestMassRenameComputeRegexNilIsIdentity(t *testing.T) {
	dir := t.TempDir()
	entries := []localfs.Entry{
		{Name: "x.txt", Path: filepath.Join(dir, "x.txt"), Type: localfs.EntryFile},
	}
	rows, err := MassRenameCompute(entries, dir, MassRenameModeRegex, "", "anything", false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].NewBase != "x.txt" {
		t.Fatalf("got %#v", rows)
	}
}

func TestMassRenameMatchRangesSimple(t *testing.T) {
	got := MassRenameMatchRanges("foo_a_foo.txt", MassRenameModeSimple, "foo", false, nil)
	want := []search.Range{{Start: 0, End: 3}, {Start: 6, End: 9}}
	if !massRenameRangesEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMassRenameMatchRangesSimpleCaseFold(t *testing.T) {
	got := MassRenameMatchRanges("Alpha.txt", MassRenameModeSimple, "alpha", true, nil)
	want := []search.Range{{Start: 0, End: 5}}
	if !massRenameRangesEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMassRenameMatchRangesRegex(t *testing.T) {
	re, err := MassRenameCompileRegex(`\.txt$`, false)
	if err != nil {
		t.Fatal(err)
	}
	got := MassRenameMatchRanges("a.txt", MassRenameModeRegex, "", false, re)
	want := []search.Range{{Start: 1, End: 5}}
	if !massRenameRangesEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMassRenameMatchRangesEmptyFind(t *testing.T) {
	if got := MassRenameMatchRanges("a.txt", MassRenameModeSimple, "", false, nil); got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

func TestMassRenameBeforePreviewHighlightRanges_emptyReplace(t *testing.T) {
	match := []search.Range{{Start: 0, End: 3}, {Start: 6, End: 9}}
	removed, replaced := MassRenameBeforePreviewHighlightRanges(match, "")
	wantRemoved := match
	if !massRenameRangesEqual(removed, wantRemoved) {
		t.Fatalf("removed: got %v, want %v", removed, wantRemoved)
	}
	if replaced != nil {
		t.Fatalf("replaced: got %v, want nil", replaced)
	}
}

func TestMassRenameBeforePreviewHighlightRanges_emptyReplaceIgnoresSharedRunesInDeletedRegions(t *testing.T) {
	// Bracket tags share digits/spaces with kept "S01 "; LCS would falsely mark "01 " removed.
	old := "Skeleton Knight in Another World - S01 [BD 1080p HEVC 10bit AAC-FLAC] [Dual-Audio]"
	re, err := MassRenameCompileRegex(`\[.*?\]`, false)
	if err != nil {
		t.Fatal(err)
	}
	match := MassRenameMatchRanges(old, MassRenameModeRegex, "", false, re)
	removed, replaced := MassRenameBeforePreviewHighlightRanges(match, "")
	if replaced != nil {
		t.Fatalf("replaced: got %v, want nil", replaced)
	}
	if !massRenameRangesEqual(removed, match) {
		t.Fatalf("removed: got %v, want %v", removed, match)
	}
	runes := []rune(old)
	for _, r := range removed {
		got := string(runes[r.Start:r.End])
		if got[0] != '[' || got[len(got)-1] != ']' {
			t.Fatalf("removed span %q is not a bracket tag", got)
		}
	}
	keep := "01 "
	for _, r := range removed {
		span := string(runes[r.Start:r.End])
		if strings.Contains(span, keep) {
			t.Fatalf("removed span %q must not include kept %q", span, keep)
		}
	}
}

func TestMassRenameBeforePreviewHighlightRanges_nonemptyReplace(t *testing.T) {
	match := []search.Range{{Start: 0, End: 3}}
	removed, replaced := MassRenameBeforePreviewHighlightRanges(match, "bar")
	if removed != nil {
		t.Fatalf("removed: got %v, want nil", removed)
	}
	if !massRenameRangesEqual(replaced, match) {
		t.Fatalf("replaced: got %v, want %v", replaced, match)
	}
}

func TestMassRenameBeforePreviewHighlightRanges_whitespaceReplace(t *testing.T) {
	match := []search.Range{{Start: 0, End: 3}}
	removed, replaced := MassRenameBeforePreviewHighlightRanges(match, " ")
	if removed != nil {
		t.Fatalf("removed: got %v, want nil", removed)
	}
	if !massRenameRangesEqual(replaced, match) {
		t.Fatalf("replaced: got %v, want %v", replaced, match)
	}
}

func TestMassRenameReplacementRangesSharedRune(t *testing.T) {
	old := "Season 11M"
	got := MassRenameReplacementRanges(old, MassRenameModeSimple, "M", "xMissing", false, nil)
	want := []search.Range{{Start: 9, End: 17}}
	if !massRenameRangesEqual(got, want) {
		t.Fatalf("xMissing: got %v, want %v", got, want)
	}
	got = MassRenameReplacementRanges(old, MassRenameModeSimple, "M", "Missing", false, nil)
	want = []search.Range{{Start: 9, End: 16}}
	if !massRenameRangesEqual(got, want) {
		t.Fatalf("Missing: got %v, want %v", got, want)
	}
}

func TestMassRenameReplacementRangesMultiple(t *testing.T) {
	got := MassRenameReplacementRanges("foo_foo.txt", MassRenameModeSimple, "foo", "x", false, nil)
	want := []search.Range{{Start: 0, End: 1}, {Start: 2, End: 3}}
	if !massRenameRangesEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMassRenameReplacementRangesRegex(t *testing.T) {
	re, err := MassRenameCompileRegex(`M`, false)
	if err != nil {
		t.Fatal(err)
	}
	got := MassRenameReplacementRanges("Season 11M", MassRenameModeRegex, "", "Missing", false, re)
	want := []search.Range{{Start: 9, End: 16}}
	if !massRenameRangesEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func massRenameRangesEqual(a, b []search.Range) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func TestMassRenameComputeSimple(t *testing.T) {
	dir := t.TempDir()
	entries := []localfs.Entry{
		{Name: "foo_bar.txt", Path: filepath.Join(dir, "foo_bar.txt"), Type: localfs.EntryFile},
		{Name: "foo_baz.txt", Path: filepath.Join(dir, "foo_baz.txt"), Type: localfs.EntryFile},
	}
	rows, err := MassRenameCompute(entries, dir, MassRenameModeSimple, "foo_", "x_", false, nil)
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
	rows, err := MassRenameCompute(entries, dir, MassRenameModeSimple, "bc", "xx", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].NewBase != "Axx.txt" {
		t.Fatalf("got %q", rows[0].NewBase)
	}
}

func TestMassRenameComputeRegex(t *testing.T) {
	dir := t.TempDir()
	re, err := MassRenameCompileRegex(`^(.+)_(.+)\.txt$`)
	if err != nil {
		t.Fatal(err)
	}
	entries := []localfs.Entry{
		{Name: "aa_bb.txt", Path: filepath.Join(dir, "aa_bb.txt"), Type: localfs.EntryFile},
	}
	rows, err := MassRenameCompute(entries, dir, MassRenameModeRegex, "", `${2}_${1}.txt`, false, re)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].NewBase != "bb_aa.txt" {
		t.Fatalf("got %q", rows[0].NewBase)
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

func TestMassRenameRegexCompileUserMessage(t *testing.T) {
	_, err := MassRenameCompileRegex("a++")
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

func TestMassRenameCompileRegexEmpty(t *testing.T) {
	if _, err := MassRenameCompileRegex("   "); err == nil {
		t.Fatal("expected error")
	}
}

func TestMassRenameComputeSimpleEmptyFindIsIdentity(t *testing.T) {
	dir := t.TempDir()
	entries := []localfs.Entry{
		{Name: "x.txt", Path: filepath.Join(dir, "x.txt"), Type: localfs.EntryFile},
	}
	rows, err := MassRenameCompute(entries, dir, MassRenameModeSimple, "", "y", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].OldBase != "x.txt" || rows[0].NewBase != "x.txt" {
		t.Fatalf("got %#v", rows)
	}
}

func TestMassRenameComputeRegexNilIsIdentity(t *testing.T) {
	dir := t.TempDir()
	entries := []localfs.Entry{
		{Name: "x.txt", Path: filepath.Join(dir, "x.txt"), Type: localfs.EntryFile},
	}
	rows, err := MassRenameCompute(entries, dir, MassRenameModeRegex, "", "anything", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].NewBase != "x.txt" {
		t.Fatalf("got %#v", rows)
	}
}

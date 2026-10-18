package commands

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func touchFile(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDetectOutputPaths(t *testing.T) {
	dir := t.TempDir()
	abs := touchFile(t, dir, "harbor.txt")
	touchFile(t, dir, "lantern.go")
	spaced := touchFile(t, dir, "velvet orchard.md")
	touchFile(t, dir, "meadow")
	touchFile(t, dir, "quartz")
	touchFile(t, dir, "new anchor")
	touchFile(t, dir, "Season willow")
	touchFile(t, dir, "harbor")
	touchFile(t, dir, "harbor lights")
	lines := []string{
		abs,                                  // 0 absolute
		"lantern.go",                         // 1 relative
		"velvet orchard.md",                  // 2 name with spaces
		"lantern.go:12:func x() {}",          // 3 grep -n style
		"\x1b[31mmeadow\x1b[0m",              // 4 ANSI-colored
		"see meadow and (quartz),",           // 5 two paths, brackets/comma trimmed
		"nothing here",                       // 6 no match
		"--- stderr ---",                     // 7 separator
		"found: './new anchor', checked",     // 8 quoted multi-word mid-line
		"./Season willow:12: grep-style hit", // 9 multi-word with grep suffix
		"see harbor lights now",              // 10 longest of harbor / harbor lights wins
	}
	want := []dialog.OutputPathMatch{
		{Line: 0, Start: 0, End: len([]rune(abs)), Path: abs},
		{Line: 1, Start: 0, End: 10, Path: filepath.Join(dir, "lantern.go")},
		{Line: 2, Start: 0, End: 17, Path: spaced},
		{Line: 3, Start: 0, End: 10, Path: filepath.Join(dir, "lantern.go")},
		{Line: 4, Start: 0, End: 6, Path: filepath.Join(dir, "meadow")},
		{Line: 5, Start: 4, End: 10, Path: filepath.Join(dir, "meadow")},
		{Line: 5, Start: 16, End: 22, Path: filepath.Join(dir, "quartz")},
		{Line: 8, Start: 8, End: 20, Path: filepath.Join(dir, "new anchor")},
		{Line: 9, Start: 0, End: 15, Path: filepath.Join(dir, "Season willow")},
		{Line: 10, Start: 4, End: 17, Path: filepath.Join(dir, "harbor lights")},
	}
	got := detectOutputPaths(lines, dir)
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestPathLineNavigation(t *testing.T) {
	p := []dialog.OutputPathMatch{{Line: 0}, {Line: 0}, {Line: 3}, {Line: 5}, {Line: 5}}
	if got := nextPathLine(p, 0); got != 2 {
		t.Fatalf("next from 0 = %d", got)
	}
	if got := nextPathLine(p, 4); got != 4 {
		t.Fatalf("next at end = %d", got)
	}
	if got := prevPathLine(p, 4); got != 2 {
		t.Fatalf("prev from 4 = %d", got)
	}
	if got := prevPathLine(p, 2); got != 0 {
		t.Fatalf("prev from 2 = %d", got)
	}
	if got := prevPathLine(p, 1); got != 1 {
		t.Fatalf("prev at top = %d", got)
	}
}

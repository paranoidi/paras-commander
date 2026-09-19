package preview

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestRenderArchiveTreeConnectors(t *testing.T) {
	paths := []string{
		"garden/tulip.txt",
		"garden/fern/",
		"garden/fern/moss.txt",
		"readme.txt",
	}
	var th theme.Theme
	cells := renderArchiveTree(paths, th, tcell.StyleDefault)
	var b strings.Builder
	for _, c := range cells {
		b.WriteRune(c.R)
	}
	got := b.String()
	// dirs first under root (garden), then file (readme); inside garden: fern dir then tulip
	wantLines := []string{
		"garden",
		"├─ fern",
		"│  └─ moss.txt",
		"└─ tulip.txt",
		"readme.txt",
	}
	for _, line := range wantLines {
		if !strings.Contains(got, line) {
			t.Fatalf("missing %q in:\n%s", line, got)
		}
	}
}

func TestRenderArchiveTreeIcons(t *testing.T) {
	paths := []string{
		"garden/tulip.txt",
		"garden/fern/",
		"garden/fern/moss.txt",
		"readme.txt",
	}
	var plainTheme theme.Theme
	plain := renderArchiveTree(paths, plainTheme, tcell.StyleDefault)

	iconTheme := theme.Theme{UseNerdfontIcons: true}
	withIcons := renderArchiveTree(paths, iconTheme, tcell.StyleDefault)

	if len(withIcons) <= len(plain) {
		t.Fatalf("expected icons to add cells: plain=%d withIcons=%d", len(plain), len(withIcons))
	}

	var b strings.Builder
	for _, c := range withIcons {
		b.WriteRune(c.R)
	}
	got := b.String()
	for _, want := range []string{"fern", "moss.txt", "tulip.txt"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

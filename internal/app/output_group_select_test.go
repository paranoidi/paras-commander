package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestOutputGroupSelect(t *testing.T) {
	root := t.TempDir()
	lantern := filepath.Join(root, "lantern.go")
	quartz := filepath.Join(root, "quartz.txt")
	orchard := filepath.Join(root, "orchard.go")
	meadow := filepath.Join(root, "meadow.go") // directory named like a Go file
	for _, p := range []string{lantern, quartz, orchard} {
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(meadow, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)
	app := newTestApp(t, screen, testOptions(root))
	app.model.CommandOutputDialog = dialog.CommandOutputDialogState{
		Open: true,
		Paths: []dialog.OutputPathMatch{
			{Line: 0, Path: lantern}, {Line: 1, Path: quartz}, {Line: 2, Path: orchard},
			{Line: 3, Path: meadow, IsDir: true}, {Line: 4, Path: lantern},
		},
	}
	press := func(k tcell.Key, r rune) {
		t.Helper()
		if quit, _ := app.handleKey(tcell.NewEventKey(k, r, tcell.ModNone)); quit {
			t.Fatal("quit")
		}
	}
	p := app.activePanel()

	press(tcell.KeyRune, '+')
	gs := &app.model.GroupSelect
	if !gs.Open || gs.Context != "output" || gs.Mode != "select" {
		t.Fatalf("group select = %+v", *gs)
	}
	for _, r := range "*.go" {
		press(tcell.KeyRune, r)
	}
	app.updateGroupSelectPreview()
	if !gs.PreviewShow || gs.PreviewFiles != 2 || gs.PreviewFolders != 1 {
		t.Fatalf("preview = %d files %d dirs show=%v", gs.PreviewFiles, gs.PreviewFolders, gs.PreviewShow)
	}
	gs.DirsOnly = true
	app.updateGroupSelectPreview()
	if gs.PreviewFiles != 0 || gs.PreviewFolders != 1 {
		t.Fatalf("dirs-only preview = %d/%d", gs.PreviewFiles, gs.PreviewFolders)
	}
	gs.DirsOnly = false
	press(tcell.KeyEnter, 0)
	if gs.Open || !app.model.CommandOutputDialog.Open {
		t.Fatal("group select should close back to the output dialog")
	}
	for path, want := range map[string]bool{lantern: true, orchard: true, meadow: true, quartz: false} {
		if p.IsSelectedPath(path) != want {
			t.Fatalf("%s selected=%v, want %v", filepath.Base(path), !want, want)
		}
	}

	press(tcell.KeyRune, '-')
	gs.FullPath = true // matched against the absolute path, so the glob needs wildcards
	gs.Text = "*lantern*"
	press(tcell.KeyEnter, 0)
	if p.IsSelectedPath(lantern) || !p.IsSelectedPath(orchard) {
		t.Fatal("unselect group should only drop lantern.go")
	}

	press(tcell.KeyRune, '*') // invert over the 4 unique paths
	if !p.IsSelectedPath(lantern) || !p.IsSelectedPath(quartz) || p.IsSelectedPath(orchard) || p.IsSelectedPath(meadow) {
		t.Fatal("invert did not toggle every detected path")
	}
}

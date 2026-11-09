package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestDedupViewGroupSelect(t *testing.T) {
	dir := t.TempDir()
	harbor := filepath.Join(dir, "harbor.jpg")
	harborCopy := filepath.Join(dir, "harbor-copy.jpg")
	lantern := filepath.Join(dir, "lantern.txt")
	lanternOld := filepath.Join(dir, "lantern-old.txt")
	for p, content := range map[string]string{harbor: "pixels", harborCopy: "pixels", lantern: "words", lanternOld: "words"} {
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 100, 30)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)

	press := func(k tcell.Key, r rune) {
		t.Helper()
		if quit, _ := app.handleKey(tcell.NewEventKey(k, r, tcell.ModNone)); quit {
			t.Fatal("quit")
		}
	}
	typeText := func(s string) {
		t.Helper()
		for _, r := range s {
			press(tcell.KeyRune, r)
		}
	}
	app.model.DedupView.Kept[harbor] = true

	press(tcell.KeyRune, '+')
	gs := &app.model.GroupSelect
	if !gs.Open || gs.Context != "dedup" || gs.Mode != "select" {
		t.Fatalf("group select = %+v", *gs)
	}
	typeText("*.jpg")
	app.updateGroupSelectPreview()
	if !gs.PreviewShow || gs.PreviewFiles != 1 || gs.PreviewFolders != 0 {
		t.Fatalf("preview = %d files %d dirs show=%v, want 1 (kept file skipped)", gs.PreviewFiles, gs.PreviewFolders, gs.PreviewShow)
	}
	press(tcell.KeyEnter, 0)
	if gs.Open {
		t.Fatal("dialog still open after Enter")
	}
	marked := app.model.DedupView.Marked
	if !marked[harborCopy] || marked[harbor] || marked[lantern] || marked[lanternOld] || app.model.DedupView.MarkedCount != 1 {
		t.Fatalf("marked = %v, want only %s", marked, harborCopy)
	}

	press(tcell.KeyRune, '+')
	typeText("lantern*")
	press(tcell.KeyEnter, 0)
	if app.model.DedupView.MarkedCount != 3 {
		t.Fatalf("MarkedCount = %d, want 3", app.model.DedupView.MarkedCount)
	}

	press(tcell.KeyRune, '-')
	if !gs.Open || gs.Mode != "unselect" {
		t.Fatalf("group select = %+v", *gs)
	}
	typeText("*old*")
	press(tcell.KeyEnter, 0)
	marked = app.model.DedupView.Marked
	if marked[lanternOld] || !marked[lantern] || !marked[harborCopy] || app.model.DedupView.MarkedCount != 2 {
		t.Fatalf("marked = %v after unselect", marked)
	}
	if want := int64(len("pixels") + len("words")); app.model.DedupView.MarkedReclaimBytes != want {
		t.Fatalf("MarkedReclaimBytes = %d, want %d", app.model.DedupView.MarkedReclaimBytes, want)
	}
}

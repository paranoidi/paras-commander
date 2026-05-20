package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEditActiveFileRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "file.txt"))
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	if err := app.model.Left.Load(dir); err != nil {
		t.Fatal(err)
	}
	for i, e := range app.model.Left.Entries {
		if e.Name == "sub" {
			app.model.Left.Cursor = i
			break
		}
	}
	prev := externalEditorRunner
	externalEditorRunner = func(_ context.Context, path string) error {
		t.Fatalf("editor should not run for directory, got %q", path)
		return nil
	}
	t.Cleanup(func() { externalEditorRunner = prev })
	app.editActiveFile()
}

func TestEditActiveFileOpensEditorForFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "note.txt")
	writeFile(t, filePath)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	if err := app.model.Left.Load(dir); err != nil {
		t.Fatal(err)
	}
	for i, e := range app.model.Left.Entries {
		if e.Name == "note.txt" {
			app.model.Left.Cursor = i
			break
		}
	}

	var edited string
	prev := externalEditorRunner
	externalEditorRunner = func(_ context.Context, path string) error {
		edited = path
		return nil
	}
	t.Cleanup(func() { externalEditorRunner = prev })

	app.editActiveFile()
	if edited != filePath {
		t.Fatalf("edited = %q, want %q", edited, filePath)
	}
}

func TestWithTerminalReleasedTwice(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	var calls int
	run := func() error {
		calls++
		return nil
	}
	app.lastScreenContentHash = 0xdeadbeef
	if err := app.withTerminalReleased(run); err != nil {
		t.Fatal(err)
	}
	if app.lastScreenContentHash == 0xdeadbeef {
		t.Fatal("render after resume should refresh screen hash cache")
	}
	if err := app.withTerminalReleased(run); err != nil {
		t.Fatalf("second suspend/resume: %v", err)
	}
	if calls != 2 {
		t.Fatalf("run calls = %d, want 2", calls)
	}
}

func TestClassifyEditPath(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, mode := classifyEditPath(filePath)
	if mode != editTargetFile || path != filePath {
		t.Fatalf("file: path=%q mode=%v", path, mode)
	}
	_, mode = classifyEditPath(dir)
	if mode != editTargetDir {
		t.Fatalf("dir: mode=%v want dir", mode)
	}
	_, mode = classifyEditPath(filepath.Join(dir, "missing.txt"))
	if mode != editTargetMissing {
		t.Fatalf("missing: mode=%v", mode)
	}
}

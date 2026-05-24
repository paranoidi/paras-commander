package panel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotParentFalseAtRoot(t *testing.T) {
	state, err := New("/")
	if err != nil {
		t.Skip("cannot open /:", err)
	}
	if _, ok := state.SnapshotParent(10); ok {
		t.Fatal("SnapshotParent at filesystem root = true, want false")
	}
}

func TestSnapshotParentHighlightsChildDir(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "walnut")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	state, err := New(child)
	if err != nil {
		t.Fatal(err)
	}
	snap, ok := state.SnapshotParent(10)
	if !ok {
		t.Fatal("SnapshotParent = false, want true")
	}
	if snap.Path.String() != root {
		t.Fatalf("parent path = %q, want %q", snap.Path.String(), root)
	}
	if snap.Cursor < 0 || snap.Cursor >= len(snap.Entries) || snap.Entries[snap.Cursor].Name != "walnut" {
		t.Fatalf("cursor index %d entries = %v, want walnut", snap.Cursor, snap.Entries)
	}
}

func TestSnapshotChildFalseOnFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "cedar.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range state.Entries {
		if e.Name == "cedar.txt" {
			state.Cursor = i
			break
		}
	}
	if _, ok := state.SnapshotChild(10); ok {
		t.Fatal("SnapshotChild on file = true, want false")
	}
}

func TestSnapshotChildRecallsCursor(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "maple")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, "birch.log"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if !state.SelectVisibleEntry("maple") {
		t.Fatal("maple not found")
	}
	if _, err := state.Enter(10); err != nil {
		t.Fatal(err)
	}
	if !state.SelectVisibleEntry("birch.log") {
		t.Fatal("birch.log not found in child")
	}
	if err := state.Parent(10); err != nil {
		t.Fatal(err)
	}
	snap, ok := state.SnapshotChild(10)
	if !ok {
		t.Fatal("SnapshotChild = false, want true")
	}
	if snap.Path.String() != child {
		t.Fatalf("child path = %q, want %q", snap.Path.String(), child)
	}
	if snap.Cursor < 0 || snap.Cursor >= len(snap.Entries) || snap.Entries[snap.Cursor].Name != "birch.log" {
		t.Fatalf("cursor index %d entries = %v, want birch.log", snap.Cursor, snap.Entries)
	}
}

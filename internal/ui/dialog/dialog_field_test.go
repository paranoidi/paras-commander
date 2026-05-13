package dialog

import "testing"

func TestFileDialogFieldInsertClearsPendingPrefill(t *testing.T) {
	field := FileDialogField{
		Value:          "suggested",
		Prefill:        "suggested",
		PrefillPending: true,
		Cursor:         len([]rune("suggested")),
	}

	field.InsertRune('x')

	if field.Value != "x" {
		t.Fatalf("Value = %q, want %q", field.Value, "x")
	}
	if field.Cursor != 1 {
		t.Fatalf("Cursor = %d, want 1", field.Cursor)
	}
	if field.PrefillPending {
		t.Fatal("PrefillPending = true, want false")
	}
}

func TestFileDialogFieldNavigationCommitsPendingPrefill(t *testing.T) {
	field := FileDialogField{
		Value:          "abc",
		Prefill:        "abc",
		PrefillPending: true,
		Cursor:         3,
	}

	field.MoveCursor(-1)

	if field.Value != "abc" {
		t.Fatalf("Value = %q, want %q", field.Value, "abc")
	}
	if field.Cursor != 2 {
		t.Fatalf("Cursor = %d, want 2", field.Cursor)
	}
	if field.PrefillPending {
		t.Fatal("PrefillPending = true, want false")
	}
}

func TestFileDialogFieldEditOperations(t *testing.T) {
	field := FileDialogField{Value: "abc", Cursor: 1}

	field.InsertRune('X')
	if field.Value != "aXbc" || field.Cursor != 2 {
		t.Fatalf("after insert: value=%q cursor=%d", field.Value, field.Cursor)
	}

	field.Backspace()
	if field.Value != "abc" || field.Cursor != 1 {
		t.Fatalf("after backspace: value=%q cursor=%d", field.Value, field.Cursor)
	}

	field.Delete()
	if field.Value != "ac" || field.Cursor != 1 {
		t.Fatalf("after delete: value=%q cursor=%d", field.Value, field.Cursor)
	}

	field.MoveCursorEnd()
	field.Clear()
	if field.Value != "" || field.Cursor != 0 {
		t.Fatalf("after clear: value=%q cursor=%d", field.Value, field.Cursor)
	}
}

func TestFileDialogFieldRestorePrefillAfterClear(t *testing.T) {
	field := FileDialogField{
		Value:          "report.txt",
		Prefill:        "report.txt",
		PrefillPending: true,
		Cursor:         len([]rune("report.txt")),
	}

	field.Clear()
	if field.Value != "" || field.Cursor != 0 || field.PrefillPending {
		t.Fatalf("clear precondition: value=%q cursor=%d pending=%v", field.Value, field.Cursor, field.PrefillPending)
	}

	if !field.RestorePrefill() {
		t.Fatal("RestorePrefill = false, want true")
	}
	if field.Value != "report.txt" {
		t.Fatalf("Value = %q, want %q", field.Value, "report.txt")
	}
	if field.Cursor != len([]rune("report.txt")) {
		t.Fatalf("Cursor = %d, want %d", field.Cursor, len([]rune("report.txt")))
	}
	if !field.PrefillPending {
		t.Fatal("PrefillPending = false, want true")
	}
}

func TestFileDialogFieldRestorePrefillAfterEdit(t *testing.T) {
	field := FileDialogField{
		Value:          "report.txt",
		Prefill:        "report.txt",
		PrefillPending: true,
		Cursor:         len([]rune("report.txt")),
	}

	field.InsertRune('x')
	field.Backspace()
	if field.PrefillPending {
		t.Fatalf("expected PrefillPending=false after edit, got true (value=%q)", field.Value)
	}

	if !field.RestorePrefill() {
		t.Fatal("RestorePrefill = false, want true")
	}
	if field.Value != "report.txt" || field.Cursor != len([]rune("report.txt")) || !field.PrefillPending {
		t.Fatalf("after restore: value=%q cursor=%d pending=%v", field.Value, field.Cursor, field.PrefillPending)
	}
}

func TestFileDialogFieldRestorePrefillNoOpWhenEmpty(t *testing.T) {
	field := FileDialogField{Value: "abc", Cursor: 2}
	if field.RestorePrefill() {
		t.Fatal("RestorePrefill = true, want false (no Prefill)")
	}
	if field.Value != "abc" || field.Cursor != 2 || field.PrefillPending {
		t.Fatalf("state mutated: value=%q cursor=%d pending=%v", field.Value, field.Cursor, field.PrefillPending)
	}

	var nilField *FileDialogField
	if nilField.RestorePrefill() {
		t.Fatal("nil receiver: want false")
	}
}

func TestFileDialogFieldKillWordBackwardPath(t *testing.T) {
	field := FileDialogField{Value: "/foo/bar", Cursor: len([]rune("/foo/bar"))}
	field.KillWordBackward()
	if field.Value != "/foo/" || field.Cursor != 5 {
		t.Fatalf("after kill: value=%q cursor=%d", field.Value, field.Cursor)
	}
}

func TestFileDialogFieldMoveWordCommitsPrefill(t *testing.T) {
	field := FileDialogField{
		Value:          "/a/b",
		Prefill:        "/a/b",
		PrefillPending: true,
		Cursor:         len([]rune("/a/b")),
	}
	field.MoveWordBackward()
	if field.PrefillPending {
		t.Fatal("MoveWordBackward should commit prefill")
	}
	if field.Cursor != 3 {
		t.Fatalf("cursor = %d, want 3 (before last segment)", field.Cursor)
	}
}

func TestFileDialogFieldMoveWordForward(t *testing.T) {
	field := FileDialogField{Value: "/x/y", Cursor: 0}
	field.MoveWordForward()
	if field.Cursor != 2 {
		t.Fatalf("cursor = %d, want 2", field.Cursor)
	}
}

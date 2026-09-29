package dialog

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathpick"
)

func TestFileDialogFieldAcceptCompletion(t *testing.T) {
	field := FileDialogField{Value: "/tmp/f", Cursor: 6}
	field.Completion.Set(field.Value, 5, []pathpick.Candidate{{Name: "foo", IsDir: true}})
	newValue, newCursor := field.Completion.Accept(field.Value)
	field.Value, field.Cursor = newValue, newCursor
	want := "/tmp/foo/"
	if field.Value != want {
		t.Fatalf("Value = %q want %q", field.Value, want)
	}
	if field.Cursor != len([]rune(want)) {
		t.Fatalf("Cursor = %d want %d", field.Cursor, len([]rune(want)))
	}
	if field.Completion.Open || len(field.Completion.Items) != 0 {
		t.Fatalf("Completion = %+v, want cleared", field.Completion)
	}
}

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

func TestFileDialogFieldKillThenYank(t *testing.T) {
	f := &FileDialogField{Value: "foo bar", Cursor: 7}
	f.KillWordBackward()
	if f.Value != "foo " {
		t.Fatalf("after kill %q", f.Value)
	}
	f.Yank()
	if f.Value != "foo bar" || f.Cursor != 7 {
		t.Fatalf("after yank %q %d", f.Value, f.Cursor)
	}
}

func TestFileDialogFieldKillLineThenYank(t *testing.T) {
	f := &FileDialogField{Value: "foo bar", Cursor: 3}
	f.KillLine()
	if f.Value != "" {
		t.Fatalf("after kill %q", f.Value)
	}
	f.KillLine() // empty: buffer untouched
	f.Yank()
	if f.Value != "foo bar" || f.Cursor != 7 {
		t.Fatalf("after yank %q %d", f.Value, f.Cursor)
	}
}

func TestFileDialogFieldKillLineBackwardThenYank(t *testing.T) {
	f := &FileDialogField{Value: "foo bar", Cursor: 4}
	f.KillLineBackward()
	if f.Value != "bar" || f.Cursor != 0 {
		t.Fatalf("after kill %q %d", f.Value, f.Cursor)
	}
	f.KillLineBackward() // at start: buffer untouched
	f.MoveCursorEnd()
	f.Yank()
	if f.Value != "barfoo " || f.Cursor != 7 {
		t.Fatalf("after yank %q %d", f.Value, f.Cursor)
	}
}

func TestFileDialogFieldKillLineForwardThenYank(t *testing.T) {
	f := &FileDialogField{Value: "foo bar", Cursor: 3}
	f.KillLineForward()
	if f.Value != "foo" || f.Cursor != 3 {
		t.Fatalf("after kill %q %d", f.Value, f.Cursor)
	}
	f.KillLineForward() // at end: buffer untouched
	f.Yank()
	if f.Value != "foo bar" || f.Cursor != 7 {
		t.Fatalf("after yank %q %d", f.Value, f.Cursor)
	}
}

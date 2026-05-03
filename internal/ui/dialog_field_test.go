package ui

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

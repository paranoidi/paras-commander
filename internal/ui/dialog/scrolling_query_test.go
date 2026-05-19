package dialog

import "testing"

func TestScrollingQueryInsertBackspaceClear(t *testing.T) {
	q := &ScrollingQuery{Value: "ab", Cursor: 2}
	q.InsertRune('c')
	if q.Value != "abc" || q.Cursor != 3 {
		t.Fatalf("insert: value=%q cursor=%d", q.Value, q.Cursor)
	}
	q.Backspace()
	if q.Value != "ab" || q.Cursor != 2 {
		t.Fatalf("backspace: value=%q cursor=%d", q.Value, q.Cursor)
	}
	q.Clear()
	if q.Value != "" || q.Cursor != 0 || q.Scroll != 0 {
		t.Fatalf("clear: value=%q cursor=%d scroll=%d", q.Value, q.Cursor, q.Scroll)
	}
}

func TestScrollingQueryKillWordBackward(t *testing.T) {
	q := &ScrollingQuery{Value: "/foo/bar", Cursor: len([]rune("/foo/bar"))}
	q.KillWordBackward()
	if q.Value != "/foo/" || q.Cursor != 5 {
		t.Fatalf("kill word: value=%q cursor=%d", q.Value, q.Cursor)
	}
}
